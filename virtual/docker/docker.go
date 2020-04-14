// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package docker

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"net"

	"io"

	"github.com/aau-network-security/haaukins/virtual"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var (
	DefaultClient     *docker.Client
	DefaultLinkBridge *defaultBridge

	TooLowMemErr              = errors.New("memory needs to be atleast 50mb")
	InvalidHostBindingErr     = errors.New("hostbing does not have correct format - (ip:)port")
	InvalidMountErr           = errors.New("incorrect mount format - src:dest")
	NoRegistriesToPullFromErr = errors.New("no registries to pull from")
	NoImageErr                = errors.New("unable to find image")
	EmptyDigestErr            = errors.New("empty digest")
	DigestFormatErr           = errors.New("unexpected digest format")
	NoRemoteDigestErr         = errors.New("unable to get digest from remote image")
	NoAvailableIPsErr         = errors.New("no available IPs")
	UnexpectedIPErr           = errors.New("unexpected IP range")
	ContNotCreatedErr         = errors.New("container is not created")

	Registries = map[string]docker.AuthConfiguration{
		"": {},
	}

	ipPool = newIPPoolFromHost()
)

func init() {
	var err error
	DefaultClient, err = docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	DefaultLinkBridge, err = newDefaultBridge("hkn-bridge")
	if err != nil {
		log.Fatal().Err(err).Msg("Error creating default bridge")
	}

	rand.Seed(time.Now().Unix())
}

type NoLocalDigestErr struct {
	img Image
}

func (err NoLocalDigestErr) Error() string {
	return fmt.Sprintf("unable to get digest from local image: %s", err.img.String())
}

type NoCredentialsErr struct {
	Registry string
}

func (err NoCredentialsErr) Error() string {
	return fmt.Sprintf("no credentials for registry: %s", err.Registry)
}

type NoLocalImageAvailableErr struct {
	err error
}

func (err NoLocalImageAvailableErr) Error() string {
	return fmt.Sprintf("no local image available: %s", err.err)
}

type NoRemoteImageAvailableErr struct {
	err error
}

func (err NoRemoteImageAvailableErr) Error() string {
	return fmt.Sprintf("failed to update local image to newest version from repository: %s", err.err)
}

type Host interface {
	GetDockerHostIP() (string, error)
}

func NewHost() Host {
	return &host{}
}

type host struct{}

func (h *host) GetDockerHostIP() (string, error) {
	return getDockerHostIP()
}

type Identifier interface {
	ID() string
}

type Container interface {
	Identifier
	virtual.Instance
	BridgeAlias(string) (string, error)
}

type ContainerConfig struct {
	Image        string
	EnvVars      map[string]string
	PortBindings map[string]string
	Labels       map[string]string
	Mounts       []string
	Resources    *Resources
	Cmd          []string
	DNS          []string
	UsedPorts    []string
	UseBridge    bool
}

type Resources struct {
	MemoryMB uint
	CPU      float64
}

type Image struct {
	Registry string
	Repo     string
	Tag      string
}

func (i Image) String() string {
	if i.Registry == "" {
		return i.Repo + ":" + i.Tag
	}

	return i.Registry + "/" + i.Repo + ":" + i.Tag
}

func (i Image) IsPublic() bool {
	return i.Registry == ""
}

func (i Image) NameWithReg() string {
	if i.Registry == "" {
		return i.Repo
	}

	return i.Registry + "/" + i.Repo
}

type container struct {
	id      string
	conf    ContainerConfig
	network *docker.Network
	linked  []Identifier
}

func NewContainer(conf ContainerConfig) Container {
	return &container{
		conf: conf,
	}
}

func (c *container) ID() string {
	return c.id
}

func (c *container) getCreateConfig() (*docker.CreateContainerOptions, error) {
	var env []string
	for k, v := range c.conf.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	bindings := make(map[docker.Port][]docker.PortBinding)
	for guestPort, hostListen := range c.conf.PortBindings {
		log.Debug().
			Str("guestPort", guestPort).
			Str("hostListen", hostListen).
			Msgf("Port bindings for new '%s' container", c.conf.Image)

		hostIP := ""
		hostPort := hostListen

		if strings.Contains(guestPort, "/") == false {
			log.Debug().Msgf("No protocol specified for portBind %s, defaulting to TCP.", guestPort)
			guestPort = guestPort + "/tcp"
		}

		if strings.Contains(hostListen, "/") {
			return nil, InvalidHostBindingErr
		}

		if strings.Contains(hostListen, ":") {
			parts := strings.Split(hostListen, ":")
			if len(parts) != 2 {
				return nil, InvalidHostBindingErr
			}

			hostIP = parts[0]
			hostPort = parts[1]
		}

		bindings[docker.Port(guestPort)] = []docker.PortBinding{
			{
				HostIP:   hostIP,
				HostPort: hostPort,
			},
		}
	}

	var mounts []docker.HostMount
	for _, mount := range c.conf.Mounts {
		parts := strings.Split(mount, ":")
		if len(parts) != 2 {
			return nil, InvalidMountErr
		}
		src, dest := parts[0], parts[1]

		mounts = append(mounts, docker.HostMount{
			Target: dest,
			Source: src,
			Type:   "bind",
		})

	}

	hostIP, err := getDockerHostIP()
	if err != nil {
		return nil, err
	}

	var swap int64 = 0
	hostConf := docker.HostConfig{
		ExtraHosts:       []string{fmt.Sprintf("host:%s", hostIP)},
		MemorySwap:       0,
		MemorySwappiness: &swap,
	}

	if c.conf.Resources != nil {
		if c.conf.Resources.MemoryMB > 0 {
			if c.conf.Resources.MemoryMB < 50 {
				return nil, TooLowMemErr
			}

			hostConf.Memory = int64(c.conf.Resources.MemoryMB) * 1024 * 1024
		}

		if c.conf.Resources.CPU > 0 {
			hostConf.CPUPeriod = 100000
			hostConf.CPUQuota = int64(100000 * c.conf.Resources.CPU)
		}
	}

	hostConf.PortBindings = bindings
	hostConf.Mounts = mounts

	if len(c.conf.DNS) > 0 {
		resolvPath, err := getResolvFile(c.conf.DNS)
		if err != nil {
			return nil, err
		}

		hostConf.Mounts = append(hostConf.Mounts, docker.HostMount{
			Target: "/etc/resolv.conf",
			Source: resolvPath,
			Type:   "bind",
		})
	}

	ports := make(map[docker.Port]struct{})
	for _, p := range c.conf.UsedPorts {
		ports[docker.Port(p)] = struct{}{}
	}

	img := parseImage(c.conf.Image)
	if err := verifyLocalImageVersion(img); err != nil {
		// we can proceed on several errors
		switch err.(type) {
		case NoLocalImageAvailableErr, NoCredentialsErr:
			return nil, err
		default:
			log.Warn().Msgf("failed to update local Docker image: %s", err)
		}
	}

	return &docker.CreateContainerOptions{
		Name: uuid.New().String(),
		Config: &docker.Config{
			Image:        c.conf.Image,
			Env:          env,
			Cmd:          c.conf.Cmd,
			Labels:       c.conf.Labels,
			ExposedPorts: ports,
		},
		HostConfig: &hostConf,
	}, nil
}

func (c *container) Create(ctx context.Context) error {
	dconf, err := c.getCreateConfig()
	if err != nil {
		return err
	}

	dconf.Context = ctx
	cont, err := DefaultClient.CreateContainer(*dconf)
	if err != nil {
		return err
	}

	if !c.conf.UseBridge {
		if err := DefaultClient.DisconnectNetwork("bridge", docker.NetworkConnectionOptions{
			Container: cont.ID,
		}); err != nil {
			return err
		}
	}

	log.Info().
		Str("ID", cont.ID[0:8]).
		Str("Image", c.conf.Image).
		Msg("Created new container")

	c.id = cont.ID

	return nil
}

func (c *container) Start(ctx context.Context) error {
	if c.id == "" {
		return ContNotCreatedErr
	}

	// If the docker is suspended unpause must be called instead
	if c.state() == virtual.Suspended {
		if err := DefaultClient.UnpauseContainer(c.id); err != nil {
			return err
		}
	} else {
		if err := DefaultClient.StartContainerWithContext(c.id, nil, ctx); err != nil {
			return err
		}
	}

	log.Debug().
		Str("ID", c.id[0:8]).
		Str("Image", c.conf.Image).
		Msg("Started container")

	return nil
}

func (c *container) Suspend(ctx context.Context) error {
	if err := DefaultClient.PauseContainer(c.id); err != nil {
		log.Error().Str("ID", c.id[0:8]).Msgf("Failed to suspend container: %s", err)
		return err
	}

	log.Debug().
		Str("ID", c.id[0:8]).
		Msg("Stopped container")

	return nil
}

func (c *container) Run(ctx context.Context) error {
	if err := c.Create(ctx); err != nil {
		return err
	}

	return c.Start(ctx)
}

func (c *container) Stop() error {
	if err := DefaultClient.StopContainer(c.id, 10); err != nil {
		return err
	}

	log.Debug().
		Str("ID", c.id[0:8]).
		Str("Image", c.conf.Image).
		Msg("Stopped container")

	return nil
}

func (c *container) state() virtual.State {
	cont, err := DefaultClient.InspectContainer(c.id)
	if err != nil {
		return virtual.Error
	}
	if cont.State.Paused {
		return virtual.Suspended
	}
	if cont.State.Running {
		return virtual.Running
	}
	return virtual.Stopped
}

func (c *container) Info() virtual.InstanceInfo {
	return virtual.InstanceInfo{
		Image: c.conf.Image,
		Type:  "docker",
		Id:    c.ID()[0:12],
		State: c.state(),
	}
}

func (c *container) Close() error {
	if c.network != nil {
		for _, cont := range append(c.linked, c) {
			DefaultClient.DisconnectNetwork(c.network.ID, docker.NetworkConnectionOptions{
				Container: cont.ID(),
			})
		}

		if err := DefaultClient.RemoveNetwork(c.network.ID); err != nil {
			return err
		}
	}

	removeContOpts := docker.RemoveContainerOptions{
		ID:            c.id,
		RemoveVolumes: true,
		Force:         true,
	}

	if err := DefaultLinkBridge.disconnect(c.id); err != nil {
		return err
	}

	err := DefaultClient.RemoveContainer(removeContOpts)
	if err != nil {
		return err
	}

	log.Debug().
		Str("ID", c.id[0:8]).
		Str("Image", c.conf.Image).
		Msg("Closed container")
	return nil
}

func (c *container) BridgeAlias(alias string) (string, error) {
	return DefaultLinkBridge.connect(c.id, alias)
}

type network struct {
	net       *docker.Network
	subnet    string
	ipPool    map[uint8]struct{}
	connected []Identifier
}

type Network interface {
	FormatIP(num int) string
	Interface() string
	Connect(c Container, ip ...int) (int, error)
	io.Closer
}

func NewNetwork() (Network, error) {
	sub, err := ipPool.Get()
	if err != nil {
		return nil, err
	}

	subnet := fmt.Sprintf("%s.0/24", sub)
	conf := docker.CreateNetworkOptions{
		Name:   uuid.New().String(),
		Driver: "macvlan",
		IPAM: &docker.IPAMOptions{
			Config: []docker.IPAMConfig{{
				Subnet: subnet,
			}},
		},
		Labels: map[string]string{
			"kn": "lab_network",
		},
	}

	netw, err := DefaultClient.CreateNetwork(conf)
	if err != nil {
		return nil, err
	}

	netInfo, _ := DefaultClient.NetworkInfo(netw.ID)
	subnet = netInfo.IPAM.Config[0].Subnet

	ipPool := make(map[uint8]struct{})
	for i := 30; i < 255; i++ {
		ipPool[uint8(i)] = struct{}{}
	}

	return &network{net: netw, subnet: subnet, ipPool: ipPool}, nil
}

func (n *network) Close() error {
	for _, cont := range n.connected {
		if err := DefaultClient.DisconnectNetwork(n.net.ID, docker.NetworkConnectionOptions{
			Container: cont.ID(),
		}); err != nil {
			continue
		}
	}

	return DefaultClient.RemoveNetwork(n.net.ID)
}

func (n *network) FormatIP(num int) string {
	return fmt.Sprintf("%s.%d", n.subnet[0:len(n.subnet)-5], num)
}

func (n *network) Interface() string {
	return fmt.Sprintf("dm-%s", n.net.ID[0:12])
}

func (n *network) getRandomIP() int {
	for randDigit, _ := range n.ipPool {
		delete(n.ipPool, randDigit)
		return int(randDigit)
	}
	return 0
}

func (n *network) releaseIP(ip string) {
	parts := strings.Split(ip, ".")
	strDigit := parts[len(parts)-1]

	num, err := strconv.Atoi(strDigit)
	if err != nil {
		return
	}

	n.ipPool[uint8(num)] = struct{}{}
}

func (n *network) Connect(c Container, ip ...int) (int, error) {
	var lastDigit int

	if len(ip) > 0 {
		lastDigit = ip[0]
	} else {
		lastDigit = n.getRandomIP()
	}

	ipAddr := n.FormatIP(lastDigit)

	err := DefaultClient.ConnectNetwork(n.net.ID, docker.NetworkConnectionOptions{
		Container: c.ID(),
		EndpointConfig: &docker.EndpointConfig{
			IPAMConfig: &docker.EndpointIPAMConfig{
				IPv4Address: ipAddr,
			},
			IPAddress: ipAddr,
		},
	})
	if err != nil {
		if len(ip) == 0 {
			n.releaseIP(ipAddr)
		}

		return lastDigit, err
	}

	n.connected = append(n.connected, c)

	return lastDigit, nil
}

type IPPool struct {
	m       sync.Mutex
	ips     map[string]struct{}
	weights map[string]int
}

func newIPPoolFromHost() *IPPool {
	ips := map[string]struct{}{}
	weights := map[string]int{
		"172": 7 * 255,   // 172.{2nd}.{0-255}.{0-255} => 2nd => 25-31 => 6 + 1 => 7
		"10":  255 * 255, // 10.{2nd}.{0-255}.{0-255} => 2nd => 0-254 => 254 + 1 => 255
		"192": 1 * 255,   // 10.{2nd}.{0-255}.{0-255} => 2nd => 198-198 => 0 + 1 => 1
	}

	ifaces, err := net.Interfaces()
	if err == nil {
		for _, i := range ifaces {
			addrs, err := i.Addrs()
			if err != nil {
				continue
			}

			for _, a := range addrs {
				addr, ok := a.(*net.IPNet)
				if !ok {
					continue
				}

				if addr.IP.To4() == nil {
					// not v4
					continue
				}

				ipParts := strings.Split(addr.IP.String(), ".")
				lvl1 := ipParts[0]
				if _, ok = weights[lvl1]; !ok {
					// not relevant ip
					continue
				}

				ipStr := strings.Join(ipParts[0:3], ".")
				ips[ipStr] = struct{}{}

				weights[lvl1] = weights[lvl1] - 1
			}
		}
	}

	return &IPPool{
		ips:     ips,
		weights: weights,
	}
}

func (ipp *IPPool) Get() (string, error) {
	ipp.m.Lock()
	defer ipp.m.Unlock()

	if len(ipp.ips) > 60000 {
		return "", NoAvailableIPsErr
	}

	genIP := func() string {
		ip := randomPickWeighted(ipp.weights)
		switch ip {
		case "172":
			ip += fmt.Sprintf(".%d", rand.Intn(6)+25)
		case "192":
			ip += ".168"
		case "10":
			ip += fmt.Sprintf(".%d", rand.Intn(255))
		}

		ip += fmt.Sprintf(".%d", rand.Intn(255))

		return ip
	}

	var ip string
	exists := true
	for exists {
		ip = genIP()
		_, exists = ipp.ips[ip]
	}

	ipp.ips[ip] = struct{}{}

	return ip, nil
}

func randomPickWeighted(m map[string]int) string {
	var totalWeight int
	for _, w := range m {
		totalWeight += w
	}

	r := rand.Intn(totalWeight)

	for k, w := range m {
		r -= w
		if r <= 0 {
			return k
		}
	}

	return ""
}

type defaultBridge struct {
	m          sync.Mutex
	id         string
	containers map[string]string
}

func newDefaultBridge(name string) (*defaultBridge, error) {
	var netID string

	networks, err := DefaultClient.ListNetworks()
	if err != nil {
		return nil, err
	}
	for _, n := range networks {
		if n.Name == name {
			netID = n.ID
			break
		}
	}

	if netID == "" {
		createNetworkOpts := docker.CreateNetworkOptions{
			Name:     name,
			Driver:   "bridge",
			Internal: true,
			Labels: map[string]string{
				"hkn": "default_bridge",
			},
		}

		net, err := DefaultClient.CreateNetwork(createNetworkOpts)
		if err != nil {
			return nil, err
		}

		netID = net.ID
	}

	return &defaultBridge{
		id:         netID,
		containers: map[string]string{},
	}, nil
}

func (dbr *defaultBridge) connect(cid string, alias string) (string, error) {
	dbr.m.Lock()
	defer dbr.m.Unlock()
	knownAlias, ok := dbr.containers[cid]
	if ok {
		return knownAlias, nil
	}

	if alias == "" {
		alias = strings.Replace(uuid.New().String(), "-", "", -1)
	}
	err := DefaultClient.ConnectNetwork(dbr.id, docker.NetworkConnectionOptions{
		Container: cid,
		EndpointConfig: &docker.EndpointConfig{
			Aliases: []string{alias},
		},
	})
	if err != nil {
		return "", err
	}

	dbr.containers[cid] = alias

	return alias, nil
}

func (dbr *defaultBridge) disconnect(cid string) error {
	dbr.m.Lock()
	_, present := dbr.containers[cid]
	delete(dbr.containers, cid)
	dbr.m.Unlock()

	if !present {
		return nil
	}

	return DefaultClient.DisconnectNetwork(dbr.id, docker.NetworkConnectionOptions{
		Container: cid,
	})
}

func (dbr *defaultBridge) Close() error {
	return DefaultClient.RemoveNetwork(dbr.id)
}

func getResolvFile(ns []string) (string, error) {
	sort.Strings(ns)
	s := md5.Sum([]byte(strings.Join(ns, ",")))

	path := filepath.Join("/tmp", fmt.Sprintf("resolvconf-%x", s))

	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	for _, nameserver := range ns {
		_, err := f.WriteString(fmt.Sprintf("nameserver %s", nameserver) + "\n")
		if err != nil {
			return "", err
		}
	}

	return path, nil
}

func parseImage(img string) Image {
	tag := "latest"
	repo := img
	registry := ""

	parts := strings.Split(img, ":")
	if len(parts) == 2 {
		repo, tag = parts[0], parts[1]
	}

	// format: reg/owner/repo
	if strings.Count(repo, "/") > 1 {
		parts = strings.Split(repo, "/")

		registry = parts[0]
		repo = strings.Join(parts[1:len(parts)], "/")
	}

	return Image{
		Registry: registry,
		Repo:     repo,
		Tag:      tag,
	}
}

func getDockerHostIP() (string, error) {
	i, err := net.InterfaceByName("docker0")
	if err != nil {
		return "", err
	}

	addrs, err := i.Addrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		switch addr.(type) {
		case *net.IPNet:
			rawIP, _ := addr.(*net.IPNet)
			ip, _, err := net.ParseCIDR(rawIP.String())
			if err != nil {
				return "", err
			}
			if ip.To4() != nil {
				return ip.String(), nil
			}
		}
	}

	return "", nil
}

func getRemoteDigestForImage(auth docker.AuthConfiguration, img Image) (string, error) {
	lookupDigest := func(req *http.Request) (string, error) {
		ctx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
		defer cancel()
		req = req.WithContext(ctx)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}

		hash := resp.Header.Get("Docker-Content-Digest")
		if hash == "" {
			return "", EmptyDigestErr
		}

		return hash, nil
	}

	digestRequestFromURL := func(URL string) (*http.Request, error) {
		req, err := http.NewRequest("HEAD", URL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
		return req, nil
	}

	path := fmt.Sprintf("/v2/%s/manifests/%s", img.Repo, img.Tag)
	if img.Registry == "" {
		resp, err := http.Get(fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", img.Repo))
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		var msg struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
			return "", err
		}

		req, err := digestRequestFromURL("https://registry.hub.docker.com" + path)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+msg.Token)

		return lookupDigest(req)
	}

	req, err := digestRequestFromURL(fmt.Sprintf("https://%s%s", auth.ServerAddress, path))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(auth.Username, auth.Password)

	return lookupDigest(req)
}

func retrieveImage(auth docker.AuthConfiguration, img Image) error {
	log.Debug().
		Str("image", img.String()).
		Msg("Attempting to pull image")

	if err := DefaultClient.PullImage(docker.PullImageOptions{
		Repository: img.NameWithReg(),
		Tag:        img.Tag,
	}, auth); err != nil {
		return err
	}

	return nil
}

func verifyLocalImageVersion(img Image) error {
	creds, ok := Registries[img.Registry]
	if !ok {
		return NoCredentialsErr{img.Registry}
	}

	localImg, err := DefaultClient.InspectImage(img.String())
	if err != nil {
		if err == docker.ErrNoSuchImage {
			if err := retrieveImage(creds, img); err != nil {
				return NoLocalImageAvailableErr{err}
			}
			return nil
		}
		return err
	}

	if len(localImg.RepoDigests) == 0 {
		return NoLocalDigestErr{img}
	}

	localDigest := localImg.RepoDigests[0]
	if strings.Contains(localDigest, "@") {
		localDigest = strings.Split(localDigest, "@")[1]
	}

	remoteDigest, err := getRemoteDigestForImage(creds, img)
	if err != nil {
		return err
	}

	if remoteDigest != localDigest {
		if err := retrieveImage(creds, img); err != nil {
			return err
		}
	}

	return nil
}
