// +build ignore

package ctfd_test

import (
	"fmt"
	"testing"

	"github.com/aau-network-security/go-ntp/docker/ctfd"
)

func TestCTFd(t *testing.T) {
	c, _ := ctfd.New(
		ctfd.Config{
			Name:       "NTP",
			AdminUser:  "admin",
			AdminPass:  "admin",
			AdminEmail: "admin@admin.dk",
			Flags: []ctfd.Flag{{
				Name:  "Test",
				Flag:  "12345678",
				Value: 15,
			}},
		})

	fmt.Println(c.Start(nil))
}
