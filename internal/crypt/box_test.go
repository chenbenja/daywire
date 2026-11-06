package crypt_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/crypt"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	key, err := crypt.GenerateKey()
	require.NoError(t, err)
	box, err := crypt.NewBox(key)
	require.NoError(t, err)

	plain := []byte("Calendar: send the weekly status note on Friday.")
	aad := crypt.AAD("seed-commitment-1", "summary")
	nonce, ct, err := box.Seal(aad, plain)
	require.NoError(t, err)
	require.NotContains(t, string(ct), "weekly status")

	got, err := box.Open(aad, nonce, ct)
	require.NoError(t, err)
	require.Equal(t, plain, got)
}

func TestRejectsWrongKeyAADAndTamper(t *testing.T) {
	t.Parallel()
	plain := []byte("Mail: confirm a meeting place for Saturday.")
	aad := crypt.AAD("seed-ask-1", "summary")

	key, err := crypt.GenerateKey()
	require.NoError(t, err)
	box, err := crypt.NewBox(key)
	require.NoError(t, err)
	nonce, ct, err := box.Seal(aad, plain)
	require.NoError(t, err)

	otherKey, err := crypt.GenerateKey()
	require.NoError(t, err)
	other, err := crypt.NewBox(otherKey)
	require.NoError(t, err)

	_, err = other.Open(aad, nonce, ct)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "Saturday")

	_, err = box.Open(crypt.AAD("other-id", "summary"), nonce, ct)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "Saturday")

	tampered := bytes.Clone(ct)
	tampered[len(tampered)-1] ^= 0x01
	_, err = box.Open(aad, nonce, tampered)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "Saturday")
}

func TestNewBoxRejectsBadKey(t *testing.T) {
	t.Parallel()
	_, err := crypt.NewBox([]byte("short"))
	require.Error(t, err)
	require.NotContains(t, err.Error(), "short")
}
