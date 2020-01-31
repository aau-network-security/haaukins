package haaukins

import (
	"bytes"
	"errors"
	"math/rand"
	"time"
	"unsafe"
)

var (
	ErrInvalidFlagFormat = errors.New("Invalid flag format")
	ErrEmptyStaticFlag   = errors.New("Static flags cannot be empty")
)

const (
	// Alphabet for flags currently alphanumeric with the exception of: o, O, 0 (to remove)
	letterBytes     = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ12345679"
	letterIdxBits   = 6                    // 6 bits to represent a letter index
	letterIdxMask   = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax    = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
	flagUniqueChars = 10
)

var flagSrc = rand.NewSource(time.Now().UnixNano())

func randCharBytes(n int) string {
	b := make([]byte, n)
	for i, cache, remain := n-1, flagSrc.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = flagSrc.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}

type Flag interface {
	IsEqual(string) bool
	String() string
}

type FlagShort [flagUniqueChars]byte

func NewFlagShort() FlagShort {
	var arr [flagUniqueChars]byte
	s := []byte(randCharBytes(10))
	copy(arr[:], s)
	return FlagShort(arr)
}

func NewFlagShortFromString(s string) (FlagShort, error) {
	b := bytes.Replace([]byte(s), []byte("-"), []byte(""), 2)
	if len(b) != flagUniqueChars {
		return FlagShort{}, ErrInvalidFlagFormat
	}
	var arr [flagUniqueChars]byte
	copy(arr[:], b)

	return FlagShort(arr), nil
}

func (f FlagShort) String() string {
	str := string(f[:])
	i := (2 + rand.Intn(2))
	j := (i + 2 + rand.Intn(2))

	return str[:i] + "-" + str[i:j] + "-" + str[j:]
}

func (f FlagShort) IsEqual(s string) bool {
	other, err := NewFlagShortFromString(s)
	if err != nil {
		return false
	}

	return f == other
}

type FlagStatic string

func NewFlagStatic(s string) (FlagStatic, error) {
	if s == "" {
		return "", ErrEmptyStaticFlag
	}

	return FlagStatic(s), nil
}

func (f FlagStatic) IsEqual(s string) bool {
	return f == FlagStatic(s)
}

func (f FlagStatic) String() string {
	return string(f)
}
