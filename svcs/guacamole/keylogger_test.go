package guacamole

import (
	"bufio"
	"github.com/aau-network-security/go-ntp/store"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKeyLogger(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	logpool := NewKeyLoggerPool(tmpDir)
	team := store.Team{
		Id: "team",
	}

	logger, err := logpool.GetLogger(team)
	if err != nil {
		t.Fatalf("Unexpected error while getting logger: %s", err)
	}
	logger.Log([]byte("3.key,5.10000,1.0;"))
	logger.Log([]byte("5.mouse,3.100,4.1000,1.2;"))
	logger.Log([]byte("4.sync,8.31163115,8.31163115;"))

	time.Sleep(10 * time.Millisecond)

	expectedFn := filepath.Join(tmpDir, "team.log")
	f, err := os.Open(expectedFn)
	if err != nil {
		t.Fatalf("Failed to open file: %s", err)
	}

	nLines := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		scanner.Text()
		nLines++
	}
	if nLines != 2 {
		t.Fatalf("Expected 2 lines in log file, but got %d", nLines)
	}
}
