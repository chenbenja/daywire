package speaker

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/testutil"
)

func TestEnsureFileChecksumAndCache(t *testing.T) {
	body := []byte("pinned-model-bytes")
	sum := sha256.Sum256(body)
	good := hex.EncodeToString(sum[:])
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	ctx := testutil.Context(t)
	dir := t.TempDir()
	art := artifact{File: "m.bin", URL: srv.URL + "/m.bin", SHA256: good, MaxBytes: 100}
	path, err := ensureFile(ctx, dir, art, testutil.Logger(t))
	require.NoError(t, err)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, body, got)
	require.Equal(t, 1, hits)

	srv.Close()
	path, err = ensureFile(ctx, dir, art, testutil.Logger(t))
	require.NoError(t, err)
	require.NotEmpty(t, path)
}

func TestEnsureFileChecksumMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("actual"))
	}))
	t.Cleanup(srv.Close)
	_, err := ensureFile(testutil.Context(t), t.TempDir(), artifact{
		File: "m.bin", URL: srv.URL, SHA256: "deadbeef", MaxBytes: 100,
	}, testutil.Logger(t))
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrModelsUnavailable))
}

func TestEnsureFileOffline(t *testing.T) {
	_, err := ensureFile(testutil.Context(t), t.TempDir(), artifact{
		File: "m.bin", URL: "http://127.0.0.1:1/m.bin", SHA256: "abc", MaxBytes: 100,
	}, testutil.Logger(t))
	require.ErrorIs(t, err, ErrModelsUnavailable)
}
