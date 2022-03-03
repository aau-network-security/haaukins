// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package guacamole

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/aau-network-security/haaukins/exercise"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	eproto "github.com/aau-network-security/haaukins/exercise/ex-proto"
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

const (
	vpnInfo          = "https://gitlab.com/-/snippets/2096681/raw/master/instructions.txt"
	installWireguard = "https://gitlab.com/-/snippets/2102002/raw/master/install_wireguard.sh"
	connectWireguard = "https://gitlab.com/-/snippets/2102000/raw/master/connectwireguard.py"
)

type Host interface {
	CreateEventFromEventDB(context.Context, store.EventConfig, string) (Event, error)
	CreateEventFromConfig(context.Context, store.EventConfig, string) (Event, error)
}

func NewHost(vlib vbox.Library, elib eproto.ExerciseStoreClient, eDir string, dbc pbc.StoreClient, config wg.WireGuardConfig) Host {
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
	elib      eproto.ExerciseStoreClient
	vpnConfig wg.WireGuardConfig
	dir       string
}

//Create the event configuration for the event got from the DB
func (eh *eventHost) CreateEventFromEventDB(ctx context.Context, conf store.EventConfig, reCaptchaKey string) (Event, error) {
	var labConf lab.Config
	var exers []store.Exercise
	var exercises []string
	disabledChals := make(map[string][]string, len(exers))
	allChals := make(map[string][]string, len(exers))
	for _, d := range conf.Lab.Exercises {
		exercises = append(exercises, string(d))
	}
	exer, err := eh.elib.GetExerciseByTags(ctx, &eproto.GetExerciseByTagsRequest{Tag: exercises})
	if err != nil {
		return nil, fmt.Errorf("[exercises-service] error %v", err)
	}

	log.Debug().Str("event name ", conf.Name).
		Str("event tag", string(conf.Tag)).
		Str("event createdby", conf.CreatedBy).
		Msgf("Exercises are retrieved from exercise-service")
	for _, e := range exer.Exercises {
		exercise, err := protobufToJson(e)
		if err != nil {
			return nil, err
		}
		estruct := store.Exercise{}
		json.Unmarshal([]byte(exercise), &estruct)
		exers = append(exers, estruct)
	}
	labConf.Exercises = exers
	for _, e := range conf.Lab.DisabledExercises {
		disabledChals[string(e)] = labConf.GetChildrenChallenges(string(e))
	}
	for _, e := range conf.Lab.Exercises {
		allChals[string(e)] = labConf.GetChildrenChallenges(string(e))
	}
	conf.AllChallenges = allChals
	conf.DisabledChallenges = disabledChals
	es, err := store.NewEventStore(conf, eh.dir, eh.dbc)
	if err != nil {
		return nil, err
	}

	if conf.OnlyVPN == docker.OnlyVPN || conf.OnlyVPN == docker.VPNBrowser {
		labConf.DisabledExercises = conf.Lab.DisabledExercises
		es.OnlyVPN = conf.OnlyVPN
		es.WireGuardConfig = eh.vpnConfig
		if conf.OnlyVPN == docker.VPNBrowser {
			labConf.Frontends = conf.Lab.Frontends
		}
	} else {
		labConf.DisabledExercises = conf.Lab.DisabledExercises
		labConf.Frontends = conf.Lab.Frontends
	}

	flags := labConf.Flags()

	lh := lab.LabHost{
		Vlib: eh.vlib,
		Conf: labConf,
	}
	hub, err := lab.NewHub(&lh, conf.Available, conf.Capacity, conf.OnlyVPN)
	if err != nil {
		return nil, err
	}

	return NewEvent(eh.ctx, es, hub, flags, reCaptchaKey)
}

func protobufToJson(message proto.Message) (string, error) {
	marshaler := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "  ",
	}

	return marshaler.MarshalToString(message)
}

//Save the event in the DB and create the event configuration
func (eh *eventHost) CreateEventFromConfig(ctx context.Context, conf store.EventConfig, reCaptchaKey string) (Event, error) {
	var exercises []string
	var disabledExercises []string
	// todo: update this in more elegant way
	for _, e := range conf.Lab.Exercises {
		exercises = append(exercises, string(e))
	}
	for _, e := range conf.Lab.DisabledExercises {
		disabledExercises = append(disabledExercises, string(e))
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
		SecretKey:          conf.SecretKey,
		DisabledExercises:  strings.Join(disabledExercises, ","),
	})

	log.Debug().Str("event tag", string(conf.Tag)).
		Str("event name", conf.Name).
		Str("event created by", conf.CreatedBy).
		Msgf("Event is saved to database")

	if err != nil {
		return nil, err
	}

	return eh.CreateEventFromEventDB(ctx, conf, reCaptchaKey)
}

type Event interface {
	Start(context.Context) error
	Close() error
	Suspend(context.Context) error
	Resume(context.Context) error

	Finish(string)
	AssignLab(*store.Team, lab.Lab) error
	Handler() http.Handler
	PauseSignup(bool)
	AddNotification(message string, loggedInUsers bool) error
	SetStatus(int32)
	GetStatus() int32
	GetConfig() store.EventConfig
	GetTeams() []*store.Team
	GetTeamById(teamId string) (*store.Team, error)
	GetHub() lab.Hub
	UpdateTeamPassword(id, pass, passRepeat string) (string, error)
	GetLabByTeam(teamId string) (lab.Lab, bool)
	GetAssignedLabs() map[string]lab.Lab
	GetFrontendData() *amigo.FrontendData
	DeleteTeam(id string) (bool, error)
}

type event struct {
	amigo         *amigo.Amigo
	guac          Guacamole
	labhub        lab.Hub
	ipT           IPTables
	labs          map[string]lab.Lab
	store         store.Event
	keyLoggerPool KeyLoggerPool

	ipRules       map[string]ipRules
	ipAddrs       []int
	wg            wg.WireguardClient
	guacUserStore *GuacUserStore
	dockerHost    docker.Host

	closers []io.Closer
}

type ipRules struct {
	labsubnet string
	vpnIps    string
}

type labNetInfo struct {
	dns             string
	subnet          string
	dnsrecords      []*exercise.DNSRecord
	wgInterfacePort int
}

func NewEvent(ctx context.Context, e store.Event, hub lab.Hub, flags []store.ChildrenChalConfig, reCaptchaKey string) (Event, error) {
	guac, err := New(ctx, Config{}, e.OnlyVPN, string(e.Tag))
	if err != nil {
		return nil, err
	}
	// New wireguard gRPC client connection
	wgClient, err := wg.NewGRPCVPNClient(e.WireGuardConfig)
	if err != nil {
		log.Error().Msgf("Connection error on wireguard service error %v ", err)
		return nil, err
	}

	ipT := IPTables{
		sudo:     true,
		execFunc: shellExec,
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

	var eventVPNIPs []int
	ipAddrs := makeRange(2, 254)
	for i := 0; i < 4; i++ {
		eventVPNIPs = append(eventVPNIPs, ipAddrs...)
	}

	ev := &event{
		store:         e,
		labhub:        hub,
		amigo:         amigo.NewAmigo(e, flags, reCaptchaKey, wgClient, amigoOpt),
		guac:          guac,
		ipAddrs:       eventVPNIPs,
		labs:          map[string]lab.Lab{},
		guacUserStore: NewGuacUserStore(),
		wg:            wgClient,
		closers:       []io.Closer{guac, hub, keyLoggerPool},
		dockerHost:    dockerHost,
		keyLoggerPool: keyLoggerPool,
		ipT:           ipT,
		ipRules:       map[string]ipRules{},
	}

	return ev, nil
}

func (ev *event) AddNotification(message string, loggedInUsers bool) error {
	notification := amigo.Notification{
		Message:       message,
		LoggedInUsers: loggedInUsers,
	}
	ev.amigo.SetNotification(notification)
	return nil
}

func (ev *event) PauseSignup(pause bool) {
	ev.amigo.PauseSignup(pause)
}

func (ev *event) GetFrontendData() *amigo.FrontendData {
	return ev.amigo.FrontEndData
}
func (ev *event) GetAssignedLabs() map[string]lab.Lab {
	return ev.labs
}

// SetStatus sets status of event in cache
func (ev *event) SetStatus(state int32) {
	ev.store.Status = state
}

func (ev *event) GetStatus() int32 {
	return ev.store.Status
}

func (ev *event) UpdateTeamPassword(id, pass, passRepeat string) (string, error) {
	tm, err := ev.store.TeamStore.GetTeamByID(id)
	if err != nil {
		return "", err
	}
	if err := tm.UpdatePass(pass, passRepeat, string(ev.store.Tag)); err != nil {
		return "", err
	}

	return fmt.Sprintf("Password for team [ %s ] is updated ! ", id), nil
}

func (ev *event) DeleteTeam(id string) (bool, error) {
	t, err := ev.GetTeamById(id)
	if err != nil {
		return false, err
	}
	if err := ev.store.DeleteTeam(t.ID(), string(ev.GetConfig().Tag)); err != nil {
		log.Debug().Msgf("Error on DeleteTeam: [ %s ] ", err.Error())
		return false, err
	}
	return true, nil
}

func (ev *event) Start(ctx context.Context) error {
	if ev.store.OnlyVPN == docker.OnlyVPN || ev.store.OnlyVPN == docker.VPNBrowser {
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

		if err := ev.guac.Start(ctx); err != nil {
			log.
				Error().
				Err(err).
				Msg("error starting guac")

			return StartingGuacErr
		}

		if err != nil {
			// handle error
			log.Debug().Msgf("Initializing interface %s for wireguard failed , VPN connection will not be available: %v\n", ev.store.VPNAddress, err)
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
			log.
				Debug().
				Err(err).
				Msgf("lab issue for team %s", team.ID())
			return err
		}

	}

	return nil
}

//CreateVPNConn will generate VPN Connection configuration per team.
func (ev *event) CreateVPNConn(t *store.Team, labInfo *labNetInfo) ([]string, error) {
	var teamConfigFiles []string
	lowBound := 240
	upperBound := 244
	ctx := context.Background()
	var vpnIPs []string
	vpnInstructions := getContent(vpnInfo)
	evTag := string(ev.GetConfig().Tag)
	teamID := t.ID()
	var hosts string
	for _, r := range labInfo.dnsrecords {
		for ip, arecord := range r.Record {
			hosts += fmt.Sprintf("# %s \t %s \n", ip, arecord)
		}

	}
	labSubnet := fmt.Sprintf("%s/24", labInfo.subnet)
	// random.random.240.1/22
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
	for i := lowBound; i < upperBound; i++ {

		// generate client privatekey
		ipAddr := pop(&ev.ipAddrs)

		log.Info().Msgf("Generating privatekey for team %s", evTag+"_"+teamID)
		_, err = ev.wg.GenPrivateKey(ctx, &wg.PrivKeyReq{PrivateKeyName: evTag + "_" + teamID + "_" + strconv.Itoa(ipAddr)})
		if err != nil {
			return []string{}, err
		}

		// generate client public key
		log.Info().Msgf("Generating public key for team %s", evTag+"_"+teamID+"_"+strconv.Itoa(ipAddr))
		_, err = ev.wg.GenPublicKey(ctx, &wg.PubKeyReq{PubKeyName: evTag + "_" + teamID + "_" + strconv.Itoa(ipAddr), PrivKeyName: evTag + "_" + teamID + "_" + strconv.Itoa(ipAddr)})
		if err != nil {
			return []string{}, err
		}
		// get client public key
		log.Info().Msgf("Retrieving public key for teaam %s", evTag+"_"+teamID+"_"+strconv.Itoa(ipAddr))
		resp, err := ev.wg.GetPublicKey(ctx, &wg.PubKeyReq{PubKeyName: evTag + "_" + teamID + "_" + strconv.Itoa(ipAddr)})
		if err != nil {
			log.Error().Msgf("Error on GetPublicKey %v", err)
			return []string{}, err
		}

		peerIP := strings.Replace(subnet, "240.1/22", fmt.Sprintf("%d.%d/32", i, ipAddr), 2)
		gwIP := strings.Replace(subnet, "1/22", fmt.Sprintf("1/32"), 1)
		log.Info().Str("NIC", evTag).
			Str("AllowedIPs", peerIP).
			Str("PublicKey ", resp.Message).Msgf("Generating ip address for peer %s, ip address of peer is %s ", teamID, peerIP)
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
			Str("Peer: ", teamID).Msgf("Message : %s", addPeerResp.Message)
		//get client privatekey
		log.Info().Msgf("Retrieving private key for team %s", teamID)
		teamPrivKey, err := ev.wg.GetPrivateKey(ctx, &wg.PrivKeyReq{PrivateKeyName: evTag + "_" + teamID + "_" + strconv.Itoa(ipAddr)})
		if err != nil {
			return []string{}, err
		}
		log.Info().Msgf("Private key for team %s is %s ", teamID, teamPrivKey.Message)
		log.Info().Msgf("Client configuration is created for server %s", endpoint)
		// creating client configuration file
		clientConfig := fmt.Sprintf(
			`[Interface]
Address = %s
PrivateKey = %s
DNS = 1.1.1.1
MTU = 1420
[Peer]
PublicKey = %s
AllowedIps = %s,%s
Endpoint =  %s
PersistentKeepalive = 25

# --------------------------------------------------------------------------
#  YOUR LAB SUBNET IS:  %s 													
# --------------------------------------------------------------------------

######### << USER SCRIPTS >> #####
#  
#	Use following scripts to install wireguard and connect to lab. 
#
#   Install Wireguard: %s 
#  	
#	Connect Event:  %s
#
#   The scripts are automating steps which you do manually. Use them with your responsibility.
#   If you notice outdated information, help us to update it :) 
#
####################

##### HOSTS INFORMATION #############
#   Append given IP Address(es) with Domain(s) to your /etc/hosts file
#   It enables you to browse domain of challenge through VPN 
#   when you connected to internet. 
#   (* If you used the script given above, your /etc/hosts file is updated already, you can skip this.)
###################################

%s

####### SETTING VPN CONFIGURATION #########

%s

`, peerIP, teamPrivKey.Message, serverPubKey.Message, labSubnet, gwIP, endpoint, labSubnet, installWireguard, connectWireguard, hosts, vpnInstructions)
		t.SetVPNKeys(i, resp.Message)
		teamConfigFiles = append(teamConfigFiles, clientConfig)
		vpnIPs = append(vpnIPs, peerIP)
	}

	vpnIPs = append(vpnIPs, labSubnet)
	ev.ipT.createRejectRule(labSubnet)
	ev.ipT.createStateRule(labSubnet)
	ev.ipT.createAcceptRule(labSubnet, strings.Join(vpnIPs, ","))
	ev.ipRules[teamID] = ipRules{
		labsubnet: labSubnet,
		vpnIps:    strings.Join(vpnIPs, ","),
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
	if ev.store.OnlyVPN == docker.OnlyVPN || ev.store.OnlyVPN == docker.VPNBrowser {
		ev.removeVPNConfs()
		ev.removeIPTableRules()
	}

	return nil
}

func (ev *event) removeIPTableRules() {
	for tid, ipR := range ev.ipRules {
		log.Debug().Str("Team ID ", tid).Msgf("iptables are removing... ")
		ev.ipT.removeRejectRule(ipR.labsubnet)
		ev.ipT.removeStateRule(ipR.labsubnet)
		ev.ipT.removeAcceptRule(ipR.labsubnet, ipR.vpnIps)
	}
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
	enableDrive := true
	createDrivePath := true
	// Drive path is the home folder inside the docker guacamole
	drivePath := "/home/" + t.ID()
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
			EnableDrive:     &enableDrive,
			CreateDrivePath: &createDrivePath,
			DrivePath:       &drivePath,
		}); err != nil {
			return err
		}
	}

	instanceInfo := lab.InstanceInfo()
	// Will not handle error below since this is not a critical function
	_ = vbox.CreateUserFolder(t.ID(), string(ev.store.Tag))

	_ = vbox.CreateFolderLink(instanceInfo[0].Id, string(ev.store.Tag), t.ID())

	return nil
}

func getDNSRecords(l []*exercise.DNSRecord) []string {
	var hosts []string
	for _, r := range l {
		for ip, arecord := range r.Record {
			hosts = append(hosts, fmt.Sprintf("%s \t %s", ip, arecord))
		}
	}
	return hosts
}

func (ev *event) AssignLab(t *store.Team, lab lab.Lab) error {
	var hosts []string
	labInfo := &labNetInfo{
		dns:        lab.Environment().LabDNS(),
		subnet:     lab.Environment().LabSubnet(),
		dnsrecords: lab.Environment().DNSRecords(),
	}
	hosts = getDNSRecords(labInfo.dnsrecords)

	if ev.store.OnlyVPN == docker.NoVPN {
		if err := ev.createGuacConn(t, lab); err != nil {
			log.Error().Msgf("Error on creating guacamole connection !, err : %v", err)
			return err
		}

		t.SetHostsInfo(hosts)
		log.Info().Str("Team DNS", labInfo.dns).
			Str("Team Subnet", labInfo.subnet).
			Msgf("Creating Guac connection for team %s", t.ID())

	} else {
		log.Info().Str("Team DNS", labInfo.dns).
			Str("Team Subnet", labInfo.subnet).
			Msgf("Creating VPN connection for team %s", t.ID())
		if ev.store.OnlyVPN == docker.VPNBrowser {
			log.Debug().Msg("Creating guacamole for VPN + Browser events ")
			if err := ev.createGuacConn(t, lab); err != nil {
				log.Error().Msgf("Error on creating guacamole connection !, err : %v", err)
				return err
			}
		}
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
		hosts = getDNSRecords(labInfo.dnsrecords)
		t.SetLabInfo(labInfo.subnet)
		t.SetVPNConn(clientConf)
		t.SetHostsInfo(hosts)
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
		teamDisabledMap := t.GetDisabledChalMap()
		_, ok = teamDisabledMap[challengeTag]
		if ok {
			if t.ManageDisabledChals(challengeTag) {
				log.Printf("Disabled exercises updated [ %s ] removed from disabled exercises by team [ %s ] ", challengeTag, t.ID())
			}
		}
		return nil
	}
	// state 0 : running
	// state 1 : stopped

	startStopHook := func(t *store.Team, challengeTag string, stopped bool) error {
		var waitGroup sync.WaitGroup
		var startStopErr error
		teamLab, ok := ev.GetLabByTeam(t.ID())
		if !ok {
			fmt.Errorf("Not found suitable team for given id: %s", t.ID())
		}
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if stopped {
				if err := teamLab.Environment().StartByTag(context.TODO(), challengeTag); err != nil {
					startStopErr = err
				}
			} else {
				if err := teamLab.Environment().StopByTag(challengeTag); err != nil {
					startStopErr = err
				}
			}
		}()
		waitGroup.Wait()
		return startStopErr
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
		if err := teamLab.ResetFrontends(context.Background(), string(ev.store.Tag), t.ID()); err != nil {
			return fmt.Errorf("Reset frontends hook error %v", err)
		}
		return nil
	}

	hooks := amigo.Hooks{
		AssignLab:         reghook,
		ResetExercise:     resetHook,
		StartStopExercise: startStopHook,
		ResetFrontend:     resetFrontendHook,
		ResumeTeamLab:     resumeTeamLab,
	}

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

func (ev *event) GetTeamById(teamid string) (*store.Team, error) {
	t, err := ev.store.GetTeamByID(teamid)
	if err != nil {
		return nil, err
	}
	return t, nil
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

// get page content
func getContent(link string) string {
	res, err := http.Get(link)
	if err != nil {
		log.Debug().Msgf("Error on retrieving link [ %s ] Err: [ %v ]", link, err)
	}
	content, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Debug().Msgf("Read error content [ %s ] Err: [ %v ]", link, err)
	}
	return string(content)
}
