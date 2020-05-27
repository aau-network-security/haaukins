package daemon

import (
	"context"
	"fmt"
	"testing"

	"github.com/aau-network-security/haaukins/client/cli"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func (a *noAuth) TokenForUser(username, password string) (string, error) {
	return respToken, nil
}

func TestSignupUser(t *testing.T) {
	tt := []struct {
		name      string
		createKey bool
		user      string
		pass      string
		err       string
	}{
		{name: "Normal", createKey: true, user: "tkp", pass: "tkptkp"},
		{name: "Too short password", createKey: true, user: "tkp", pass: "tkp", err: "Password too short, requires at least six characters"},
		{name: "No key", user: "tkp", pass: "tkptkp", err: "Signup key not found"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var key store.SignupKey
			var signupKeys []store.SignupKey
			if tc.createKey {
				key = store.NewSignupKey()
				signupKeys = append(signupKeys, key)
			}

			ks := store.NewSignupKeyStore(signupKeys)
			us := store.NewUserStore([]store.User{})

			ctx := context.Background()
			d := &daemon{
				auth: &noAuth{
					allowed: true,
				},
				users: struct {
					store.SignupKeyStore
					store.UserStore
				}{
					ks,
					us,
				},
			}

			dialer, close := getServer(d)
			defer close()

			conn, err := grpc.DialContext(ctx, "bufnet",
				grpc.WithDialer(dialer),
				grpc.WithInsecure(),
			)

			if err != nil {
				t.Fatalf("failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewDaemonClient(conn)
			resp, err := client.SignupUser(ctx, &pb.SignupUserRequest{
				Key:      key.String(),
				Username: tc.user,
				Password: tc.pass,
			})
			if err != nil {
				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: %s) received: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("expected no error, but received: %s", err)
			}

			if resp.Error != "" {
				if tc.err != "" {
					if tc.err != resp.Error {
						t.Fatalf("unexpected response error (expected: %s) received: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("expected no response error, but received: %s", resp.Error)
			}

			if tc.err != "" {
				t.Fatalf("expected error, but received none")
			}

			if len(us.ListUsers()) != 1 {
				t.Fatalf("expected one user to have been created in store")
			}

			if resp.Token != respToken {
				t.Fatalf("unexpected token (expected: %s) in response: %s", respToken, resp.Token)
			}
		})
	}
}

func TestInviteUser(t *testing.T) {
	tt := []struct {
		name      string
		token     string
		allowed   bool
		superuser bool
		err       string
	}{
		{name: "Normal with auth and super", allowed: true, superuser: true},
		{name: "No super with auth", allowed: true, err: "This action requires super user permissions"},
		{name: "Unauthorized", allowed: false, err: "unauthorized"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewSignupKeyStore([]store.SignupKey{})

			ctx := context.Background()
			d := &daemon{
				auth: &noAuth{
					allowed:   tc.allowed,
					superuser: tc.superuser,
				},
				users: struct {
					store.SignupKeyStore
					store.UserStore
				}{
					s,
					store.NewUserStore([]store.User{}),
				},
			}

			dialer, close := getServer(d)
			defer close()

			conn, err := grpc.DialContext(ctx, "bufnet",
				grpc.WithDialer(dialer),
				grpc.WithInsecure(),
				grpc.WithPerRPCCredentials(cli.Creds{Token: tc.token, Insecure: true}),
			)

			if err != nil {
				t.Fatalf("failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewDaemonClient(conn)
			resp, err := client.InviteUser(ctx, &pb.InviteUserRequest{})
			if resp != nil && resp.Error != "" {
				err = fmt.Errorf(resp.Error)
			}

			if err != nil {
				st, ok := status.FromError(err)
				if ok {
					err = fmt.Errorf(st.Message())
				}

				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: %s) received: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("expected no error, but received: %s", err)
			}

			if tc.err != "" {
				t.Fatalf("expected error, but received none")
			}

			if resp.Key == "" {
				t.Fatalf("expected key not be empty string")
			}

			if len(s.ListSignupKeys()) != 1 {
				t.Fatalf("expected one key to have been inserted into store")
			}

		})
	}
}

func TestLoginUser(t *testing.T) {
	type user struct {
		u string
		n string
		s string
		e string
		p string
	}

	tt := []struct {
		name       string
		createUser bool
		user       user
		err        string
	}{
		{name: "Normal", createUser: true, user: user{u: "tkp", n: "", p: "tkptkp"}},
		{name: "Unknown user", user: user{u: "tkp", p: "tkptkp"}, err: "Invalid username or password"},
		{name: "No username", user: user{u: "", p: "whatever"}, err: "Username cannot be empty"},
		{name: "No password", user: user{u: "tkp", p: ""}, err: "Password cannot be empty"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var users []store.User
			if tc.createUser {
				u, err := store.NewUser(tc.user.u, tc.user.n, tc.user.s, tc.user.e, tc.user.p)
				if err != nil {
					t.Fatalf("unexpected error when creating user: %s", err)
				}

				users = append(users, u)
			}

			us := store.NewUserStore(users)
			auth := NewAuthenticator(us, "some-signing-key")
			ctx := context.Background()
			d := &daemon{
				auth: auth,
				users: struct {
					store.SignupKeyStore
					store.UserStore
				}{
					nil,
					us,
				},
			}

			dialer, close := getServer(d)
			defer close()

			conn, err := grpc.DialContext(ctx, "bufnet",
				grpc.WithDialer(dialer),
				grpc.WithInsecure(),
			)

			if err != nil {
				t.Fatalf("failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewDaemonClient(conn)
			resp, err := client.LoginUser(ctx, &pb.LoginUserRequest{
				Username: tc.user.u,
				Password: tc.user.p,
			})
			if err != nil {
				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: %s) received: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("expected no error, but received: %s", err)
			}

			if resp.Error != "" {
				if tc.err != "" {
					if tc.err != resp.Error {
						t.Fatalf("unexpected response error (expected: %s) received: %s", tc.err, resp.Error)
					}

					return
				}

				t.Fatalf("expected no response error, but received: %s", resp.Error)
			}

			if tc.err != "" {
				t.Fatalf("expected error, but received none")
			}

			if resp.Token == "" {
				t.Fatalf("expected token to be non-empty")
			}
		})
	}
}
