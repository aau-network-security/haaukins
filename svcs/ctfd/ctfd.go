package ctfd

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"errors"

	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/svcs"
	"github.com/aau-network-security/go-ntp/virtual"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var (
	NONCEREGEXP            = regexp.MustCompile(`csrf_nonce[ ]*=[ ]*"(.+)"`)
	ServerUnavailableErr   = errors.New("Server is unavailable")
	UserNotFoundErr        = errors.New("Could not find the specified user")
	CouldNotFindSessionErr = errors.New("Could not find the specified user")
)

type CTFd interface {
	docker.Identifier
	svcs.ProxyConnector
	Middleware(http.Handler) http.Handler
	Start() error
	Close() error
	Stop() error
	Flags() []exercise.FlagConfig
}

type Config struct {
	Name         string `yaml:"name"`
	AdminUser    string `yaml:"admin_user"`
	AdminEmail   string `yaml:"admin_email"`
	AdminPass    string `yaml:"admin_pass"`
	CallbackHost string
	CallbackPort uint
	Flags        []exercise.FlagConfig
}

type ctfd struct {
	conf       Config
	cont       docker.Container
	confDir    string
	port       uint
	httpclient *http.Client
	users      []*user
	relation   map[string]*user
}

type user struct {
	teamname string
	email    string
	password string
}

func New(conf Config) (CTFd, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	hc := &http.Client{
		Jar: jar,
	}

	if conf.Name == "" {
		conf.Name = "Demo"
	}

	if conf.AdminUser == "" {
		conf.AdminUser = "admin"
	}

	if conf.AdminEmail == "" {
		conf.AdminEmail = "admin@admin.com"
	}

	if conf.AdminPass == "" {
		pass := uuid.New().String()
		log.Info().
			Str("password", pass).
			Msg("setting new default password for ctfd")

		conf.AdminPass = pass
	}

	ctf := &ctfd{
		conf:       conf,
		httpclient: hc,
		port:       virtual.GetAvailablePort(),
		relation:   make(map[string]*user),
	}

	confDir, err := ioutil.TempDir("", "ctfd")
	if err != nil {
		return nil, err
	}

	ctf.confDir = confDir

	dconf := docker.ContainerConfig{
		Image: "registry.sec-aau.dk/aau/ctfd",
		Mounts: []string{
			fmt.Sprintf("%s/:/opt/CTFd/CTFd/data", confDir),
		},
		EnvVars: map[string]string{
			"ADMIN_HOST": conf.CallbackHost,
			"ADMIN_PORT": fmt.Sprintf("%d", conf.CallbackPort),
		},
		PortBindings: map[string]string{
			"8000/tcp": fmt.Sprintf("127.0.0.1:%d", ctf.port),
		},
		UseBridge: true,
	}

	c, err := docker.NewContainer(dconf)
	if err != nil {
		return nil, err
	}

	err = c.Start()
	if err != nil {
		return nil, err
	}

	err = ctf.configureInstance()
	if err != nil {
		return nil, err
	}

	err = c.Stop()
	if err != nil {
		return nil, err
	}

	ctf.cont = c

	return ctf, nil

}

func (ctf *ctfd) Start() error {
	return ctf.cont.Start()
}

func (ctf *ctfd) Close() error {
	if err := os.RemoveAll(ctf.confDir); err != nil {
		return err
	}

	if err := ctf.cont.Close(); err != nil {
		return err
	}

	return nil
}

func (ctf *ctfd) Stop() error {
	return ctf.cont.Stop()
}

func (ctf *ctfd) Flags() []exercise.FlagConfig {
	return ctf.conf.Flags
}

func (ctf *ctfd) ID() string {
	return ctf.cont.ID()
}

func (ctf *ctfd) ProxyHandler() http.Handler {
	origin, _ := url.Parse(ctf.baseUrl())
	return httputil.NewSingleHostReverseProxy(origin)
}

func (ctf *ctfd) baseUrl() string {
	return fmt.Sprintf("http://127.0.0.1:%d", ctf.port)
}

func (ctf *ctfd) getNonce(path string) (string, error) {
	resp, err := ctf.httpclient.Get(path)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	matches := NONCEREGEXP.FindAllSubmatch(content, 1)
	if len(matches) == 0 {
		return "", fmt.Errorf("Unable to find nonce in page")
	}

	return string(matches[0][1]), nil
}

func (ctf *ctfd) createFlag(name, flag string, points uint) error {
	endpoint := ctf.baseUrl() + "/admin/chal/new"

	nonce, err := ctf.getNonce(endpoint)
	if err != nil {
		return err
	}

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	values := map[string]string{
		"name":         name,
		"value":        fmt.Sprintf("%d", points),
		"key":          flag,
		"nonce":        nonce,
		"key_type[0]":  "static",
		"category":     "",
		"description":  "",
		"max_attempts": "",
		"chaltype":     "standard",
	}

	for k, v := range values {
		err := w.WriteField(k, v)
		if err != nil {
			return err
		}
	}
	w.Close()

	req, err := http.NewRequest("POST", endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", w.FormDataContentType())

	resp, err := ctf.httpclient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}

func (ctf *ctfd) configureInstance() error {
	endpoint := ctf.baseUrl() + "/setup"

	if err := waitForServer(endpoint); err != nil {
		return err
	}

	nonce, err := ctf.getNonce(endpoint)
	if err != nil {
		return err
	}

	form := url.Values{
		"ctf_name": {ctf.conf.Name},
		"name":     {ctf.conf.AdminUser},
		"password": {ctf.conf.AdminPass},
		"email":    {ctf.conf.AdminEmail},
		"nonce":    {nonce},
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ctf.httpclient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for _, flag := range ctf.conf.Flags {
		err := ctf.createFlag(flag.Name, flag.Default, flag.Points)
		if err != nil {
			return err
		}
		log.Debug().
			Str("name", flag.Name).
			Str("flag", flag.Default).
			Uint("points", flag.Points).
			Msg("Flag created")
	}

	return nil
}

func waitForServer(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	errc := make(chan error, 1)
	poll := func() error {
		resp, err := http.Get(path)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
		}

		return nil
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				errc <- ServerUnavailableErr
				return
			default:
				err := poll()
				switch err.(type) {
				case net.Error:
					time.Sleep(time.Second)
					continue
				default:
					errc <- err
					return
				}
			}
		}
	}()

	return <-errc
}

type Interception interface {
	ValidRequest(func(r *http.Request)) bool
	Intercept(http.Handler) http.Handler
}

type Interceptors []Interception

func (i Interceptors) Intercept(http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	})
}

type RegisterInterception struct {
	out chan store.Team
}

func (*RegisterInterception) ValidRequest(r *http.Request) bool {
	if r.URL.Path == "/register" && r.Method == http.MethodPost {
		return true
	}

	return false
}

func (*RegisterInterception) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pass := r.FormValue("password")
		r.Form.Set("password", fmt.Sprintf("%x", sha256.Sum256([]byte(pass))))

		next.ServeHTTP(w, r)
	})
}

type chalRes struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func (ctf *ctfd) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// we are only interested in post
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}

		// we only want these endpoints (and check)
		reqLogin := false
		reqLoginUsername := ""
		reqRegister := false
		reqChal := false

		if strings.Index(r.URL.Path, "/login") == 0 {
			reqLogin = true
		} else if strings.Index(r.URL.Path, "/register") == 0 {
			reqRegister = true
		} else if strings.Index(r.URL.Path, "/chal/") == 0 {
			reqChal = true
		} else {
			next.ServeHTTP(w, r)
			return
		}

		// log
		log.Info().
			Str("URL.Path", r.URL.Path).
			Str("Method", r.Method).
			Msg("CTFd.Middleware")

		// populate r.Form if body is available
		if r.Body != nil && (reqLogin || reqRegister) {
			// pass our body into parsequery to get post params
			buf := new(bytes.Buffer)
			buf.ReadFrom(r.Body)
			urlValues, err := url.ParseQuery(buf.String())

			if err != nil {
				log.Error().
					Msg("Failed to ParseQuery")
			}

			// set our username for later use
			reqLoginUsername = urlValues.Get("name")

			// init our hasher, and replace password
			hash := sha256.New()
			hash.Write([]byte(urlValues.Get("password")))
			hashHexed := hex.EncodeToString(hash.Sum(nil))
			urlValues.Set("password", hashHexed)

			if reqRegister {
				ctf.users = append(ctf.users, &user{
					teamname: urlValues.Get("name"),
					email:    urlValues.Get("email"),
					password: urlValues.Get("password"),
				})
			}

			postQuery := urlValues.Encode()
			clen := len(postQuery)

			r.Body = ioutil.NopCloser(bytes.NewReader([]byte(postQuery)))
			r.ContentLength = int64(clen)
		}

		// start a recorder, record the request and set the headers
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		for k, v := range rec.Header() {
			fmt.Printf("%s = %v\n", k, v)
			w.Header()[k] = v
		}

		// if rec.Code is 302 (redirect), then it was a success, so create a lab
		if reqLogin || reqRegister {
			if rec.Code == 302 {
				if reqLogin || reqRegister {
					user, err := ctf.findUser(reqLoginUsername)
					if err != nil {
						log.Error().
							Str("userInfo", reqLoginUsername).
							Err(err).
							Msg("Could not find user")
					}

					sessionToken, err := ctf.extractSession(rec.Header())
					if err != nil {
						log.Error().
							Err(err).
							Msg("Could not extract session")
					}

					ctf.relation[sessionToken] = user

					fmt.Println("----------")
					for _, user := range ctf.users {
						fmt.Printf("%+v\n", user)
					}
					fmt.Printf("%+v\n", ctf.relation)
				}

			}
		} else if reqChal {
			fmt.Println(reqChal)
			log.Debug().Msg("Challenge solution")
			var res chalRes
			if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
				log.Error().
					Err(err).
					Msg("Could not decode response")
				return
			}

			fmt.Printf("%+v\n", res)

			sessionToken, err := ctf.extractSession(r.Header)
			if err != nil {
				log.Error().
					Err(err).
					Msg("Could not extract session")
				return
			}

			// should check for errors
			user := ctf.relation[sessionToken]

			log.Debug().
				Str("solved", res.Message).
				Str("teamName", user.teamname).
				Msg("Challenge solution")

			b, err := json.Marshal(res)
			if err != nil {
				log.Error().
					Err(err).
					Msg("Could not encode")
				return
			}

			rec.Body = bytes.NewBuffer(b)
		}

		// set statuscode too ! (this writes out our headers)
		w.WriteHeader(rec.Code)
		// write out our body
		w.Write(rec.Body.Bytes())
	})
}

func (ctf *ctfd) extractSession(headers http.Header) (string, error) {
	for k, v := range headers {
		if strings.Contains(v[0], "session") {
			fmt.Printf("%s = %s\n", k, v)
			if strings.Index(v[0], ";") != -1 {
				return v[0][8:strings.Index(v[0], ";")], nil
			}
			return v[0][8:], nil
		}
	}

	return "", CouldNotFindSessionErr
}

func (ctf *ctfd) findUser(userinfo string) (*user, error) {
	for _, user := range ctf.users {
		if user.teamname == userinfo || user.email == userinfo {
			return user, nil
		}
	}

	return nil, UserNotFoundErr
}
