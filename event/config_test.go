package event

import (
	"testing"
)

func TestLoadConfig(t *testing.T) {
	conf, _ := loadConfig("test_resources/test_event.yml")
	if conf.ctfd.Name != "Test CTFd" {
		t.Fatalf("Unexpected CTFd name (expected 'Test CTFd'): %s", conf.ctfd.Name)
	}
	if conf.guac.Host != "localhost" {
		t.Fatalf("Unexpected Guacamole host (expected 'localhost'): %s", conf.guac.Host)
	}
	if conf.revproxy.Host != "127.0.0.1" {
		t.Fatalf("Unexpected reverse proxy host (expected '127.0.0.1'): %s", conf.revproxy.Host)
	}
}
