package daemon

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/aau-network-security/haaukins/client/cli"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func TestListFrontends(t *testing.T) {
	tt := []struct {
		name           string
		unauthorized   bool
		err            string
		expectedImages []string
	}{
		{
			name:           "Normal",
			expectedImages: []string{"1/1", "2/2"},
		},
		{
			name:         "Unauthorized",
			unauthorized: true,
			err:          "unauthorized",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("failed to create temporary directory")
			}
			defer os.RemoveAll(tmpDir)
			for _, dir := range []string{"1", "2"} {
				if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
					t.Fatalf("failed to created subdirectories")
				}
			}

			data := "content of file"
			filenames := []string{
				filepath.Join(tmpDir, "1", "1.ova"),
				filepath.Join(tmpDir, "1", "1.txt"),
				filepath.Join(tmpDir, "2", "2.ova"),
			}
			for _, fn := range filenames {
				f, err := os.Create(fn)
				if err != nil {
					t.Fatalf("failed to create '%s': %s", fn, err)
				}
				defer f.Close()
				if _, err := f.WriteString(data); err != nil {
					t.Fatalf("failed to write to '%s': %s", fn, err)
				}
			}

			ctx := context.Background()

			var d = &daemon{
				conf: &Config{
					ConfFiles: Files{OvaDir: tmpDir},
				},
				eventPool: NewEventPool(""),
				frontends: &fakeFrontendStore{},
				auth: &noAuth{
					allowed: !tc.unauthorized,
				},
			}
			dialer, close := getServer(d)
			defer close()

			conn, err := grpc.DialContext(ctx, "bufnet",
				grpc.WithDialer(dialer),
				grpc.WithInsecure(),
				grpc.WithPerRPCCredentials(cli.Creds{Insecure: true}),
			)

			if err != nil {
				t.Fatalf("failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewDaemonClient(conn)
			resp, err := client.ListFrontends(ctx, &pb.Empty{})

			if err != nil && err != io.EOF {
				st, ok := status.FromError(err)
				if ok {
					err = fmt.Errorf(st.Message())
				}
				if tc.err != "" && err.Error() != tc.err {
					t.Fatalf("expected error '%s', but got '%s'", tc.err, err.Error())
				}
				return
			}

			if len(resp.Frontends) != len(tc.expectedImages) {
				t.Fatalf("expected %d frontends, but got %d", len(tc.expectedImages), len(resp.Frontends))
			}

			for i, f := range resp.Frontends {
				if f.Image != tc.expectedImages[i] {
					t.Fatalf("expected image '%s', but got '%s'", tc.expectedImages[i], f.Image)
				}
			}
		})
	}
}
