// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package guacamole

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aau-network-security/haaukins/virtual/vbox"

	"github.com/aau-network-security/haaukins/store"

	"github.com/aau-network-security/haaukins/svcs"
	"github.com/aau-network-security/haaukins/svcs/amigo"
	"github.com/aau-network-security/haaukins/virtual"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var (
	MalformedLoginErr = errors.New("malformed login response")
	NoHostErr         = errors.New("host is missing")
	NoPortErr         = errors.New("port is missing")
	NoNameErr         = errors.New("name is missing")
	IncorrectColorErr = errors.New("colorDepth can take the following values: 8, 16, 24, 32")
	UnexpectedRespErr = errors.New("unexpected response from Guacamole")
	SessionErr        = errors.New("session must exist")

	DefaultAdminUser = "guacadmin"
	DefaultAdminPass = "guacadmin"

	wsHeaders = []string{
		"Sec-Websocket-Extensions",
		"Sec-Websocket-Version",
		"Sec-Websocket-Key",
		"Connection",
		"Upgrade",
	}

	upgrader = websocket.Upgrader{}
)

type GuacError struct {
	action string
	err    error
}

type Config struct {
	AdminPass string `yaml:"admin_pass"`
}

type guacamole struct {
	conf       Config
	token      string
	client     *http.Client
	webPort    uint
	containers map[string]docker.Container
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

func (ge *GuacError) Error() string {
	return fmt.Sprintf("guacamole: trying to %s. failed: %s", ge.action, ge.err)
}

type Guacamole interface {
	io.Closer
	Start(context.Context) error
	CreateUser(username, password string) error
	CreateRDPConn(opts CreateRDPConnOpts) error
	GetAdminPass() string
	GetPort() uint
	RawLogin(username, password string) ([]byte, error)
	ProxyHandler(us *GuacUserStore, klp KeyLoggerPool, am *amigo.Amigo, event Event) svcs.ProxyConnector
}

func New(ctx context.Context, conf Config, onlyVPN int32, eventTag string) (Guacamole, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Jar: jar,
	}

	if conf.AdminPass == "" {
		pass := uuid.New().String()
		log.Info().
			Str("password", pass).
			Msg("setting new default password for guacamole")

		conf.AdminPass = pass
	}

	guac := &guacamole{
		client: client,
		conf:   conf,
	}
	if onlyVPN != docker.OnlyVPN {
		if err := guac.create(ctx, eventTag); err != nil {
			return nil, err
		}
	}
	return guac, nil
}

func (guac *guacamole) Close() error {
	for _, c := range guac.containers {
		c.Close()
	}
	return nil
}

func (guac *guacamole) GetAdminPass() string {
	return guac.conf.AdminPass
}

func (guac *guacamole) GetPort() uint {
	return guac.webPort
}

func (guac *guacamole) create(ctx context.Context, eventTag string) error {
	_ = vbox.CreateEventFolder(eventTag)

	user := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	log.Debug().Str("user", user).Msg("starting guacd")
  
	containers := map[string]docker.Container{}
	containers["guacd"] = docker.NewContainer(docker.ContainerConfig{
		Image:     "guacamole/guacd:1.2.0",
		UseBridge: true,
		Labels: map[string]string{
			"hkn": "guacamole_guacd",
		},
		Mounts: []string{
			vbox.FileTransferRoot + "/" + eventTag + "/:/home/",
		},
		User: user,
	})

	mysqlPass := uuid.New().String()
	containers["db"] = docker.NewContainer(docker.ContainerConfig{
		Image: "aaunetworksecurity/guacamole-mysql",
		EnvVars: map[string]string{
			"MYSQL_ROOT_PASSWORD": uuid.New().String(),
			"MYSQL_DATABASE":      "guacamole_db",
			"MYSQL_USER":          "guacamole_user",
			"MYSQL_PASSWORD":      mysqlPass,
		},
		Labels: map[string]string{
			"hkn": "guacamole_db",
		},
	})

	guac.webPort = virtual.GetAvailablePort()
	guacdAlias := uuid.New().String()
	dbAlias := uuid.New().String()
	containers["web"] = docker.NewContainer(docker.ContainerConfig{
		Image: "registry.gitlab.com/haaukins/core-utils/guacamole",
		EnvVars: map[string]string{
			"MYSQL_DATABASE": "guacamole_db",
			"MYSQL_USER":     "guacamole_user",
			"MYSQL_PASSWORD": mysqlPass,
			"GUACD_HOSTNAME": guacdAlias,
			"MYSQL_HOSTNAME": dbAlias,
		},
		PortBindings: map[string]string{
			"8080/tcp": fmt.Sprintf("127.0.0.1:%d", guac.webPort),
		},
		UseBridge: true,
		Labels: map[string]string{
			"hkn": "guacamole_web",
		},
	})

	closeAll := func() {
		for _, c := range containers {
			c.Close()
		}
	}

	for _, cname := range []string{"guacd", "db", "web"} {
		c := containers[cname]

		if err := c.Run(ctx); err != nil {
			closeAll()
			return err
		}

		var alias string
		switch cname {
		case "guacd":
			alias = guacdAlias
		case "db":
			alias = dbAlias
		}

		if _, err := c.BridgeAlias(alias); err != nil {
			closeAll()
			return err
		}
	}

	if err := guac.configureInstance(); err != nil {
		closeAll()
		return err
	}

	guac.containers = containers
	guac.stop()

	return nil
}

func (guac *guacamole) Start(ctx context.Context) error {
	for _, container := range guac.containers {
		if err := container.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (guac *guacamole) stop() error {
	for _, container := range guac.containers {
		if err := container.Stop(); err != nil {
			return err
		}
	}
	return nil
}

func (guac *guacamole) ProxyHandler(us *GuacUserStore, klp KeyLoggerPool, am *amigo.Amigo, ev Event) svcs.ProxyConnector {
	loginFunc := func(u string, p string) (string, error) {
		content, err := guac.RawLogin(u, p)
		if err != nil {
			return "", err
		}

		return url.QueryEscape(string(content)), nil
	}

	return func(ef store.Event) http.Handler {
		origin, _ := url.Parse(guac.baseUrl() + "/guacamole")
		host := fmt.Sprintf("127.0.0.1:%d", guac.webPort)
		interceptors := svcs.Interceptors{
			NewGuacTokenLoginEndpoint(us, ef, am, loginFunc),
		}

		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.Header.Add("X-Forwarded-Host", req.Host)
				req.URL.Scheme = "http"
				req.URL.Host = origin.Host

			}}

		return interceptors.Intercept(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if isWebSocket(r) {
					websocketProxy(host, ef, klp, am).ServeHTTP(w, r)
					return
				}
				proxy.ServeHTTP(w, r)
			}))
	}
}

func (guac *guacamole) configureInstance() error {
	temp := &guacamole{
		client:  guac.client,
		conf:    Config{AdminPass: DefaultAdminPass},
		webPort: guac.webPort,
	}

	var err error
	for i := 0; i < 120; i++ {
		_, err = temp.login(DefaultAdminUser, DefaultAdminPass)
		if err == nil {
			break
		}

		time.Sleep(time.Second)
	}
	if err != nil {
		return err
	}

	if err := temp.changeAdminPass(guac.conf.AdminPass); err != nil {
		return err
	}

	return nil
}

func (guac *guacamole) baseUrl() string {
	return fmt.Sprintf("http://127.0.0.1:%d", guac.webPort)
}

func (guac *guacamole) login(username, password string) (string, error) {
	content, err := guac.RawLogin(username, password)
	if err != nil {
		return "", err
	}

	var output struct {
		Message   *string `json:"message"`
		AuthToken *string `json:"authToken"`
	}

	if err := json.Unmarshal(content, &output); err != nil {
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

func (guac *guacamole) RawLogin(username, password string) ([]byte, error) {
	form := url.Values{
		"username": {username},
		"password": {password},
	}

	endpoint := guac.baseUrl() + "/guacamole/api/tokens"
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := guac.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := isExpectedStatus(resp.StatusCode); err != nil {
		return nil, &GuacError{action: "login", err: err}
	}

	return ioutil.ReadAll(resp.Body)
}

func (guac *guacamole) authAction(action string, a func(string) (*http.Response, error), i interface{}) error {
	perform := func() ([]byte, int, error) {
		resp, err := a(guac.token)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()

		content, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, 0, err
		}

		return content, resp.StatusCode, nil
	}

	shouldTryAgain := func(content []byte, status int, connErr error) (bool, error) {
		if connErr != nil {
			return true, connErr
		}

		if err := isExpectedStatus(status); err != nil {
			return true, err
		}

		if status == http.StatusForbidden {
			token, err := guac.login(DefaultAdminUser, guac.conf.AdminPass)
			if err != nil {
				return false, err
			}

			guac.token = token

			return true, nil
		}

		var msg struct {
			Message string `json:"message"`
		}

		if err := json.Unmarshal(content, &msg); err == nil {
			switch {
			case msg.Message == "Permission Denied.":
				token, err := guac.login(DefaultAdminUser, guac.conf.AdminPass)
				if err != nil {
					return false, err
				}

				guac.token = token

				return true, nil
			case msg.Message != "":
				return false, &GuacError{action: action, err: fmt.Errorf("unexpected message: %s", msg.Message)}
			}
		}

		return false, nil
	}

	var retry bool
	content, status, err := perform()
	for i := 1; i <= 3; i++ {
		retry, err = shouldTryAgain(content, status, err)
		if !retry {
			break
		}

		time.Sleep(time.Second)

		content, status, err = perform()
	}

	if err != nil {
		return err
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

	if err := guac.authAction("change admin password", action, nil); err != nil {
		return err
	}

	return nil
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

	var output struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := guac.authAction("create user", action, &output); err != nil {
		return err
	}

	return nil
}

func (guac *guacamole) logout() error {
	action := func(t string) (*http.Response, error) {
		endpoint := guac.baseUrl() + "/guacamole/api/tokens/" + t
		req, err := http.NewRequest("DELETE", endpoint, nil)
		if err != nil {
			return nil, err
		}

		return guac.client.Do(req)
	}

	if err := guac.authAction("logout", action, nil); err != nil {
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
	DrivePath                *string `json:"drive-path"`
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
	EnableWallPaper  *bool
	ResolutionWidth  uint
	ResolutionHeight uint
	MaxConn          uint
	ColorDepth       uint
	EnableDrive      *bool
	CreateDrivePath  *bool
	DrivePath        *string
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
	log.Debug().Str("drive-path", *opts.DrivePath).Msg("Drivepath for user is")
	conf := createRDPConnConf{
		Hostname:        &opts.Host,
		Width:           &opts.ResolutionWidth,
		Height:          &opts.ResolutionHeight,
		Port:            &opts.Port,
		ColorDepth:      &opts.ColorDepth,
		Username:        opts.Username,
		Password:        opts.Password,
		EnableWallpaper: opts.EnableWallPaper,
		EnableDrive:     opts.EnableDrive,
		CreateDrivePath: opts.CreateDrivePath,
		DrivePath:       opts.DrivePath,
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
	if err := guac.authAction("create rdp connection", action, &out); err != nil {
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

	if err := guac.authAction("add user to connection", action, nil); err != nil {
		return err
	}

	return nil
}

func websocketProxy(target string, ef store.Event, keyLoggerPool KeyLoggerPool, am *amigo.Amigo) http.Handler {
	origin := fmt.Sprintf("http://%s", target)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := r.URL
		url.Host = target
		url.Scheme = "ws"

		cookie, err := r.Cookie("session")
		if err != nil {
			log.Error().Err(SessionErr)
			return
		}

		t, err := am.TeamStore.GetTeamByToken(cookie.Value)
		if err != nil {
			log.Error().Msgf("Failed to find team by token")
			return
		}

		var logger KeyLogger
		logger, err = keyLoggerPool.GetLogger(*t)
		if err != nil {
			log.Warn().Msg("Failed to create keylogger")
		}

		rHeader := http.Header{}
		copyHeaders(r.Header, rHeader, wsHeaders)
		rHeader.Set("Origin", origin)
		rHeader.Set("X-Forwarded-Host", r.Host)

		backend, resp, err := websocket.DefaultDialer.Dial(url.String(), rHeader)
		if err != nil {
			log.Error().Msgf("Failed to connect target (%s): %s", url.String(), err)
			return
		}
		defer backend.Close()

		upgradeHeader := http.Header{}
		if h := resp.Header.Get("Sec-Websocket-Protocol"); h != "" {
			upgradeHeader.Set("Sec-Websocket-Protocol", h)
		}

		c, err := upgrader.Upgrade(w, r, upgradeHeader)
		if err != nil {
			log.Error().Msgf("Failed to upgrade connection: %s", err)
			return
		}
		defer c.Close()

		errClient := make(chan error, 1)
		errBackend := make(chan error, 1)

		cp := func(logger KeyLogger) func(src *websocket.Conn, dst *websocket.Conn, errc chan error) {
			var actions []func(RawFrame)
			if logger != nil {
				actions = append(actions, logger.Log)
			}

			return func(src *websocket.Conn, dst *websocket.Conn, errc chan error) {
				for {
					msgType, data, err := src.ReadMessage()
					if err != nil {
						m := getCloseMsg(err)
						dst.WriteMessage(websocket.CloseMessage, m)
						errc <- err
					}

					if err := dst.WriteMessage(msgType, data); err != nil {
						errc <- err
						break
					}

					for _, action := range actions {
						action(data)
					}
				}
			}
		}

		go cp(logger)(c, backend, errClient)
		go cp(nil)(backend, c, errClient)

		log.Debug().
			Str("id", t.ID()).
			Str("event", string(ef.Tag)).
			Msg("team connected")

		var msgFormat string
		select {
		case err = <-errClient:
			msgFormat = "Error when copying from client to backend: %s"
		case err = <-errBackend:
			msgFormat = "Error when copying from backend to client: %s"
		}

		e, ok := err.(*websocket.CloseError)
		if ok && e.Code == websocket.CloseNoStatusReceived {
			log.Debug().
				Str("id", t.ID()).
				Str("event", string(ef.Tag)).
				Msg("team disconnected")
		} else if !ok || e.Code != websocket.CloseNormalClosure {
			log.Error().Msgf(msgFormat, err)
		}
	})
}

func copyHeaders(src, dst http.Header, ignore []string) {
	for k, vv := range src {
		isIgnored := false
		for _, h := range ignore {
			if k == h {
				isIgnored = true
				break
			}
		}
		if isIgnored {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func getCloseMsg(err error) []byte {
	res := websocket.FormatCloseMessage(websocket.CloseNormalClosure, fmt.Sprintf("%s", err))
	if e, ok := err.(*websocket.CloseError); ok {
		if e.Code != websocket.CloseNoStatusReceived {
			res = websocket.FormatCloseMessage(e.Code, e.Text)
		}
	}
	return res
}

func isWebSocket(req *http.Request) bool {
	if upgrade := req.Header.Get("Upgrade"); upgrade != "" {
		return upgrade == "websocket" || upgrade == "Websocket"
	}

	return false
}

func isExpectedStatus(s int) error {
	if (s >= http.StatusOK) && (s <= 302) || s == http.StatusForbidden {
		return nil
	}

	return fmt.Errorf("unexpected response %d", s)
}
