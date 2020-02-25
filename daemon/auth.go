// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aau-network-security/haaukins/store"
	jwt "github.com/dgrijalva/jwt-go"
	"google.golang.org/grpc/metadata"
)

const (
	USERNAME_KEY    = "un"
	SUPERUSER_KEY   = "su"
	NONPRIVUSER_KEY = "mm"
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
	AuthenticateContext(context.Context) (context.Context, error)
}

type auth struct {
	us  store.UserStore
	key string
}

type us struct{}

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
		USERNAME_KEY:    u.Username,
		SUPERUSER_KEY:   u.SuperUser,
		NONPRIVUSER_KEY: u.NonPrivUser,
		VALID_UNTIL_KEY: time.Now().Add(31 * 24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(a.key))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (a *auth) AuthenticateContext(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, MissingTokenErr
	}

	if len(md["token"]) == 0 {
		return ctx, MissingTokenErr
	}

	token := md["token"][0]
	if token == "" {
		return ctx, MissingTokenErr
	}

	jwtToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return ctx, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(a.key), nil
	})
	if err != nil {
		return ctx, err
	}

	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok || !jwtToken.Valid {
		return ctx, InvalidTokenFormatErr
	}

	username, ok := claims[USERNAME_KEY].(string)
	if !ok {
		return ctx, InvalidTokenFormatErr
	}

	u, err := a.us.GetUserByUsername(username)
	if err != nil {
		return ctx, UnknownUserErr
	}

	validUntil, ok := claims[VALID_UNTIL_KEY].(float64)
	if !ok {
		return ctx, InvalidTokenFormatErr
	}

	if int64(validUntil) < time.Now().Unix() {
		return ctx, TokenExpiredErr
	}

	ctx = context.WithValue(ctx, us{}, u)

	return ctx, nil
}
