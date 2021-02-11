// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package guacamole

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/aau-network-security/haaukins/exercise"
	wg "github.com/aau-network-security/haaukins/network/vpn"

	"net/http"
	"path/filepath"
	"time"

	pbc "github.com/aau-network-security/haaukins/store/proto"

	"io"
	"sync"

	"github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/amigo"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/rs/zerolog/log"
)

const (
	displayTimeFormat = "2006-01-02 15:04:05"
	Running           = State(0)
	Suspended         = State(1)
	Booked            = State(2)
	Closed            = State(3)
	Error             = State(4)
)

type State int

var (
	RdpConfErr      = errors.New("error too few rdp connections")
	StartingGuacErr = errors.New("error while starting guac")
	//EmptyNameErr    = errors.New("event requires a name")
	//EmptyTagErr     = errors.New("event requires a tag")

	ErrMaxLabs         = errors.New("maximum amount of allowed labs has been reached")
	ErrNoAvailableLabs = errors.New("no labs available in the queue")
	// random port number for creating different VPN servers
	min = 5000
	max = 6000
)

type Host interface {
	UpdateEventHostExercisesFile(store.ExerciseStore) error
	CreateEventFromEventDB(context.Context, store.EventConfig, string) (Event, error)
	CreateEventFromConfig(context.Context, store.EventConfig, string) (Event, error)
}

func NewHost(vlib vbox.Library, elib store.ExerciseStore, eDir string, dbc pbc.StoreClient, config wg.WireGuardConfig) Host {
	return &eventHost{
		ctx:       context.Background(),
		dbc:       dbc,
		vlib:      vlib,
		elib:      elib,
		dir:       eDir,
		vpnConfig: config,
	}
}

type eventHost struct {
	ctx       context.Context
	dbc       pbc.StoreClient
	vlib      vbox.Library
	elib      store.ExerciseStore
	vpnConfig wg.WireGuardConfig
	dir       string
}

//Create the event configuration for the event got from the DB
func (eh *eventHost) CreateEventFromEventDB(ctx context.Context, conf store.EventConfig, reCaptchaKey string) (Event, error) {
	exer, err := eh.elib.GetExercisesByTags(conf.Lab.Exercises...)
	if err != nil {
		return nil, err
	}
	es, err := store.NewEventStore(conf, eh.dir, eh.dbc)
	if err != nil {
		return nil, err
	}

	var labConf lab.Config
	if conf.OnlyVPN {
		labConf = lab.Config{
			Exercises: exer,
		}
		es.OnlyVPN = conf.OnlyVPN
		es.WireGuardConfig = eh.vpnConfig
	} else {
		labConf = lab.Config{
			Exercises: exer,
			Frontends: conf.Lab.Frontends,
		}
	}
	lh := lab.LabHost{
		Vlib: eh.vlib,
		Conf: labConf,
	}
	hub, err := lab.NewHub(ctx, &lh, conf.Available, conf.Capacity, conf.OnlyVPN)
	if err != nil {
		return nil, err
	}

	return NewEvent(eh.ctx, es, hub, labConf.Flags(), reCaptchaKey)
}

//Save the event in the DB and create the event configuration
func (eh *eventHost) CreateEventFromConfig(ctx context.Context, conf store.EventConfig, reCaptchaKey string) (Event, error) {
	var exercises []string
	log.Info().Msgf("VPN Address from CreateEventFromConfig function %s ", conf.VPNAddress)
	for _, e := range conf.Lab.Exercises {
		exercises = append(exercises, string(e))
	}
	_, err := eh.dbc.AddEvent(ctx, &pbc.AddEventRequest{
		Name:               conf.Name,
		Tag:                string(conf.Tag),
		Frontends:          conf.Lab.Frontends[0].Image,
		Exercises:          strings.Join(exercises, ","),
		Available:          int32(conf.Available),
		Capacity:           int32(conf.Capacity),
		Status:             int32(conf.Status),
		StartTime:          conf.StartedAt.Format(displayTimeFormat),
		ExpectedFinishTime: conf.FinishExpected.Format(displayTimeFormat),
		CreatedBy:          conf.CreatedBy,
		OnlyVPN:            conf.OnlyVPN,
	})

	if err != nil {
		return nil, err
	}

	return eh.CreateEventFromEventDB(ctx, conf, reCaptchaKey)
}

func (eh *eventHost) UpdateEventHostExercisesFile(es store.ExerciseStore) error {
	if len(es.ListExercises()) == 0 {
		return errors.New("Provided exercisestore is empty, be careful next time ! ")
	}
	eh.elib = es
	return nil
}

type Event interface {
	Start(context.Context) error
	Close() error
	Suspend(context.Context) error
	Resume(context.Context) error

	Finish(string)
	AssignLab(*store.Team, lab.Lab) error
	Handler() http.Handler

	SetStatus(int32)
	GetStatus() int32
	GetConfig() store.EventConfig
	GetTeams() []*store.Team
	GetHub() lab.Hub
	GetLabByTeam(teamId string) (lab.Lab, bool)
}

type event struct {
	amigo  *amigo.Amigo
	guac   Guacamole
	labhub lab.Hub

	labs          map[string]lab.Lab
	store         store.Event
	keyLoggerPool KeyLoggerPool

	ipAddrs       []int
	wg            wg.WireguardClient
	guacUserStore *GuacUserStore
	dockerHost    docker.Host

	closers []io.Closer
}

type labNetInfo struct {
	dns             string
	subnet          string
	dnsrecords      []*exercise.DNSRecord
	wgInterfacePort int
}

func NewEvent(ctx context.Context, e store.Event, hub lab.Hub, flags []store.FlagConfig, reCaptchaKey string) (Event, error) {

	guac, err := New(ctx, Config{}, e.OnlyVPN)
	if err != nil {
		return nil, err
	}
	// New wireguard gRPC client connection
	wgClient, err := wg.NewGRPCVPNClient(e.WireGuardConfig)
	if err != nil {
		log.Error().Msgf("Connection error on wireguard service error %v ", err)
		return nil, err
	}

	dirname, err := store.GetDirNameForEvent(e.Dir, e.Tag, e.StartedAt)
	if err != nil {
		return nil, err
	}

	dockerHost := docker.NewHost()
	amigoOpt := amigo.WithEventName(e.Name)
	keyLoggerPool, err := NewKeyLoggerPool(filepath.Join(e.Dir, dirname))
	if err != nil {
		return nil, err
	}

	ev := &event{
		store:         e,
		labhub:        hub,
		amigo:         amigo.NewAmigo(e, flags, reCaptchaKey, wgClient, amigoOpt),
		guac:          guac,
		ipAddrs:       makeRange(2, 254),
		labs:          map[string]lab.Lab{},
		guacUserStore: NewGuacUserStore(),
		wg:            wgClient,
		closers:       []io.Closer{guac, hub, keyLoggerPool},
		dockerHost:    dockerHost,
		keyLoggerPool: keyLoggerPool,
	}

	return ev, nil
}

// SetStatus sets status of event in cache
func (ev *event) SetStatus(state int32) {
	ev.store.Status = state
}

func (ev *event) GetStatus() int32 {
	return ev.store.Status
}

func (ev *event) Start(ctx context.Context) error {
	if ev.store.OnlyVPN {
		//randomly taken port for each VPN endpoint
		port := rand.Intn(max-min) + min
		for checkPort(port) {
			port = rand.Intn(max-min) + min
		}
		ev.store.EndPointPort = port
		log.Info().Msgf("Connection established with VPN service on port %d", port)
		log.Info().Msgf("Initializing VPN endpoint for event ")
		_, err := ev.wg.InitializeI(context.Background(), &wg.IReq{
			Address:    ev.store.VPNAddress, // this should be randomized and should not collide with lab subnet like 124.5.6.0/24
			ListenPort: uint32(port),        // this should be randomized and should not collide with any used ports by host
			SaveConfig: true,
			Eth:        "eth0",
			IName:      string(ev.store.Tag),
		})

		if err != nil {
			// handle error
			log.Debug().Msgf("Information initializing interface for wireguard failed, VPN connection might not be available ! err %v\n", err)
			return err
		}
		log.Info().Str("Address:", ev.store.VPNAddress).
			Int("ListenPort", port).
			Str("Ethernet", "eth").
			Str("Interface Name: ", string(ev.store.Tag)).Msgf("Wireguard interface initialized for event %s", string(ev.store.Tag))
	} else {
		if err := ev.guac.Start(ctx); err != nil {
			log.
				Error().
				Err(err).
				Msg("error starting guac")

			return StartingGuacErr
		}
	}

	for _, team := range ev.store.GetTeams() {
		lab, ok := <-ev.labhub.Queue()
		if !ok {
			return ErrMaxLabs
		}

		if err := ev.AssignLab(team, lab); err != nil {
			fmt.Println("Issue assigning lab: ", err)
			return err
		}

	}

	return nil
}

//CreateVPNConn will generate VPN Connection configuration per team.
func (ev *event) CreateVPNConn(t *store.Team, labInfo *labNetInfo) ([]string, error) {
	var teamConfigFiles []string
	ctx := context.Background()
	evTag := string(ev.GetConfig().Tag)
	team := t.ID()
	var hosts string
	for _, r := range labInfo.dnsrecords {
		for ip, arecord := range r.Record {
			hosts += fmt.Sprintf("# %s \t %s \n", ip, arecord)
		}

	}

	// generate an ip for peer for wireguard interface
	subnet := ev.store.VPNAddress

	// retrieve domain from configuration file
	endpoint := fmt.Sprintf("%s.%s:%d", evTag, ev.store.Host, ev.store.EndPointPort)

	// get public key of server
	log.Info().Msg("Getting server public key...")
	serverPubKey, err := ev.wg.GetPublicKey(ctx, &wg.PubKeyReq{PubKeyName: evTag, PrivKeyName: evTag})
	if err != nil {
		return []string{}, err
	}

	// create 4 different config file for 1 user
	for i := 0; i < 4; i++ {
		// generate client privatekey
		ipAddr := pop(&ev.ipAddrs)
		log.Info().Msgf("Generating privatekey for team %s", evTag+"_"+team)
		_, err = ev.wg.GenPrivateKey(ctx, &wg.PrivKeyReq{PrivateKeyName: evTag + "_" + team + "_" + strconv.Itoa(ipAddr)})
		if err != nil {
			return []string{}, err
		}

		// generate client public key
		log.Info().Msgf("Generating public key for team %s", evTag+"_"+team+"_"+strconv.Itoa(ipAddr))
		_, err = ev.wg.GenPublicKey(ctx, &wg.PubKeyReq{PubKeyName: evTag + "_" + team + "_" + strconv.Itoa(ipAddr), PrivKeyName: evTag + "_" + team + "_" + strconv.Itoa(ipAddr)})
		if err != nil {
			return []string{}, err
		}
		// get client public key
		log.Info().Msgf("Retrieving public key for teaam %s", evTag+"_"+team+"_"+strconv.Itoa(ipAddr))
		resp, err := ev.wg.GetPublicKey(ctx, &wg.PubKeyReq{PubKeyName: evTag + "_" + team + "_" + strconv.Itoa(ipAddr)})
		if err != nil {
			log.Error().Msgf("Error on GetPublicKey %v", err)
			return []string{}, err
		}

		//pIP := fmt.Sprintf("%d/32", len(ev.GetTeams())+2)
		peerIP := strings.Replace(subnet, "1/24", fmt.Sprintf("%d/32", ipAddr), 1)
		log.Info().Str("NIC", evTag).
			Str("AllowedIPs", peerIP).
			Str("PublicKey ", resp.Message).Msgf("Generating ip address for peer %s, ip address of peer is %s ", team, peerIP)
		addPeerResp, err := ev.wg.AddPeer(ctx, &wg.AddPReq{
			Nic:        evTag,
			AllowedIPs: peerIP,
			PublicKey:  resp.Message,
		})
		if err != nil {
			log.Error().Msgf("Error on adding peer to interface %v", err)
			return []string{}, err
		}
		log.Info().Str("Event: ", evTag).
			Str("Peer: ", team).Msgf("Message : %s", addPeerResp.Message)
		//get client privatekey
		log.Info().Msgf("Retrieving private key for team %s", team)
		teamPrivKey, err := ev.wg.GetPrivateKey(ctx, &wg.PrivKeyReq{PrivateKeyName: evTag + "_" + team + "_" + strconv.Itoa(ipAddr)})
		if err != nil {
			return []string{}, err
		}
		log.Info().Msgf("Privatee key for team %s is %s ", team, teamPrivKey.Message)
		log.Info().Msgf("Client configuration is created for server %s", endpoint)
		// creating client configuration file
		clientConfig := fmt.Sprintf(
			`[Interface]
Address = %s
PrivateKey = %s
DNS = 1.1.1.1
MTU = 1500
[Peer]
PublicKey = %s
AllowedIps = %s
Endpoint =  %s
PersistentKeepalive = 25


##### HOSTS INFORMATION ########
#   Append given IP Address(es) with Domain(s) to your /etc/hosts file
#   It enables you to browse domain of challenge through VPN 
#   when you connected to internet.
################################

%s
`, peerIP, teamPrivKey.Message, serverPubKey.Message, fmt.Sprintf("%s/24", labInfo.subnet), endpoint, hosts)
		t.SetVPNKeys(i, resp.Message)
		//log.Info().Msgf("Client configuration:\n %s\n", clientConfig)
		teamConfigFiles = append(teamConfigFiles, clientConfig)
	}

	return teamConfigFiles, nil
}

//Suspend function suspends event by using from event hub.
func (ev *event) Suspend(ctx context.Context) error {
	var teamLabSuspendError error
	if err := ev.labhub.Suspend(ctx); err != nil {
		return err
	}

	if err := ev.store.SetStatus(string(ev.store.Tag), int32(Suspended)); err != nil {
		return err
	}
	return teamLabSuspendError
}
func checkPort(port int) bool {
	portAllocated := fmt.Sprintf(":%d", port)
	// ensure that VPN port is free to allocate
	conn, _ := net.DialTimeout("tcp", portAllocated, time.Second)
	if conn != nil {
		_ = conn.Close()
		fmt.Printf("Checking VPN port %s\n", portAllocated)
		// true means port is already allocated
		return true
	}
	return false
}

//Resume function resumes event by using event hub
func (ev *event) Resume(ctx context.Context) error {
	var teamLabResumeError error
	if err := ev.labhub.Resume(ctx); err != nil {
		return err
	}

	// sets status of the event on haaukins store
	if err := ev.store.SetStatus(string(ev.store.Tag), int32(Running)); err != nil {
		return err
	}

	return teamLabResumeError
}

func (ev *event) Close() error {
	var waitGroup sync.WaitGroup

	for _, closer := range ev.closers {
		waitGroup.Add(1)
		go func(c io.Closer) {
			if err := c.Close(); err != nil {
				log.Warn().Msgf("error while closing event '%s': %s", ev.GetConfig().Name, err)
			}
			defer waitGroup.Done()
		}(closer)
	}
	waitGroup.Wait()
	if ev.store.OnlyVPN {
		ev.removeVPNConfs()
	}

	return nil
}

func (ev *event) removeVPNConfs() {
	evTag := string(ev.GetConfig().Tag)
	log.Debug().Msgf("Closing VPN connection for event %s", evTag)
	resp, err := ev.wg.ManageNIC(context.Background(), &wg.ManageNICReq{Cmd: "down", Nic: evTag})
	if err != nil {
		log.Error().Msgf("Error when disabling VPN connection for event %s", evTag)

	}
	if resp != nil {
		log.Info().Str("Message", resp.Message).Msgf("VPN connection is closed for event %s ", evTag)
	}
	//removeVPNConfigs removes all generated config files when Haaukins is stopped
	if err := removeVPNConfigs(ev.store.WireGuardConfig.Dir + evTag + "*"); err != nil {
		log.Error().Msgf("Error happened on deleting VPN configuration files for event %s on host  %v", evTag, err)
	}
}

func (ev *event) Finish(newTag string) {
	now := time.Now()
	err := ev.store.Finish(newTag, now)
	if err != nil {
		log.Warn().Msgf("error while archiving event: %s", err)
	}
}

func removeVPNConfigs(confFile string) error {
	log.Info().Msgf("Cleaning up VPN configuration files with following pattern { %s }", confFile)
	files, err := filepath.Glob(confFile)
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			log.Error().Msgf("Error removing file with name %s", f)
		}
	}
	return err
}

func (ev *event) createGuacConn(t *store.Team, lab lab.Lab) error {
	enableWallPaper := true
	rdpPorts := lab.RdpConnPorts()
	if n := len(rdpPorts); n == 0 {
		log.
			Debug().
			Int("amount", n).
			Msg("Too few RDP connections")

		return RdpConfErr
	}
	u := GuacUser{
		Username: t.Name(),
		Password: t.GetHashedPassword(),
	}

	if err := ev.guac.CreateUser(u.Username, u.Password); err != nil {
		log.
			Debug().
			Str("err", err.Error()).
			Msg("Unable to create guacamole user")
		return err
	}

	ev.guacUserStore.CreateUserForTeam(t.ID(), u)

	hostIp, err := ev.dockerHost.GetDockerHostIP()
	if err != nil {
		return err
	}

	for i, port := range rdpPorts {
		num := i + 1
		name := fmt.Sprintf("%s-client%d", t.ID(), num)

		log.Debug().Str("team", t.Name()).Uint("port", port).Msg("Creating RDP Connection for group")
		if err := ev.guac.CreateRDPConn(CreateRDPConnOpts{
			Host:            hostIp,
			Port:            port,
			Name:            name,
			GuacUser:        u.Username,
			Username:        &u.Username,
			Password:        &u.Password,
			EnableWallPaper: &enableWallPaper,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (ev *event) AssignLab(t *store.Team, lab lab.Lab) error {
	var hosts []string
	if !ev.store.OnlyVPN {
		if err := ev.createGuacConn(t, lab); err != nil {
			log.Error().Msgf("Error on creatig guacamole connection !, err : %v", err)
			return err
		}
		labInfo := &labNetInfo{
			dns:        lab.Environment().LabDNS(),
			subnet:     lab.Environment().LabSubnet(),
			dnsrecords: lab.Environment().DNSRecords(),
		}
		for _, r := range labInfo.dnsrecords {
			for ip, arecord := range r.Record {
				hosts = append(hosts, fmt.Sprintf("%s \t %s", ip, arecord))
			}
		}
		t.SetHostsInfo(hosts)
		log.Info().Str("Team DNS", labInfo.dns).
			Str("Team Subnet", labInfo.subnet).
			Msgf("Creating Guac connection for team %s", t.ID())

		//}
	} else {
		// create client configuration file for team
		labInfo := &labNetInfo{
			dns:        lab.Environment().LabDNS(),
			subnet:     lab.Environment().LabSubnet(),
			dnsrecords: lab.Environment().DNSRecords(),
		}

		log.Info().Str("Team DNS", labInfo.dns).
			Str("Team Subnet", labInfo.subnet).
			Msgf("Creating VPN connection for team %s", t.ID())

		clientConf, err := ev.CreateVPNConn(t, labInfo)
		if err != nil {
			return err
		}
		//todo[VPN]: update writeToFile function to take directory of conf files
		// writing configuration into file !
		//log.Info().Msgf("Client configuration\n %s ", clientConf)
		//client configuration is written to given dir with following pattern : <event-name>_<team-id>.conf
		for _, c := range clientConf {
			if err := writeToFile(ev.store.WireGuardConfig.Dir+string(ev.store.Tag)+"_"+t.ID()+".conf", c); err != nil {
				log.Error().Msgf("Configuration file create error %v", err)
			}
			log.Info().Msg("Client configuration file written to wg dir")
		}
		t.SetLabInfo(labInfo.subnet)
		t.SetVPNConn(clientConf)
	}

	ev.labs[t.ID()] = lab
	chals := lab.Environment().Challenges()

	for _, chal := range chals {
		tag, _ := store.NewTag(string(chal.Tag))
		_, _ = t.AddChallenge(store.Challenge{
			Tag:   tag,
			Name:  chal.Name,
			Value: chal.Value,
		})
		log.Info().Str("chal-tag", string(tag)).
			Str("chal-val", chal.Value).
			Msgf("Flag is created for team %s [assignlab function] ", t.Name())
	}
	t.CorrectedAssignedLab()
	return nil
}

func (ev *event) Handler() http.Handler {

	reghook := func(t *store.Team) error {
		select {
		case l, ok := <-ev.labhub.Queue():
			if !ok {
				return ErrMaxLabs
			}
			if err := ev.AssignLab(t, l); err != nil {
				return err
			}
		default:

			return ErrNoAvailableLabs
		}

		return nil
	}

	resetHook := func(t *store.Team, challengeTag string) error {
		teamLab, ok := ev.GetLabByTeam(t.ID())
		if !ok {
			return fmt.Errorf("Not found suitable team for given id: %s", t.ID())
		}
		if err := teamLab.Environment().ResetByTag(context.Background(), challengeTag); err != nil {
			return fmt.Errorf("Reset challenge hook error %v", err)
		}
		return nil
	}
	// resume labs in login of amigo
	resumeTeamLab := func(t *store.Team) error {
		var waitGroup sync.WaitGroup
		lab, ok := ev.GetLabByTeam(t.ID())
		if !ok {
			return errors.New("Lab could not found for given team, error on loginhook")
		}
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if err := lab.Resume(context.Background()); err != nil {
				log.Error().Msgf("Error on lab resume %v", err)
			}
		}()
		waitGroup.Wait()
		return nil
	}

	resetFrontendHook := func(t *store.Team) error {
		teamLab, ok := ev.GetLabByTeam(t.ID())
		if !ok {
			return fmt.Errorf("Not found suitable team for given id: %s", t.ID())
		}
		if err := teamLab.ResetFrontends(context.Background()); err != nil {
			return fmt.Errorf("Reset frontends hook error %v", err)
		}
		return nil
	}

	hooks := amigo.Hooks{AssignLab: reghook, ResetExercise: resetHook, ResetFrontend: resetFrontendHook, ResumeTeamLab: resumeTeamLab}

	guacHandler := ev.guac.ProxyHandler(ev.guacUserStore, ev.keyLoggerPool, ev.amigo, ev)(ev.store)

	return ev.amigo.Handler(hooks, guacHandler)
}

func (ev *event) GetHub() lab.Hub {
	return ev.labhub
}

func (ev *event) GetConfig() store.EventConfig {
	return ev.store.EventConfig
}

func (ev *event) GetTeams() []*store.Team {
	return ev.store.GetTeams()
}

func (ev *event) GetLabByTeam(teamId string) (lab.Lab, bool) {
	lab, ok := ev.labs[teamId]
	return lab, ok
}

func writeToFile(filename string, data string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.WriteString(file, data)
	if err != nil {
		return err
	}
	return file.Sync()
}

// creates range of ip addresses per event
func makeRange(min, max int) []int {
	a := make([]int, max-min+1)
	for i := range a {
		a[i] = min + i
	}
	return a
}

//pop function is somehow same with python pop function
func pop(alist *[]int) int {
	f := len(*alist)
	rv := (*alist)[f-1]
	*alist = append((*alist)[:f-1])
	return rv
}
