package guacamole

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/aau-network-security/go-ntp/svcs/revproxy"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/google/uuid"
)

var (
	MalformedLoginErr = errors.New("Malformed login response")
	NoHostErr         = errors.New("Host is missing")
	NoPortErr         = errors.New("Port is missing")
	NoNameErr         = errors.New("Name is missing")
	IncorrectColorErr = errors.New("ColorDepth can take the following values: 8, 16, 24, 32")
)

type Guacamole interface {
	docker.Identifier
	revproxy.Connector
	Start(context.Context) error
	CreateUser(username, password string) error
	CreateRDPConn(opts CreateRDPConnOpts) error
	Close()
}

type Config struct {
	AdminUser string
	AdminPass string
	Host      string
	Port      uint
}

func New(conf Config) (Guacamole, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Jar: jar,
	}

	if conf.Host == "" {
		conf.Host = "localhost"
	}

	if conf.Port == 0 {
		conf.Port = 8080
	}

	guac := &guacamole{
		client: client,
		conf:   conf,
	}

	return guac, nil
}

type guacamole struct {
	conf       Config
	token      string
	client     *http.Client
	web        docker.Container
	containers []docker.Container
}

func (guac *guacamole) ID() string {
	return guac.web.ID()
}

func (guac *guacamole) Close() {
	for _, c := range guac.containers {
		c.Kill()
	}
}

func (guac *guacamole) Start(ctx context.Context) error {
	// Guacd
	guacd, err := docker.NewContainer(docker.ContainerConfig{
		Image: "guacamole/guacd",
	})
	if err != nil {
		return err
	}
	guac.containers = append(guac.containers, guacd)

	err = guacd.Start()
	if err != nil {
		return err
	}

	// Database
	dbEnv := map[string]string{
		"MYSQL_ROOT_PASSWORD": uuid.New().String(),
		"MYSQL_DATABASE":      "guacamole_db",
		"MYSQL_USER":          "guacamole_user",
		"MYSQL_PASSWORD":      uuid.New().String(),
	}

	db, err := docker.NewContainer(docker.ContainerConfig{
		Image:   "aau/guacamole-mysql",
		EnvVars: dbEnv,
	})
	if err != nil {
		return err
	}
	guac.containers = append(guac.containers, db)

	err = db.Start()
	if err != nil {
		return err
	}

	// Web Init
	webEnv := map[string]string{
		"MYSQL_DATABASE": "guacamole_db",
		"MYSQL_USER":     "guacamole_user",
		"MYSQL_PASSWORD": dbEnv["MYSQL_PASSWORD"],
		"GUACD_HOSTNAME": "guacd",
		"MYSQL_HOSTNAME": "db",
	}

	webBaseConf := &docker.ContainerConfig{
		Image:   "aau/guacamole",
		EnvVars: webEnv,
	}

	webInitConf := *webBaseConf
	webInitPort := docker.GetAvailablePort()
	webInitConf.PortBindings = map[string]string{
		"8080/tcp": fmt.Sprintf("127.0.0.1:%d", webInitPort),
	}

	initWeb, err := docker.NewContainer(webInitConf)
	if err != nil {
		return err
	}

	if err = initWeb.Link(db, "db"); err != nil {
		return err
	}

	if err = initWeb.Link(guacd, "guacd"); err != nil {
		return err
	}

	err = initWeb.Start()
	if err != nil {
		return err
	}

	err = guac.configureInstance(webInitPort)
	if err != nil {
		return err
	}

	err = initWeb.Kill()
	if err != nil {
		return err
	}

	webFinalConf := *webBaseConf
	finalWeb, err := docker.NewContainer(webFinalConf)
	if err != nil {
		return err
	}
	guac.containers = append(guac.containers, finalWeb)
	guac.web = finalWeb

	if err = finalWeb.Link(db, "db"); err != nil {
		return err
	}

	if err = finalWeb.Link(guacd, "guacd"); err != nil {
		return err
	}

	err = finalWeb.Start()
	if err != nil {
		return err
	}

	return nil
}

func (guac *guacamole) ConnectProxy(p revproxy.Proxy) error {
	conf := `location /guacamole/ {
        proxy_pass http://{{.Host}}:8080/guacamole/;
        proxy_buffering off;
        proxy_http_version 1.1;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$http_connection;
        # proxy_cookie_path /guacamole/ /;
        access_log off;
    }`

	return p.Add(guac, conf)
}

func (guac *guacamole) configureInstance(port uint) error {
	temp := &guacamole{
		client: guac.client,
		conf: Config{
			AdminUser: "guacadmin",
			AdminPass: "guacadmin",
			Host:      "127.0.0.1",
			Port:      port,
		}}

	for i := 0; i < 15; i++ {
		_, err := temp.login("guacadmin", "guacadmin")
		if err == nil {
			break
		}

		time.Sleep(time.Second)
	}
	newPass := guac.conf.AdminPass
	guac.conf.AdminPass = "guacadmin"

	if err := temp.changeAdminPass(newPass); err != nil {
		return err
	}

	return nil
}

func (guac *guacamole) baseUrl() string {
	return fmt.Sprintf("http://%s:%d", guac.conf.Host, guac.conf.Port)
}

func (guac *guacamole) login(username, password string) (string, error) {
	form := url.Values{
		"username": {username},
		"password": {password},
	}

	endpoint := guac.baseUrl() + "/guacamole/api/tokens"
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := guac.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var output struct {
		Message   *string `json:"message"`
		AuthToken *string `json:"authToken"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return "", err
	}

	if output.Message != nil {
		return "", fmt.Errorf(*output.Message)
	}

	if output.AuthToken == nil {
		return "", MalformedLoginErr
	}

	return *output.AuthToken, nil
}

func (guac *guacamole) authAction(a func(string) (*http.Response, error), i interface{}) error {
	if guac.token == "" {
		token, err := guac.login(guac.conf.AdminUser, guac.conf.AdminPass)
		if err != nil {
			return err
		}

		guac.token = token
	}

	resp, err := a(guac.token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var msg struct {
		Message string `json:"message"`
	}

	if err := json.Unmarshal(content, msg); err == nil {
		return fmt.Errorf(msg.Message)
	}

	if i != nil {
		if err := json.Unmarshal(content, i); err != nil {
			return err
		}
	}

	return nil
}

func (guac *guacamole) changeAdminPass(newPass string) error {
	action := func(t string) (*http.Response, error) {
		data := map[string]string{
			"newPassword": newPass,
			"oldPassword": guac.conf.AdminPass,
		}

		jsonData, _ := json.Marshal(data)
		endpoint := guac.baseUrl() + "/guacamole/api/session/data/mysql/users/guacadmin/password?token=" + t
		req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		return guac.client.Do(req)
	}

	if err := guac.authAction(action, nil); err != nil {
		return err
	}

	return nil
}

type createUserAttributes struct {
	Disabled          string  `json:"disabled"`
	Expired           string  `json:"expired"`
	AccessWindowStart string  `json:"access-window-start"`
	AccessWindowEnd   string  `json:"access-window-end"`
	ValidFrom         string  `json:"valid-from"`
	ValidUntil        string  `json:"valid-until"`
	TimeZone          *string `json:"timezone"`
}

type createUserInput struct {
	Username   string               `json:"username"`
	Password   string               `json:"password"`
	Attributes createUserAttributes `json:"attributes"`
}

func (guac *guacamole) CreateUser(username, password string) error {
	action := func(t string) (*http.Response, error) {
		data := createUserInput{
			Username: username,
			Password: password,
		}
		jsonData, _ := json.Marshal(data)
		endpoint := guac.baseUrl() + "/guacamole/api/session/data/mysql/users?token=" + t

		req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		return guac.client.Do(req)
	}

	if err := guac.authAction(action, nil); err != nil {
		return err
	}

	return nil
}

type createRDPConnAttr struct {
	FailOverOnly     *bool   `json:"failover-only"`
	GuacdEncripytion *string `json:"guacd-encryption"`
	GuacdPort        *uint   `json:"guacd-port"`
	MaxConn          uint    `json:"max-connections"`
	MaxConnPerUser   uint    `json:"max-connections-per-user"`
	Weight           *uint   `json:"weight"`
}

type createRDPConnConf struct {
	ClipboardEncoding        *string `json:"clipboard-encoding"`
	ColorDepth               *uint   `json:"color-depth"`
	Console                  *string `json:"console"`
	ConsoleAudio             *string `json:"console-audio"`
	Cursor                   *string `json:"cursor"`
	DestPort                 *uint   `json:"dest-port"`
	DisableAudio             *bool   `json:"disable-audio"`
	DisableAuth              *bool   `json:"disable-auth"`
	DPI                      *uint   `json:"dpi"`
	EnableAudio              *bool   `json:"enable-audio"`
	EnableAudioInput         *bool   `json:"enable-audio-input"`
	EnableDesktopComposition *bool   `json:"enable-desktop-composition"`
	EnableDrive              *bool   `json:"enable-drive"`
	EnableFontSmoothing      *bool   `json:"enable-font-smoothing"`
	EnableFullWindowDrag     *bool   `json:"enable-full-window-drag"`
	EnableMenuAnimations     *bool   `json:"enable-menu-animations"`
	EnablePrinting           *bool   `json:"enable-printing"`
	EnableSFTP               *bool   `json:"enable-sftp"`
	EnableTheming            *bool   `json:"enable-theming"`
	EnableWallpaper          *bool   `json:"enable-wallpaper"`
	GatewayPort              *uint   `json:"gateway-port"`
	Height                   *uint   `json:"height"`
	Width                    *uint   `json:"width"`
	Hostname                 *string `json:"hostname"`
	IgnoreCert               *bool   `json:"ignore-cert"`
	Port                     *uint   `json:"port"`
	PreConnectionID          *uint   `json:"preconnection-id"`
	ReadOnly                 *bool   `json:"read-only"`
	ResizeMethod             *string `json:"resize-method"`
	Security                 *string `json:"security"`
	ServerLayout             *string `json:"server-layout"`
	SFTPPort                 *uint   `json:"sftp-port"`
	SFTPAliveInterval        *uint   `json:"sftp-server-alive-interval"`
	SwapRedBlue              *bool   `json:"swap-red-blue"`
	CreateDrivePath          *bool   `json:"create-drive-path"`
	Username                 *string `json:"username,omitempty"`
	Password                 *string `json:"password,omitempty"`
}

type CreateRDPConnOpts struct {
	Host             string
	Port             uint
	Name             string
	GuacUser         string
	Username         *string
	Password         *string
	ResolutionWidth  uint
	ResolutionHeight uint
	MaxConn          uint
	ColorDepth       uint
}

func (guac *guacamole) CreateRDPConn(opts CreateRDPConnOpts) error {
	if opts.Host == "" {
		return NoHostErr
	}

	if opts.Port == 0 {
		return NoPortErr
	}

	if opts.Name == "" {
		return NoNameErr
	}

	if opts.ResolutionWidth == 0 || opts.ResolutionHeight == 0 {
		opts.ResolutionWidth = 1920
		opts.ResolutionHeight = 1080
	}

	if opts.MaxConn == 0 {
		opts.MaxConn = 10
	}

	if opts.ColorDepth%8 != 0 || opts.ColorDepth > 32 {
		return IncorrectColorErr
	}

	if opts.ColorDepth == 0 {
		opts.ColorDepth = 16
	}

	conf := createRDPConnConf{
		Hostname:   &opts.Host,
		Width:      &opts.ResolutionWidth,
		Height:     &opts.ResolutionHeight,
		Port:       &opts.Port,
		ColorDepth: &opts.ColorDepth,
		Username:   opts.Username,
		Password:   opts.Password,
	}

	data := struct {
		Name             string            `json:"name"`
		ParentIdentifier string            `json:"parentIdentifier"`
		Protocol         string            `json:"protocol"`
		Attributes       createRDPConnAttr `json:"attributes"`
		Parameters       createRDPConnConf `json:"parameters"`
	}{
		Name:             opts.Name,
		ParentIdentifier: "ROOT",
		Protocol:         "rdp",
		Attributes: createRDPConnAttr{
			MaxConn:        opts.MaxConn,
			MaxConnPerUser: opts.MaxConn,
		},
		Parameters: conf,
	}

	jsonData, _ := json.Marshal(data)

	action := func(t string) (*http.Response, error) {
		endpoint := guac.baseUrl() + "/guacamole/api/session/data/mysql/connections?token=" + t

		req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		return guac.client.Do(req)
	}

	var out struct {
		Id string `json:"identifier"`
	}
	if err := guac.authAction(action, &out); err != nil {
		return err
	}

	if err := guac.addConnectionToUser(out.Id, opts.GuacUser); err != nil {
		return err
	}

	return nil
}

func (guac *guacamole) addConnectionToUser(id string, guacuser string) error {
	data := []struct {
		Operation string `json:"op"`
		Path      string `json:"path"`
		Value     string `json:"value"`
	}{{
		Operation: "add",
		Path:      fmt.Sprintf("/connectionPermissions/%s", id),
		Value:     "READ",
	}}

	jsonData, _ := json.Marshal(data)

	action := func(t string) (*http.Response, error) {
		endpoint := fmt.Sprintf("%s/guacamole/api/session/data/mysql/users/%s/permissions?token=%s",
			guac.baseUrl(),
			guacuser,
			t)

		req, err := http.NewRequest("PATCH", endpoint, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		return guac.client.Do(req)
	}

	if err := guac.authAction(action, nil); err != nil {
		return err
	}

	return nil
}
