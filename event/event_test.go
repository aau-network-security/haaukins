package event

import (
	"testing"
)

func TestNewEvent(t *testing.T) {
	ev, _ := New("test_resources/test_event.yml", "test_resources/test_lab.yml")
	nEndpoints := ev.Proxy.NumberOfEndpoints()
	if nEndpoints != 2 {
		t.Fatalf("Unexpected number of endpoints: %d", nEndpoints)
	}
}
