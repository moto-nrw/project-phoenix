package securityruntime

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// The cryptography of the data return (Data Act, ADR 0203). A released
// package is encrypted at rest with its own random archive key. The server
// never stores that key in the clear: each retrieval token wraps it, and the
// operator keeps it to issue further tokens. Without a live token, the
// stored archive and the database together do not open.

// DataReturnTokenPrefix marks a retrieval token, so secret scanners find it.
const DataReturnTokenPrefix = "mdr_"

const (
	dataReturnKeyBytes   = 32
	dataReturnChunkBytes = 64 << 10
	dataReturnWrapLabel  = "moto data return key wrap v1"
	dataReturnLookupTag  = "moto data return token lookup v1"
	dataReturnCheckLabel = "moto data return key check v1"
)

// dataReturnMagic heads every encrypted archive and names its format.
var dataReturnMagic = []byte("MDR1")

// ErrDataReturnCiphertext reports an archive or wrapped key that does not
// open: a wrong key, a wrong token, truncation or tampering.
var ErrDataReturnCiphertext = errors.New("data return: ciphertext does not open")

// NewDataReturnArchiveKey answers a fresh random archive key.
func NewDataReturnArchiveKey() ([]byte, error) {
	key := make([]byte, dataReturnKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("data return: random source: %w", err)
	}
	return key, nil
}

// EncodeDataReturnArchiveKey writes an archive key for the operator.
func EncodeDataReturnArchiveKey(key []byte) string {
	return base64.RawURLEncoding.EncodeToString(key)
}

// DecodeDataReturnArchiveKey reads an archive key the operator supplies.
func DecodeDataReturnArchiveKey(value string) ([]byte, error) {
	key, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(key) != dataReturnKeyBytes {
		return nil, errors.New("data return: malformed archive key")
	}
	return key, nil
}

// DataReturnKeyCheck answers a value that proves an archive key without
// revealing it, so a later grant can refuse a wrong key.
func DataReturnKeyCheck(key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(dataReturnCheckLabel))
	return hex.EncodeToString(mac.Sum(nil))
}

// DataReturnKeyMatches compares a key with its stored check in constant time.
func DataReturnKeyMatches(key []byte, check string) bool {
	return hmac.Equal([]byte(DataReturnKeyCheck(key)), []byte(check))
}

// NewDataReturnToken answers a fresh retrieval token: the prefix and 256
// random bits.
func NewDataReturnToken() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("data return: random source: %w", err)
	}
	return DataReturnTokenPrefix + base64.RawURLEncoding.EncodeToString(secret), nil
}

// DataReturnTokenLookup answers the value a grant is stored and found by.
// It reveals nothing of the token and differs from the wrapping key.
func DataReturnTokenLookup(token string) string {
	mac := hmac.New(sha256.New, []byte(dataReturnLookupTag))
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func dataReturnWrapAEAD(token string) (cipher.AEAD, error) {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(dataReturnWrapLabel))
	return newDataReturnAEAD(mac.Sum(nil))
}

func newDataReturnAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != dataReturnKeyBytes {
		return nil, errors.New("data return: archive key must have 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// SealDataReturnKey wraps an archive key under a retrieval token
// (AES-256-GCM, random nonce).
func SealDataReturnKey(token string, key []byte) ([]byte, error) {
	if len(key) != dataReturnKeyBytes {
		return nil, errors.New("data return: archive key must have 32 bytes")
	}
	aead, err := dataReturnWrapAEAD(token)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("data return: random source: %w", err)
	}
	return aead.Seal(nonce, nonce, key, []byte(dataReturnWrapLabel)), nil
}

// OpenDataReturnKey unwraps the archive key with the token that wrapped it.
func OpenDataReturnKey(token string, sealed []byte) ([]byte, error) {
	aead, err := dataReturnWrapAEAD(token)
	if err != nil {
		return nil, err
	}
	if len(sealed) < aead.NonceSize() {
		return nil, ErrDataReturnCiphertext
	}
	key, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], []byte(dataReturnWrapLabel))
	if err != nil || len(key) != dataReturnKeyBytes {
		return nil, ErrDataReturnCiphertext
	}
	return key, nil
}

// dataReturnNonce numbers the chunks of one archive; the last byte marks
// the final chunk, so a cut-off archive never opens as complete. Each
// archive has its own key, so counter nonces never repeat under a key.
func dataReturnNonce(counter uint64, final bool) []byte {
	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce[3:11], counter)
	if final {
		nonce[11] = 1
	}
	return nonce
}

// readChunks calls emit with each chunk of src and whether it is the last
// one, reading one chunk ahead. emit must use the chunk before it returns.
func readChunks(src io.Reader, size int, emit func(chunk []byte, final bool) error) error {
	current, next := make([]byte, size), make([]byte, size)
	n, err := readChunk(src, current)
	if err != nil {
		return err
	}
	for {
		m := 0
		if n == size {
			if m, err = readChunk(src, next); err != nil {
				return err
			}
		}
		final := n < size || m == 0
		if err := emit(current[:n], final); err != nil {
			return err
		}
		if final {
			return nil
		}
		current, next, n = next, current, m
	}
}

// readChunk fills buf as far as src goes; the end of src is no error.
func readChunk(src io.Reader, buf []byte) (int, error) {
	n, err := io.ReadFull(src, buf)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return n, nil
	}
	return n, err
}

// EncryptDataReturnArchive encrypts src to dst in chunks of 64 KiB, each
// sealed with AES-256-GCM under the archive key.
func EncryptDataReturnArchive(key []byte, dst io.Writer, src io.Reader) error {
	aead, err := newDataReturnAEAD(key)
	if err != nil {
		return err
	}
	if _, err := dst.Write(dataReturnMagic); err != nil {
		return err
	}
	var counter uint64
	return readChunks(src, dataReturnChunkBytes, func(chunk []byte, final bool) error {
		_, err := dst.Write(aead.Seal(nil, dataReturnNonce(counter, final), chunk, nil))
		counter++
		return err
	})
}

// DecryptDataReturnArchive writes the plaintext of an encrypted archive to
// dst. Each chunk is authenticated before it is written; a wrong key,
// tampering or truncation stops with ErrDataReturnCiphertext.
func DecryptDataReturnArchive(key []byte, dst io.Writer, src io.Reader) error {
	aead, err := newDataReturnAEAD(key)
	if err != nil {
		return err
	}
	magic := make([]byte, len(dataReturnMagic))
	if _, err := io.ReadFull(src, magic); err != nil || !bytes.Equal(magic, dataReturnMagic) {
		return ErrDataReturnCiphertext
	}
	var counter uint64
	return readChunks(src, dataReturnChunkBytes+aead.Overhead(), func(chunk []byte, final bool) error {
		plain, err := aead.Open(nil, dataReturnNonce(counter, final), chunk, nil)
		if err != nil {
			return ErrDataReturnCiphertext
		}
		counter++
		_, err = dst.Write(plain)
		return err
	})
}

// NewDataReturnFingerprint answers a writer and the SHA-256 in hex of what
// was written to it, for the plaintext archive as it is encrypted.
func NewDataReturnFingerprint() (io.Writer, func() string) {
	hash := sha256.New()
	return hash, func() string { return hex.EncodeToString(hash.Sum(nil)) }
}
