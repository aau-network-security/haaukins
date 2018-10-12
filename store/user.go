package store

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	yaml "gopkg.in/yaml.v2"
)

var (
	UserStoreNoFileErr = errors.New("Unable to find user store file")
	UserExistsErr      = errors.New("User already exists")
	UserNotFoundErr    = errors.New("User not found")

	SignupKeyExistsErr   = errors.New("SignupKey already exists")
	SignupKeyNotFoundErr = errors.New("SignupKey not found")
)

type User struct {
	Username       string    `yaml:"username"`
	HashedPassword string    `yaml:"hashed-password"`
	CreatedAt      time.Time `yaml:"created-at"`
}

func NewUser(username, password string) (User, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}

	return User{
		Username:       strings.ToLower(username),
		HashedPassword: string(hashedBytes[:]),
	}, nil
}

func (u User) IsCorrectPassword(pass string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.HashedPassword), []byte(pass)) == nil
}

type UserStore interface {
	DeleteUserByUsername(string) error
	CreateUser(User) error
	GetUserByUsername(string) (User, error)
	ListUsers() []User
}

type userstore struct {
	m       sync.RWMutex
	userMap map[string]*User
	hooks   []func([]User) error
	users   []User
}

func NewUserStore(users []User, hooks ...func([]User) error) UserStore {
	s := userstore{
		userMap: map[string]*User{},
		users:   users,
		hooks:   hooks,
	}

	for i, _ := range users {
		u := users[i]
		s.userMap[u.Username] = &u
	}

	return &s
}

func (us *userstore) DeleteUserByUsername(username string) error {
	us.m.Lock()
	defer us.m.Unlock()

	_, ok := us.userMap[username]
	if !ok {
		return UserNotFoundErr
	}

	delete(us.userMap, username)

	for i, cu := range us.users {
		if username == cu.Username {
			us.users = append(us.users[:i], us.users[i+1:]...)
			break
		}
	}

	return us.RunHooks()
}

func (us *userstore) GetUserByUsername(username string) (User, error) {
	us.m.RLock()
	defer us.m.RUnlock()

	u, ok := us.userMap[username]
	if !ok {
		return User{}, UserNotFoundErr
	}

	return *u, nil

}

func (us *userstore) ListUsers() []User {
	us.m.Lock()
	defer us.m.Unlock()

	return us.users
}

func (us *userstore) CreateUser(u User) error {
	us.m.Lock()
	defer us.m.Unlock()

	_, ok := us.userMap[u.Username]
	if ok {
		return UserExistsErr
	}

	u.CreatedAt = time.Now()

	us.userMap[u.Username] = &u
	us.users = append(us.users, u)

	return us.RunHooks()
}

func (us *userstore) RunHooks() error {
	for _, h := range us.hooks {
		if err := h(us.users); err != nil {
			return err
		}
	}

	return nil
}

type SignupKey string

func NewSignupKey() SignupKey {
	return SignupKey(uuid.New().String())
}

type SignupKeyStore interface {
	CreateSignupKey(SignupKey) error
	DeleteSignupKey(SignupKey) error
	ListSignupKeys() []SignupKey
}

type signupkeystore struct {
	m      sync.Mutex
	keyMap map[SignupKey]struct{}
	hooks  []func([]SignupKey) error
}

func NewSignupKeyStore(keys []SignupKey, hooks ...func([]SignupKey) error) SignupKeyStore {
	s := signupkeystore{
		keyMap: map[SignupKey]struct{}{},
		hooks:  hooks,
	}

	for _, k := range keys {
		s.keyMap[k] = struct{}{}
	}

	return &s
}

func (ss *signupkeystore) CreateSignupKey(k SignupKey) error {
	_, ok := ss.keyMap[k]
	if ok {
		return SignupKeyExistsErr
	}

	ss.keyMap[k] = struct{}{}

	list := ss.ListSignupKeys()
	return ss.RunHooks(list)
}

func (ss *signupkeystore) DeleteSignupKey(k SignupKey) error {
	_, ok := ss.keyMap[k]
	if !ok {
		return SignupKeyNotFoundErr
	}

	delete(ss.keyMap, k)

	list := ss.ListSignupKeys()
	return ss.RunHooks(list)
}

func (ss *signupkeystore) ListSignupKeys() []SignupKey {
	keys := make([]SignupKey, len(ss.keyMap))

	var i int
	for k, _ := range ss.keyMap {
		keys[i] = k
		i++
	}

	return keys
}

func (ss *signupkeystore) RunHooks(keys []SignupKey) error {
	for _, h := range ss.hooks {
		if err := h(keys); err != nil {
			return err
		}
	}

	return nil
}

type UsersFile interface {
	UserStore
	SignupKeyStore
}

func NewUserFile(path string) (UsersFile, error) {
	var conf struct {
		Users      []User      `yaml:"users"`
		SignupKeys []SignupKey `yaml:"signup-keys"`
	}

	var m sync.Mutex
	save := func() error {
		m.Lock()
		defer m.Unlock()

		bytes, err := yaml.Marshal(conf)
		if err != nil {
			return err
		}

		return ioutil.WriteFile(path, bytes, 0644)
	}

	// file exists
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		f, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(f, &conf)
		if err != nil {
			return nil, err
		}
	}

	return &struct {
		UserStore
		SignupKeyStore
	}{
		NewUserStore(conf.Users, func(u []User) error {
			conf.Users = u
			return save()
		}),
		NewSignupKeyStore(conf.SignupKeys, func(k []SignupKey) error {
			conf.SignupKeys = k
			return save()
		}),
	}, nil
}
