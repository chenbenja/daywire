package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

// KeySize is the AES-256 key length in bytes.
const KeySize = 32

// Box seals and opens small byte slices.
//
// A Box holds an AEAD, not a copy of application payloads. Callers pass
// additional authenticated data (typically an id and a column name) so a
// ciphertext cannot be moved to another row unnoticed.
type Box struct {
	aead cipher.AEAD
}

// NewBox returns a Box for key. key must be KeySize bytes.
func NewBox(key []byte) (*Box, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("data key length is %d, want %d", len(key), KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &Box{aead: aead}, nil
}

// GenerateKey returns a fresh random KeySize key.
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate data key: %w", err)
	}
	return key, nil
}

// Seal encrypts plaintext under aad. The nonce is random and returned separately.
func (b *Box) Seal(aad, plaintext []byte) (nonce, ciphertext []byte, err error) {
	nonce = make([]byte, b.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext = b.aead.Seal(nil, nonce, plaintext, aad)
	return nonce, ciphertext, nil
}

// Open decrypts ciphertext. It returns an error if the nonce, tag, or aad
// does not match. The error does not include plaintext.
func (b *Box) Open(aad, nonce, ciphertext []byte) ([]byte, error) {
	if len(nonce) != b.aead.NonceSize() {
		return nil, fmt.Errorf("decrypt: invalid nonce length")
	}
	out, err := b.aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return out, nil
}

// AAD binds a ciphertext to an identifier and a column name.
func AAD(id, column string) []byte {
	return []byte(id + "\x00" + column)
}
