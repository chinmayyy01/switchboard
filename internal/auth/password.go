package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory     = 64 * 1024
	argonIterations = 3
	argonThreads    = 2
	argonKeyLength  = 32
	argonSaltLength = 16
)

var ErrInvalidHash = errors.New("invalid password hash")

func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonThreads, argonKeyLength)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonIterations,
		argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var memory, iterations uint32
	var threads uint8
	var err error
	for _, field := range strings.Split(parts[3], ",") {
		kv := strings.SplitN(field, "=", 2)
		if len(kv) != 2 {
			return false
		}
		switch kv[0] {
		case "m":
			var n uint64
			n, err = strconv.ParseUint(kv[1], 10, 32)
			memory = uint32(n)
		case "t":
			var n uint64
			n, err = strconv.ParseUint(kv[1], 10, 32)
			iterations = uint32(n)
		case "p":
			var n uint64
			n, err = strconv.ParseUint(kv[1], 10, 8)
			threads = uint8(n)
		default:
			return false
		}
		if err != nil {
			return false
		}
	}
	if memory == 0 || memory > 1024*1024 || iterations == 0 || iterations > 10 || threads == 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
