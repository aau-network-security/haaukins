package daemon

import (
	"errors"
	"fmt"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

const (
	USERNAME_KEY    = "un"
	VALID_UNTIL_KEY = "vu"
)

var (
	UnknownSignupKey         = errors.New("Unknown signup key")
	UserAlreadyExistsErr     = errors.New("User already exists")
	UnknownUserErr           = errors.New("Unknown user")
	InvalidUsernameOrPassErr = errors.New("Invalid username or password")
	InvalidTokenFormatErr    = errors.New("Invalid token format")
	TokenExpiredErr          = errors.New("Token has expired")
)

type User struct {
	Username       string    `yaml:"username"`
	HashedPassword string    `yaml:"hashed-password"`
	CreatedAt      time.Time `yaml:"created-at"`
}

func NewUser(username, password string) (*User, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return &User{
		Username:       strings.ToLower(username),
		HashedPassword: string(hashedBytes[:]),
		CreatedAt:      time.Now(),
	}, nil
}

func (u *User) IsCorrectPassword(pass string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.HashedPassword), []byte(pass)) == nil
}

type SignupKey string

func NewSignupKey() SignupKey {
	return SignupKey(uuid.New().String())
}

type UserHub struct {
	conf       *Config
	users      map[string]*User
	signupKeys map[SignupKey]struct{}
}

func NewUserHub(conf *Config) *UserHub {
	users := map[string]*User{}
	for i, _ := range conf.Users {
		u := conf.Users[i]
		users[u.Username] = &u
	}

	signupKeys := map[SignupKey]struct{}{}
	for i, _ := range conf.SignupKeys {
		k := conf.SignupKeys[i]
		signupKeys[k] = struct{}{}
	}

	uh := &UserHub{
		conf:       conf,
		users:      users,
		signupKeys: signupKeys,
	}

	if len(uh.users) == 0 && len(signupKeys) == 0 {
		k, err := uh.CreateSignupKey()
		if err != nil {
			log.Info().Err(err).Msg("Unable to add signup key")
		}

		log.Info().Str("key", string(k)).Msg("no users found, use key to signup")
	}

	return uh
}

func (uh *UserHub) CreateSignupKey() (SignupKey, error) {
	k := NewSignupKey()
	if err := uh.conf.AddSignupKey(k); err != nil {
		return "", err
	}
	uh.signupKeys[k] = struct{}{}

	return k, nil
}

func (uh *UserHub) AddUser(k SignupKey, username, password string) error {
	if _, ok := uh.signupKeys[k]; !ok {
		return UnknownSignupKey
	}

	if err := uh.conf.DeleteSignupKey(k); err != nil {
		return err
	}
	delete(uh.signupKeys, k)

	if _, ok := uh.users[username]; ok {
		return UserAlreadyExistsErr
	}

	u, err := NewUser(username, password)
	if err != nil {
		return err
	}

	if err := uh.conf.AddUser(u); err != nil {
		return err
	}

	uh.users[username] = u

	return nil
}

func (uh *UserHub) DeleteUser(username string) error {
	username = strings.ToLower(username)

	if _, ok := uh.users[username]; !ok {
		return UnknownUserErr
	}

	if err := uh.conf.DeleteUserByUsername(username); err != nil {
		return err
	}
	delete(uh.users, username)

	return nil
}

func (uh *UserHub) TokenForUser(username, password string) (string, error) {
	username = strings.ToLower(username)

	u, ok := uh.users[username]
	if !ok {
		return "", InvalidUsernameOrPassErr
	}

	if ok := u.IsCorrectPassword(password); !ok {
		return "", InvalidUsernameOrPassErr
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		USERNAME_KEY:    username,
		VALID_UNTIL_KEY: time.Now().Add(31 * 24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(uh.conf.SecretSigningKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (uh *UserHub) AuthenticateUserByToken(t string) error {
	token, err := jwt.Parse(t, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(uh.conf.SecretSigningKey), nil
	})
	if err != nil {
		return err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		username, ok := claims[USERNAME_KEY].(string)
		if !ok {
			fmt.Println("user miss")
			return InvalidTokenFormatErr
		}

		if _, ok := uh.users[username]; !ok {
			return UnknownUserErr
		}

		validUntil, ok := claims[VALID_UNTIL_KEY].(float64)
		if !ok {
			return InvalidTokenFormatErr
		}

		if int64(validUntil) < time.Now().Unix() {
			return TokenExpiredErr
		}

		return nil
	}

	return err
}
