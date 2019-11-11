package errors

import (
	"errors"
	"github.com/rs/zerolog"
	"testing"
)

func TestE(t *testing.T) {

	tests := []struct {
		functionCall FCall
		err          error
		message      string
		level        zerolog.Level
	}{
		{functionCall: "daemon.New", err: errors.New("configuration file not found"), message: "error on daemon.New function", level: zerolog.ErrorLevel},
		{functionCall: "daemon.New", err: errors.New("No error informing ..."), message: "Daemon has been created", level: zerolog.InfoLevel},
		{functionCall: "auth.NewAuthenticator", err: errors.New("Warning: "), message: "key is plain text", level: zerolog.WarnLevel},
		{functionCall: "eventpool.RemoveEvent", err: errors.New("Debugging: "), message: "Removing event.", level: zerolog.DebugLevel},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			if err := E(tt.functionCall, tt.err, tt.message); err.Error() != (tt.err.Error() + " " + tt.message) {
				t.Errorf("E() error = %v", err)
			}
		})
	}
}

func TestSeverity(t *testing.T) {
	const functioncall FCall = "TestSeverity"
	message := "error to test out severity of zerolog levels"
	tests := []struct {
		functionCall FCall
		message      string
		level        zerolog.Level
	}{
		{functionCall: functioncall, message: message, level: zerolog.ErrorLevel},
	}
	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			if err := E(tt.functionCall, tt.message, tt.level); Severity(err) != tt.level {
				t.Fatalf("Error on getting severity %v", err)
			}
		})

	}
}
