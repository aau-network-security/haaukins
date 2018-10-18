package daemon

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aau-network-security/go-ntp/store"
	jwt "github.com/dgrijalva/jwt-go"
)

const (
	USERNAME_KEY    = "un"
	VALID_UNTIL_KEY = "vu"
)

var (
	InvalidUsernameOrPassErr = errors.New("Invalid username or password")
	InvalidTokenFormatErr    = errors.New("Invalid token format")
	TokenExpiredErr          = errors.New("Token has expired")
	UnknownUserErr           = errors.New("Unknown user")
	EmptyUserErr             = errors.New("Username cannot be empty")
	EmptyPasswdErr           = errors.New("Password cannot be empty")
)

type Authenticator interface {
	TokenForUser(username, password string) (string, error)
	AuthenticateUserByToken(t string) (*store.User, error)
}

type auth struct {
	us  store.UserStore
	key string
}

func NewAuthenticator(us store.UserStore, key string) Authenticator {
	return &auth{
		us:  us,
		key: key,
	}
}

func (a *auth) TokenForUser(username, password string) (string, error) {
	username = strings.ToLower(username)

	if username == "" {
		return "", EmptyUserErr
	}

	if password == "" {
		return "", EmptyPasswdErr
	}

	u, err := a.us.GetUserByUsername(username)
	if err != nil {
		return "", InvalidUsernameOrPassErr
	}

	if ok := u.IsCorrectPassword(password); !ok {
		return "", InvalidUsernameOrPassErr
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		USERNAME_KEY:    username,
		VALID_UNTIL_KEY: time.Now().Add(31 * 24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(a.key))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (a *auth) AuthenticateUserByToken(t string) (*store.User, error) {
	token, err := jwt.Parse(t, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(a.key), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		username, ok := claims[USERNAME_KEY].(string)
		if !ok {
			return nil, InvalidTokenFormatErr
		}

		u, err := a.us.GetUserByUsername(username)
		if err != nil {
			return nil, UnknownUserErr
		}

		validUntil, ok := claims[VALID_UNTIL_KEY].(float64)
		if !ok {
			return nil, InvalidTokenFormatErr
		}

		if int64(validUntil) < time.Now().Unix() {
			return nil, TokenExpiredErr
		}

		return &u, nil
	}

	return nil, err
}
