package speaker

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/crypt"
	"github.com/chenbenja/daywire/internal/testutil"
)

func TestProfileRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	key := testKey(t)
	dir := t.TempDir()
	vec := []float32{0.25, -0.5, 1, 0}
	require.NoError(t, SaveProfile(ctx, dir, key, "owner", vec))

	path := filepath.Join(dir, "speakers", "owner.emb")
	st, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	dirInfo, err := os.Stat(filepath.Join(dir, "speakers"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

	body, err := os.ReadFile(path)
	require.NoError(t, err)
	raw := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(v))
	}
	require.False(t, bytes.Contains(body, raw))
	require.NotContains(t, string(body), "RIFF")

	got, err := LoadProfile(ctx, dir, key, "owner")
	require.NoError(t, err)
	require.Equal(t, vec, got.Vec)

	other := testKey(t)
	_, err = LoadProfile(ctx, dir, other, "owner")
	require.Error(t, err)
	_, err = LoadProfile(ctx, dir, key, "other")
	require.ErrorIs(t, err, ErrNotEnrolled)
}

func TestProfileAADDoesNotMatchEventStore(t *testing.T) {
	t.Parallel()
	storeColumns := []string{"summary", "title", "message"}
	ids := []string{"owner", "seed-commitment-1", "speaker.owner"}
	for _, id := range ids {
		got := profileAAD(id)
		require.Equal(t, crypt.AAD(id, "speaker.embedding"), got)
		for _, column := range storeColumns {
			require.NotEqual(t, crypt.AAD(id, column), got)
		}
	}
}

func TestValidSpeakerID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id string
		ok bool
	}{
		{id: "owner", ok: true},
		{id: "a", ok: true},
		{id: "speaker_1", ok: true},
		{id: "", ok: false},
		{id: "Owner", ok: false},
		{id: "../x", ok: false},
		{id: "1abc", ok: false},
		{id: "has space", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.ok, ValidSpeakerID(tc.id))
		})
	}
}

func testKey(t *testing.T) []byte {
	t.Helper()
	key, err := crypt.GenerateKey()
	require.NoError(t, err)
	return key
}
