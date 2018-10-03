package daemon

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"testing"

	"context"
	"fmt"
	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"log"
	"net"
	"time"
)

var lis *bufconn.Listener

type testUserHub struct {
	keyValue string
	err      error
	UserHub
}

func (t testUserHub) CreateSignupKey() (SignupKey, error) {
	return SignupKey(t.keyValue), t.err
}

func init() {
	lis = bufconn.Listen(1024 * 1024)
	d := &daemon{
		uh: testUserHub{},
	}
	s := d.GetServer()

	pb.RegisterDaemonServer(s, d)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
}

func bufDialer(string, time.Duration) (net.Conn, error) {
	return lis.Dial()
}

func TestNoToken(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.Dial("bufnet", grpc.WithDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewDaemonClient(conn)
	req := &pb.InviteUserRequest{}
	_, err = client.InviteUser(ctx, req)
	if err == nil {
		t.Fatalf("Expected an error, but did not receive one")
	}
}

func TestInviteUser(t *testing.T) {

	cases := []struct {
		name     string
		keyValue string
		err      error
	}{
		{"Valid SignupKey", "1", nil},
		{"Error in retrieving SignupKey", "", fmt.Errorf("failure")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			d := &daemon{
				uh: testUserHub{
					keyValue: c.keyValue,
					err:      c.err,
				},
			}
			req := &pb.InviteUserRequest{}
			resp, _ := d.InviteUser(ctx, req)

			if resp.Key != c.keyValue {
				t.Fatalf("Expected key '%s', but got '%s'", c.keyValue, resp.Key)
			}
			expectedErrMsg := ""
			if c.err != nil {
				expectedErrMsg = c.err.Error()
			}

			if resp.Error != expectedErrMsg {
				t.Fatalf("Expected error: '%s', but got '%s'", expectedErrMsg, resp.Error)
			}
		})
	}
}
