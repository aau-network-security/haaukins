package guacamole

import (
	"testing"
)

func TestNewFrame(t *testing.T) {
	tt := []struct {
		name         string
		input        string
		expectedArgs int
		expectErr    bool
	}{
		{
			name:         "Normal",
			input:        "4.sync,8.31163115,8.31163115;",
			expectedArgs: 2,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			b := []byte(tc.input)
			f, err := NewFrame(b)

			if (err == nil && tc.expectErr) || (err != nil && !tc.expectErr) {
				t.Fatalf("Expected error (%t), but got: %s", tc.expectErr, err)
			}
			if err == nil && len(f.args) != tc.expectedArgs {
				t.Fatalf("Expected %d args, but got %d", tc.expectedArgs, len(f.args))
			}
		})
	}
}

func TestKeyFrame(t *testing.T) {
	tt := []struct {
		name        string
		input       string
		expectedErr error
		key         Element
		pressed     Element
	}{
		{
			name:    "Normal",
			input:   "3.key,5.10000,1.0;",
			key:     "10000",
			pressed: "0",
		},
		{
			name:        "Invalid opcode",
			input:       "3.kez,5.10000,1.0;",
			expectedErr: InvalidOpcodeErr,
		},
		{
			name:        "Invalid number of args",
			input:       "3.key,5.10000",
			expectedErr: InvalidArgsErr,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			rf := []byte(tc.input)
			f, err := NewFrame(rf)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			kf, err := NewKeyFrame(f)
			if err != tc.expectedErr {
				t.Fatalf("Expected error (%t), but got: %s", tc.expectedErr, err)
			}
			if tc.expectedErr != nil {
				return
			}
			if kf.Key != tc.key {
				t.Fatalf("Expected key pressed to equal %s, but got %s", tc.key, kf.Key)
			}
			if kf.Pressed != tc.pressed {
				t.Fatalf("Expected key pressed to equal %s, but got %s", tc.pressed, kf.Pressed)
			}
		})
	}
}

func TestKeyFrameFilter(t *testing.T) {
	tt := []struct {
		name        string
		input       string
		expectedErr error
		expectedOk  bool
		key         Element
		pressed     Element
	}{
		{
			name:       "Normal",
			input:      "3.key,5.10000,1.0;",
			expectedOk: true,
			key:        "10000",
			pressed:    "0",
		},
		{
			name:       "Invalid opcode",
			input:      "5.mouse,3.100,4.1000,1.2;",
			expectedOk: false,
		},
		{
			name:        "Invalid number of args",
			input:       "3.key,5.10000;",
			expectedErr: InvalidArgsErr,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			kff := KeyFrameFilter{}

			rf := []byte(tc.input)
			kf, ok, err := kff.Filter(rf)
			if err != tc.expectedErr {
				t.Fatalf("Expected error (%t), but got: %s", tc.expectedErr, err)
			}
			if tc.expectedErr != nil {
				return
			}
			if ok != tc.expectedOk {
				t.Fatalf("Expected OK (%t), but got: %t", tc.expectedOk, ok)
			}
			if !tc.expectedOk {
				return
			}
			if kf.Key != tc.key {
				t.Fatalf("Expected key pressed to equal %s, but got %s", tc.key, kf.Key)
			}
			if kf.Pressed != tc.pressed {
				t.Fatalf("Expected key pressed to equal %s, but got %s", tc.pressed, kf.Pressed)
			}
		})
	}
}

func TestMouseFrame(t *testing.T) {
	tt := []struct {
		name        string
		input       string
		expectedErr error
		x           Element
		y           Element
		button      Element
	}{
		{
			name:   "Normal",
			input:  "5.mouse,3.100,4.1000,1.2;",
			x:      "100",
			y:      "1000",
			button: "2",
		},
		{
			name:        "Invalid opcode",
			input:       "5.mouze,3.100,4.1000,1.2;",
			expectedErr: InvalidOpcodeErr,
		},
		{
			name:        "Invalid number of args",
			input:       "5.mouse,3.100,4.1000;",
			expectedErr: InvalidArgsErr,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			rf := []byte(tc.input)
			f, err := NewFrame(rf)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			mf, err := NewMouseFrame(f)
			if err != tc.expectedErr {
				t.Fatalf("Expected error (%t), but got: %s", tc.expectedErr, err)
			}
			if tc.expectedErr != nil {
				return
			}
			if mf.X != tc.x {
				t.Fatalf("Expected x to equal %s, but got %s", tc.x, mf.X)
			}
			if mf.Y != tc.y {
				t.Fatalf("Expected x to equal %s, but got %s", tc.y, mf.Y)
			}
			if mf.Button != tc.button {
				t.Fatalf("Expected button to equal %s, but got %s", tc.button, mf.Button)
			}
		})
	}
}

func TestMouseFrameFilter(t *testing.T) {
	tt := []struct {
		name        string
		input       string
		expectedErr error
		expectedOk  bool
		x           Element
		y           Element
		button      Element
	}{
		{
			name:       "Normal",
			input:      "5.mouse,3.100,4.1000,1.2;",
			expectedOk: true,
			x:          "100",
			y:          "1000",
			button:     "2",
		},
		{
			name:       "Invalid opcode",
			input:      "3.key,5.10000,1.0;",
			expectedOk: false,
		},
		{
			name:        "Invalid number of args",
			input:       "5.mouse,3.100,4.1000;",
			expectedErr: InvalidArgsErr,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			mff := MouseFrameFilter{}

			rf := []byte(tc.input)
			mf, ok, err := mff.Filter(rf)
			if err != tc.expectedErr {
				t.Fatalf("Expected error (%t), but got: %s", tc.expectedErr, err)
			}
			if tc.expectedErr != nil {
				return
			}
			if ok != tc.expectedOk {
				t.Fatalf("Expected OK (%t), but got: %t", tc.expectedOk, ok)
			}
			if !tc.expectedOk {
				return
			}
			if mf.X != tc.x {
				t.Fatalf("Expected x to equal %s, but got %s", tc.x, mf.X)
			}
			if mf.Y != tc.y {
				t.Fatalf("Expected x to equal %s, but got %s", tc.y, mf.Y)
			}
			if mf.Button != tc.button {
				t.Fatalf("Expected button to equal %s, but got %s", tc.button, mf.Button)
			}
		})
	}
}
