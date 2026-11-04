// Package fieldcipher seals single database values with AES-256-GCM (#4722).
package fieldcipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Cipher seals single database values with AES-256-GCM (#4722). Every
// sealed value names the key that sealed it, so the keys can rotate: the
// first key of the ring seals, every key of the ring opens. The caller binds
// each value to its context (table, column, owning row) as additional
// authenticated data, so a value copied into another row does not open.
//
// A sealed value is: format byte 1, the length of the key ID, the key ID,
// the 12-byte nonce, and the GCM ciphertext with its tag.
type Cipher struct {
	activeID string
	keys     map[string]cipher.AEAD
}

const (
	fieldCipherFormat = byte(1)
	fieldKeyBytes     = 32
)

var fieldKeyIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

// ErrOpen reports a value that does not open: an unknown key, a
// wrong context or a changed byte.
var ErrOpen = errors.New("sealed field value does not open")

// Parse reads a key ring of the form "id:base64key[,id:base64key]":
// each key is 32 random bytes in standard base64, each ID 1 to 32 letters,
// digits, "_" or "-". The first key seals new values. An error never names
// key material.
func Parse(spec string) (*Cipher, error) {
	entries := strings.Split(spec, ",")
	ring := &Cipher{keys: make(map[string]cipher.AEAD, len(entries))}
	for i, entry := range entries {
		id, encoded, found := strings.Cut(strings.TrimSpace(entry), ":")
		if !found || !fieldKeyIDPattern.MatchString(id) {
			return nil, fmt.Errorf("field cipher: entry %d needs the form id:base64key with an id of 1 to 32 letters, digits, _ or -", i+1)
		}
		if _, dup := ring.keys[id]; dup {
			return nil, fmt.Errorf("field cipher: key id %q appears twice", id)
		}
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(key) != fieldKeyBytes {
			return nil, fmt.Errorf("field cipher: key %q must be %d bytes in standard base64", id, fieldKeyBytes)
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("field cipher: key %q: %w", id, err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("field cipher: key %q: %w", id, err)
		}
		ring.keys[id] = aead
		if i == 0 {
			ring.activeID = id
		}
	}
	return ring, nil
}

// Seal encrypts plaintext with the active key, bound to context.
func (c *Cipher) Seal(plaintext, context []byte) ([]byte, error) {
	aead := c.keys[c.activeID]
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("field cipher: nonce: %w", err)
	}
	out := make([]byte, 0, 2+len(c.activeID)+len(nonce)+len(plaintext)+aead.Overhead())
	out = append(out, fieldCipherFormat, byte(len(c.activeID)))
	out = append(out, c.activeID...)
	out = append(out, nonce...)
	return aead.Seal(out, nonce, plaintext, context), nil
}

// Open decrypts a value Seal produced with any key of the ring for the same
// context.
func (c *Cipher) Open(sealed, context []byte) ([]byte, error) {
	id, rest, err := splitSealedField(sealed)
	if err != nil {
		return nil, err
	}
	aead, ok := c.keys[id]
	if !ok || len(rest) < aead.NonceSize()+aead.Overhead() {
		return nil, ErrOpen
	}
	nonce, ciphertext := rest[:aead.NonceSize()], rest[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, context)
	if err != nil {
		return nil, ErrOpen
	}
	return plaintext, nil
}

func splitSealedField(sealed []byte) (string, []byte, error) {
	if len(sealed) < 2 || sealed[0] != fieldCipherFormat {
		return "", nil, ErrOpen
	}
	idLength := int(sealed[1])
	if idLength == 0 || len(sealed) < 2+idLength {
		return "", nil, ErrOpen
	}
	return string(sealed[2 : 2+idLength]), sealed[2+idLength:], nil
}
