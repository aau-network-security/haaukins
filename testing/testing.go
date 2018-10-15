package testing

import (
	"testing"
	"os"
)

func SkipCI(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skipf("Ignore test %s in CI environment", t.Name())
	}
}