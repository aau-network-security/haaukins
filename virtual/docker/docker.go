package docker

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var (
	DefaultClient, dockerErr  = docker.NewClient("unix:///var/run/docker.sock")
	TooLowMemErr              = errors.New("Memory needs to be atleast 50mb")
	InvalidHostBindingErr     = errors.New("Hostbing does not have correct format - (ip:)port")
	InvalidMountErr           = errors.New("Incorrect mount format - src:dest")
	NoRegistriesToPullFromErr = errors.New("No registries to pull from")
	NoImageErr                = errors.New("Unable to find image")
	EmptyDigestErr            = errors.New("Empty digest")
	DigestFormatErr           = errors.New("Unexpected digest format")
	NoDigestDockerHubErr      = errors.New("Unable to get digest from docker hub")

	Registries = map[string]docker.AuthConfiguration{
		"": {},
	}
)

func init() {
	if dockerErr != nil {
		log.Fatal().Err(dockerErr)
	}

	rand.Seed(time.Now().Unix())
}

type Identifier interface {
	ID() string
}

type Container interface {
	Identifier
	Start() error
	Stop() error
	Close() error
	Link(Identifier, string) error
}

type ContainerConfig struct {
	Image        string
	EnvVars      map[string]string
	PortBindings map[string]string
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

type container struct {
	id      string
	conf    ContainerConfig
	network *docker.Network
	linked  []Identifier
}

func NewContainer(conf ContainerConfig) (Container, error) {
	var env []string
	for k, v := range conf.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	bindings := make(map[docker.Port][]docker.PortBinding)
	for guestPort, hostListen := range conf.PortBindings {
		log.Debug().
			Str("guestPort", guestPort).
			Str("hostListen", hostListen).
			Msgf("Port bindings for new '%s' container", conf.Image)

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
	for _, mount := range conf.Mounts {
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

	hostIP, err := GetDockerHostIP()
	if err != nil {
		return nil, err
	}

	hostConf := docker.HostConfig{
		ExtraHosts:       []string{fmt.Sprintf("host:%s", hostIP)},
		MemorySwap:       0,
		MemorySwappiness: 0,
	}

	if conf.Resources != nil {
		if conf.Resources.MemoryMB > 0 {
			if conf.Resources.MemoryMB < 50 {
				return nil, TooLowMemErr
			}

			hostConf.Memory = int64(conf.Resources.MemoryMB) * 1024 * 1024
		}

		if conf.Resources.CPU > 0 {
			hostConf.CPUPeriod = 100000
			hostConf.CPUQuota = int64(100000 * conf.Resources.CPU)
		}
	}

	hostConf.PortBindings = bindings
	hostConf.Mounts = mounts

	if len(conf.DNS) > 0 {
		resolvPath, err := getResolvFile(conf.DNS)
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
	for _, p := range conf.UsedPorts {
		ports[docker.Port(p)] = struct{}{}
	}

	if err := ensureImage(conf.Image); err != nil {
		return nil, err
	}

	createContOpts := docker.CreateContainerOptions{
		Name: uuid.New().String(),
		Config: &docker.Config{
			Image:        conf.Image,
			Env:          env,
			Cmd:          conf.Cmd,
			ExposedPorts: ports,
		},
		HostConfig: &hostConf,
	}

	cont, err := DefaultClient.CreateContainer(createContOpts)
	if err != nil {
		return nil, err
	}

	if !conf.UseBridge {
		if err := DefaultClient.DisconnectNetwork("bridge", docker.NetworkConnectionOptions{
			Container: cont.ID,
		}); err != nil {
			return nil, err
		}
	}

	log.Info().
		Str("ID", cont.ID[0:8]).
		Str("Image", conf.Image).
		Msg("Created new container")

	return &container{
		id:   cont.ID,
		conf: conf,
	}, nil
}

func digestRemoteImg(img Image, reg docker.AuthConfiguration) (string, error) {
	url := fmt.Sprintf("https://%s/v2/%s/manifests/%s", reg.ServerAddress, img.Repo, img.Tag)

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	req.SetBasicAuth(reg.Username, reg.Password)

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

	log.
		Debug().
		Str("digest", hash[0:12]).
		Str("image", img.String()).
		Msg("Retrieved digest")

	return hash, nil
}

func pullImage(img Image, reg docker.AuthConfiguration) error {

	log.Debug().
		Str("image", img.String()).
		Msg("Attempting to pull image")

	if err := DefaultClient.PullImage(docker.PullImageOptions{
		Repository: img.NameWithReg(),
		Tag:        img.Tag,
	}, reg); err != nil {
		return err
	}

	return nil
}

func ensureImage(imgStr string) error {
	img := parseImage(imgStr)

	dImg, err := DefaultClient.InspectImage(img.String())
	foundLocal := dImg != nil && err != docker.ErrNoSuchImage

	if img.IsPublic() {
		if !foundLocal {
			return pullImage(img, docker.AuthConfiguration{})
		}

		if err != nil {
			return err
		}

		return nil
	}

	creds, hasCreds := Registries[img.Registry]
	var rdig string
	for i := 0; i < 3 && hasCreds && rdig == ""; i++ {
		rdig, err = digestRemoteImg(img, creds)
		if err != nil {
			continue
		}
	}

	var newVersion bool
	if rdig != "" && foundLocal {
		ldig := dImg.RepoDigests[0]
		if strings.Contains(ldig, "@") {
			ldig = strings.Split(ldig, "@")[1]
		}

		if rdig == ldig {
			newVersion = true
		}
	}

	if newVersion {
		// newVersion is only set if a local image exists AND a new digest has been observed
		if err := pullImage(img, creds); err != nil {
			log.Warn().
				Err(err).
				Str("image", img.String()).
				Msg("Attempted to update version but failed")
		}

		return nil
	}

	if !foundLocal {
		if !hasCreds {
			return fmt.Errorf("No credentials for registry: %s", img.Registry)
		}

		return pullImage(img, creds)
	}

	return nil
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

func (c *container) ID() string {
	return c.id
}

func (c *container) Close() error {
	if c.network != nil {
		for _, cont := range append(c.linked, c) {
			if err := DefaultClient.DisconnectNetwork(c.network.ID, docker.NetworkConnectionOptions{
				Container: cont.ID(),
			}); err != nil {
				log.Warn().Msgf("Failed to disconnect container %s from network %s", c.id, c.network.ID)
				continue
			}
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

func (c *container) Start() error {
	if err := DefaultClient.StartContainer(c.id, nil); err != nil {
		return err
	}

	log.Debug().
		Str("ID", c.id[0:8]).
		Str("Image", c.conf.Image).
		Msg("Started container")
	return nil
}

func (c *container) Stop() error {
	if err := DefaultClient.StopContainer(c.id, 5); err != nil {
		return err
	}

	log.Debug().
		Str("ID", c.id[0:8]).
		Str("Image", c.conf.Image).
		Msg("Stopped container")

	return nil
}

func (c *container) Link(other Identifier, alias string) error {
	if c.network == nil {
		createNetworkOpts := docker.CreateNetworkOptions{
			Name:   uuid.New().String(),
			Driver: "bridge",
		}

		net, err := DefaultClient.CreateNetwork(createNetworkOpts)
		if err != nil {
			return err
		}

		c.network = net
	}

	err := DefaultClient.ConnectNetwork(c.network.ID, docker.NetworkConnectionOptions{
		Container: other.ID(),
		EndpointConfig: &docker.EndpointConfig{
			Aliases: []string{alias},
		},
	})
	if err != nil {
		return err
	}
	c.linked = append(c.linked, other)

	err = DefaultClient.ConnectNetwork(c.network.ID, docker.NetworkConnectionOptions{
		Container: c.ID(),
	})

	if err != nil {
		return err
	}

	return nil
}

type Network struct {
	net       *docker.Network
	subnet    string
	ipPool    map[uint8]struct{}
	connected []Identifier
}

func NewNetwork() (*Network, error) {
	conf := func() docker.CreateNetworkOptions {
		sub := randomPrivateSubnet24()

		subnet := fmt.Sprintf("%s.0/24", sub)
		return docker.CreateNetworkOptions{
			Name:   uuid.New().String(),
			Driver: "macvlan",
			IPAM: &docker.IPAMOptions{
				Config: []docker.IPAMConfig{{
					Subnet: subnet,
				}},
			},
		}
	}

	var config docker.CreateNetworkOptions
	var net *docker.Network
	var err error
	for i := 0; i < 10; i++ {
		config = conf()
		net, err = DefaultClient.CreateNetwork(config)
		if err != nil {
			if strings.Contains(err.Error(), "Pool overlaps") {
				continue
			}
		}

		break
	}

	if err != nil {
		return nil, err
	}

	net, _ = DefaultClient.NetworkInfo(net.ID)
	subnet := config.IPAM.Config[0].Subnet

	ipPool := make(map[uint8]struct{})
	for i := 30; i < 255; i++ {
		ipPool[uint8(i)] = struct{}{}
	}

	return &Network{net: net, subnet: subnet, ipPool: ipPool}, nil
}

func (n *Network) Close() error {
	for _, cont := range n.connected {
		if err := DefaultClient.DisconnectNetwork(n.net.ID, docker.NetworkConnectionOptions{
			Container: cont.ID(),
		}); err != nil {
			continue
		}
	}

	return DefaultClient.RemoveNetwork(n.net.ID)
}

func (n *Network) FormatIP(num int) string {
	return fmt.Sprintf("%s.%d", n.subnet[0:len(n.subnet)-5], num)
}

func (n *Network) Interface() string {
	return fmt.Sprintf("dm-%s", n.net.ID[0:12])
}

func (n *Network) getRandomIP() int {
	for randDigit, _ := range n.ipPool {
		delete(n.ipPool, randDigit)
		return int(randDigit)
	}
	return 0
}

func (n *Network) releaseIP(ip string) {
	parts := strings.Split(ip, ".")
	strDigit := parts[len(parts)-1]

	num, err := strconv.Atoi(strDigit)
	if err != nil {
		return
	}

	n.ipPool[uint8(num)] = struct{}{}
}

func (n *Network) Connect(c Container, ip ...int) (int, error) {
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

func GetDockerHostIP() (string, error) {
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

var (
	privateSubnets = map[uint8]struct {
		minLvl2 int
		maxLvl2 int
		minLvl3 int
		maxLvl3 int
	}{
		10: {
			minLvl2: 0,
			maxLvl2: 255,
			minLvl3: 0,
			maxLvl3: 255,
		},
		172: {
			minLvl2: 25,
			maxLvl2: 31,
			minLvl3: 0,
			maxLvl3: 255,
		},
		192: {
			minLvl2: 168,
			maxLvl2: 168,
			minLvl3: 0,
			maxLvl3: 255,
		},
	}
)

func randomPrivateSubnet24() string {
	v := []uint8{10, 172, 192}
	lvl1 := v[rand.Intn(len(v))]
	opts := privateSubnets[lvl1]

	lvl2range := opts.maxLvl2 - opts.minLvl2
	lvl2 := opts.maxLvl2

	if lvl2range > 0 {
		lvl2 = rand.Intn(lvl2range) + opts.minLvl2
	}

	lvl3range := opts.maxLvl3 - opts.minLvl3
	lvl3 := opts.maxLvl3

	if lvl3range > 0 {
		lvl3 = rand.Intn(lvl3range) + opts.minLvl3
	}

	return fmt.Sprintf("%d.%d.%d", lvl1, lvl2, lvl3)
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
