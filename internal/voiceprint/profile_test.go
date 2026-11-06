package voiceprint

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

func testKey(t *testing.T) []byte {
	t.Helper()
	key, err := crypt.GenerateKey()
	require.NoError(t, err)
	return key
}

func TestProfileRoundTripAndForget(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	key := testKey(t)
	dir := t.TempDir()
	vec := []float32{0.25, -0.5, 1, 0}
	require.NoError(t, Save(ctx, dir, key, OwnerID, vec))

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

	got, err := Load(ctx, dir, key, OwnerID)
	require.NoError(t, err)
	require.Equal(t, vec, got.Vec)

	enrolled, err := Exists(ctx, dir, OwnerID)
	require.NoError(t, err)
	require.True(t, enrolled)

	require.NoError(t, Forget(ctx, dir, OwnerID))
	enrolled, err = Exists(ctx, dir, OwnerID)
	require.NoError(t, err)
	require.False(t, enrolled)
	require.NoError(t, Forget(ctx, dir, OwnerID))
	_, err = Load(ctx, dir, key, OwnerID)
	require.ErrorIs(t, err, ErrNotEnrolled)
}

func TestProfileAADDoesNotMatchEventColumns(t *testing.T) {
	t.Parallel()
	got := ProfileAAD("owner")
	require.Equal(t, crypt.AAD("owner", "speaker.embedding"), got)
	for _, column := range []string{"summary", "title", "message"} {
		require.NotEqual(t, crypt.AAD("owner", column), got)
	}
}
