// Package argon2 provides low-level bindings for the Argon2 hashing library:
// libargon2. Argon2 specifies three versions: Argon2i, Argon2d, and Argon2id.
// Argon2i is useful for protection against side-channel attacks (key
// derivation), while Argon2d provides the highest resistance against GPU
// cracking attacks (proof-of-work). Argon2id provides good protection against
// both side-channel and GPU cracking attacks.
package argon2

// #cgo CFLAGS: -I${SRCDIR}/argon2/include
// #include <stdlib.h>
// #include <argon2.h>
// #include "argon2/src/argon2.c"
// #include "argon2/src/core.c"
// #include "argon2/src/blake2/blake2b.c"
// #include "argon2/src/thread.c"
// #include "argon2/src/encoding.c"
// #include "argon2/src/opt.c"
import "C"

import (
	"bytes"
	"crypto/subtle"
	"strings"
	"unsafe"
)

const (
	ModeArgon2d  int = C.Argon2_d
	ModeArgon2i  int = C.Argon2_i
	ModeArgon2id int = C.Argon2_id
)

const (
	Version10      int = C.ARGON2_VERSION_10
	Version13      int = C.ARGON2_VERSION_13
	VersionDefault int = C.ARGON2_VERSION_NUMBER
)

const (
	FlagDefault       int = C.ARGON2_DEFAULT_FLAGS
	FlagClearPassword int = C.ARGON2_FLAG_CLEAR_PASSWORD
	FlagClearSecret   int = C.ARGON2_FLAG_CLEAR_SECRET
)

// Set C.FLAG_clear_internal_memory = 0 to disable internal memory clearing

// Hash hashes a password given a salt and an initialized Argon2 context. It
// returns the calculated hash as an output of raw bytes.
func Hash(ctx *Context, password, salt []byte) ([]byte, error) {
	if ctx == nil {
		return nil, ErrContext
	}

	return ctx.hash(password, salt)
}

// HashEncoded hashes a password and produces a crypt-like encoded string.
func HashEncoded(ctx *Context, password []byte, salt []byte) (string, error) {
	if ctx == nil {
		return "", ErrContext
	}

	if len(password) == 0 {
		return "", ErrPassword
	}
	if len(salt) == 0 {
		return "", ErrSalt
	}

	encodedlen := C.argon2_encodedlen(
		C.uint32_t(ctx.Iterations),
		C.uint32_t(ctx.Memory),
		C.uint32_t(ctx.Parallelism),
		C.uint32_t(len(salt)),
		C.uint32_t(ctx.HashLen),
		C.argon2_type(ctx.Mode))

	s := make([]byte, encodedlen)

	result := C.argon2_hash(
		C.uint32_t(ctx.Iterations),
		C.uint32_t(ctx.Memory),
		C.uint32_t(ctx.Parallelism),
		unsafe.Pointer(&password[0]), C.size_t(len(password)),
		unsafe.Pointer(&salt[0]), C.size_t(len(salt)),
		nil, C.size_t(ctx.HashLen),
		(*C.char)(unsafe.Pointer(&s[0])), C.size_t(encodedlen),
		C.argon2_type(ctx.Mode),
		C.uint32_t(ctx.Version))

	if result != C.ARGON2_OK {
		return "", Error(result)
	}

	// Strip trailing null byte(s)
	s = bytes.TrimRight(s, "\x00")
	return string(s), nil
}

// Verify verifies an Argon2 hash against a plaintext password.
func Verify(ctx *Context, hash, password, salt []byte) (bool, error) {
	if ctx == nil {
		return false, ErrContext
	}
	if len(hash) == 0 {
		return false, ErrHash
	}

	hash2, err := ctx.hash(password, salt)
	if err != nil {
		return false, err
	}

	return subtle.ConstantTimeCompare(hash, hash2) == 1, nil
}

// VerifyEncoded verifies an encoded Argon2 hash s against a plaintext password.
func VerifyEncoded(s string, password []byte) (bool, error) {
	mode, err := getMode(s)

	if err != nil {
		return false, err
	}

	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))

	result := C.argon2_verify(
		cs,
		unsafe.Pointer(&password[0]),
		C.size_t(len(password)),
		C.argon2_type(mode))

	if result == C.ARGON2_OK {
		return true, nil
	} else if result == C.ARGON2_VERIFY_MISMATCH {
		return false, nil
	}

	// argon2_verify always seems to return an error in this case...
	return false, Error(result)
}

// getMode tries to extract the mode from an Argon2 encoded string.
func getMode(s string) (int, error) {
	switch {
	case strings.HasPrefix(s, "$argon2d"):
		return ModeArgon2d, nil
	case strings.HasPrefix(s, "$argon2id"):
		return ModeArgon2id, nil
	case strings.HasPrefix(s, "$argon2i"):
		return ModeArgon2i, nil
	default:
		return -1, ErrDecodingFail
	}
}
