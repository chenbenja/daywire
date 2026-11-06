package testutil

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/config"
	"github.com/chenbenja/daywire/internal/store"
)

// Context returns a context that cancels when the test ends.
func Context(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// Logger returns a logger that discards output.
func Logger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ClearEnv blanks Daywire variables so a test does not inherit the host environment.
//
// Callers cannot use t.Parallel together with this helper.
func ClearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DAYWIRE_ADDR",
		"DAYWIRE_DATA_DIR",
		"DAYWIRE_NTFY_URL",
		"DAYWIRE_NTFY_TOKEN",
		"DAYWIRE_DATA_KEY",
		"DAYWIRE_DATA_KEY_FILE",
		"DAYWIRE_LOG_LEVEL",
		"XDG_CONFIG_HOME",
	} {
		t.Setenv(key, "")
	}
}

// LoadConfig returns a config rooted at a temp directory with a generated key.
func LoadConfig(t *testing.T) config.Config {
	t.Helper()
	ClearEnv(t)
	cfg, err := config.Load(config.Options{
		DataDir: t.TempDir(),
		Addr:    "127.0.0.1:8765",
	})
	require.NoError(t, err)
	return cfg
}

// OpenStore opens the database for cfg and closes it when the test ends.
func OpenStore(t *testing.T, cfg config.Config) *store.Store {
	t.Helper()
	st, err := store.Open(Context(t), cfg.DBPath(), cfg.DataKey, Logger(t))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})
	return st
}
