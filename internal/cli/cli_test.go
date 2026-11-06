package cli_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/cli"
	"github.com/chenbenja/daywire/internal/config"
	"github.com/chenbenja/daywire/internal/store"
	"github.com/chenbenja/daywire/internal/testutil"
)

func TestSeedAndDoctor(t *testing.T) {
	testutil.ClearEnv(t)
	dir := t.TempDir()
	logger := testutil.Logger(t)
	ctx := testutil.Context(t)
	args := []string{"daywire", "seed", "--data-dir", dir}
	require.NoError(t, cli.Run(ctx, args, logger))
	require.NoError(t, cli.Run(ctx, args, logger))

	cfg, err := config.Load(config.Options{DataDir: dir, Addr: "127.0.0.1:9"})
	require.NoError(t, err)
	st := testutil.OpenStore(t, cfg)
	rows, err := st.ListByOccurred(ctx, store.OccurredFilter{Limit: 20})
	require.NoError(t, err)
	require.Len(t, rows, 4)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	err = cli.Run(ctx, []string{"daywire", "doctor", "--data-dir", dir, "--addr", addr}, logger)
	require.NoError(t, err)
}

func TestServeRejectsPublicBind(t *testing.T) {
	testutil.ClearEnv(t)
	err := cli.Run(testutil.Context(t), []string{
		"daywire", "serve", "--data-dir", t.TempDir(), "--addr", "0.0.0.0:8765",
	}, testutil.Logger(t))
	require.Error(t, err)
	require.NotContains(t, err.Error(), "summary")
}
