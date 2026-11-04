package fieldcipher

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func testFieldKey(fill byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{fill}, 32))
}

func TestFieldCipherSealsAndOpensWithItsContext(t *testing.T) {
	t.Parallel()
	cipher, err := Parse("k1:" + testFieldKey(1))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sealed, err := cipher.Seal([]byte("Erdnussallergie"), []byte("context-a"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if bytes.Contains(sealed, []byte("Erdnuss")) {
		t.Fatal("the sealed value contains the plaintext")
	}
	again, err := cipher.Seal([]byte("Erdnussallergie"), []byte("context-a"))
	if err != nil {
		t.Fatalf("seal again: %v", err)
	}
	if bytes.Equal(sealed, again) {
		t.Fatal("two seals of the same value are equal: the nonce repeats")
	}
	opened, err := cipher.Open(sealed, []byte("context-a"))
	if err != nil || string(opened) != "Erdnussallergie" {
		t.Fatalf("open = %q, %v", opened, err)
	}
	if _, err := cipher.Open(sealed, []byte("context-b")); err == nil {
		t.Fatal("a value sealed for one context opens in another")
	}
	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 1
	if _, err := cipher.Open(tampered, []byte("context-a")); err == nil {
		t.Fatal("a tampered value opens")
	}
}

func TestFieldCipherRotatesKeysByTheKeyIDInTheValue(t *testing.T) {
	t.Parallel()
	old, err := Parse("old:" + testFieldKey(1))
	if err != nil {
		t.Fatalf("parse old: %v", err)
	}
	sealedOld, err := old.Seal([]byte("Asthma"), nil)
	if err != nil {
		t.Fatalf("seal old: %v", err)
	}
	rotated, err := Parse("new:" + testFieldKey(2) + ", old:" + testFieldKey(1))
	if err != nil {
		t.Fatalf("parse rotated: %v", err)
	}
	if opened, err := rotated.Open(sealedOld, nil); err != nil || string(opened) != "Asthma" {
		t.Fatalf("open an old value after rotation = %q, %v", opened, err)
	}
	sealedNew, err := rotated.Seal([]byte("Asthma"), nil)
	if err != nil {
		t.Fatalf("seal new: %v", err)
	}
	if _, err := old.Open(sealedNew, nil); err == nil {
		t.Fatal("a value sealed with the new key opens with the old key only")
	}
	if id, _, err := splitSealedField(sealedNew); err != nil || id != "new" {
		t.Fatalf("key id of a new value = %q, %v", id, err)
	}
}

func TestParseFieldCipherRefusesUnusableKeys(t *testing.T) {
	t.Parallel()
	for name, spec := range map[string]string{
		"empty":          "",
		"no id":          testFieldKey(1),
		"short key":      "k1:" + base64.StdEncoding.EncodeToString([]byte("too short")),
		"not base64":     "k1:not*base64",
		"duplicate id":   "k1:" + testFieldKey(1) + ",k1:" + testFieldKey(2),
		"invalid id":     "k 1:" + testFieldKey(1),
		"empty entry":    "k1:" + testFieldKey(1) + ",",
		"overlong id":    strings.Repeat("k", 33) + ":" + testFieldKey(1),
		"placeholder id": ":" + testFieldKey(1),
	} {
		if _, err := Parse(spec); err == nil {
			t.Errorf("%s: parse succeeded", name)
		} else if strings.Contains(err.Error(), testFieldKey(1)) {
			t.Errorf("%s: the error names the key material", name)
		}
	}
}

func TestFieldCipherRefusesAValueOfAnUnknownKey(t *testing.T) {
	t.Parallel()
	one, _ := Parse("one:" + testFieldKey(1))
	two, _ := Parse("two:" + testFieldKey(2))
	sealed, err := one.Seal([]byte("x"), nil)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := two.Open(sealed, nil); err == nil {
		t.Fatal("a value of an unknown key opens")
	}
	if _, err := two.Open([]byte{9, 9}, nil); err == nil {
		t.Fatal("garbage opens")
	}
}
