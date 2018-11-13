package guacamole

import (
	"testing"
)

func TestNewMessage(t *testing.T) {
	tt := []struct {
		name      	 string
		input     	 string
		expectedArgs int
		expectErr    bool
	}{
		{
			name: "Normal",
			input: "4.sync,8.31163115,8.31163115;",
			expectedArgs: 2,
		},
		{
			name: "Invalid length",
			input: "a.sync;",
			expectErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			b := []byte(tc.input)
			msg, err := NewMessage(b)

			if (err == nil && tc.expectErr) || (err != nil && !tc.expectErr) {
				t.Fatalf("Expected error (%t), but got: %s", tc.expectErr, err)
			}
			if err == nil && len(msg.args) != tc.expectedArgs {
				t.Fatalf("Expected %d args, but got %d", tc.expectedArgs, len(msg.args))
			}
		})
	}
}

func TestMessageFilter(t *testing.T) {
	tt := []struct {
		name          string
		input         string
		expectedArgs  int
		expectDropped bool
		expectErr     bool
	}{
		{
			name:         "Normal",
			input:        "4.sync,8.31163115,8.31163115;",
			expectedArgs: 2,
		},
		{
			name:      "Invalid length",
			input:     "a.sync;",
			expectErr: true,
		},
		{
			name:          "Filtered opcode",
			input:         "7.dropped;",
			expectDropped: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			b := []byte(tc.input)
			mf := MessageFilter{
				opcodes: []string{
					"droppe",
				},
			}
			msg, dropped, err := mf.Filter(b)
			if (err == nil && tc.expectErr) || (err != nil && !tc.expectErr) {
				t.Fatalf("Expected error (%t), but got: %s", tc.expectErr, err)
			}
			if dropped != tc.expectDropped {
				t.Fatalf("Expected filtered (%t), but got: %t", tc.expectDropped, dropped)
			}
			if !dropped && err == nil && len(msg.args) != tc.expectedArgs {
				t.Fatalf("Expected %d args, but got %d", tc.expectedArgs, len(msg.args))
			}
		})
	}
}