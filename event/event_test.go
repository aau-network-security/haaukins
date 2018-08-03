package event

import (
	"testing"
)

func TestNewEvent(t *testing.T) {
	expected := 2

	ev, _ := New("test_resources/test_event.yml", "test_resources/test_lab.yml")
	nEndpoints := ev.Proxy.NumberOfEndpoints()
	if nEndpoints != expected {
		t.Fatalf("Unexpected number of endpoints (expected %d): %d", expected, nEndpoints)
	}

	expected = 3
	ctfdFlags := ev.CTFd.Flags()
	if len(ctfdFlags) != expected {
		t.Fatalf("Unexpected number of flags (expected %d): %d", expected, len(ctfdFlags))
	}
	expectedFlagName := "First flag"
	if ctfdFlags[0].Name != expectedFlagName {
		t.Fatalf("Unexpected flag name (expected %v): %v", expectedFlagName, ctfdFlags[0].Name)
	}
}
