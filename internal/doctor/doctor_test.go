package doctor_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/doctor"
	"github.com/chenbenja/daywire/internal/nudge"
	"github.com/chenbenja/daywire/internal/server"
	"github.com/chenbenja/daywire/internal/service"
	"github.com/chenbenja/daywire/internal/store"
	"github.com/chenbenja/daywire/internal/testutil"
)

func TestDoctorWithoutServer(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	cfg.Addr = ln.Addr().String()
	require.NoError(t, ln.Close())

	report, err := doctor.Run(testutil.Context(t), cfg, testutil.Logger(t))
	require.NoError(t, err)
	require.True(t, report.OK())
	require.Equal(t, "pass", status(report, "bind"))
	require.Equal(t, "pass", status(report, "database"))
	require.Equal(t, "skip", status(report, "ntfy"))
	require.Equal(t, "pass", status(report, "mcp"))
	require.Contains(t, detail(report, "mcp"), "live server not running")
}

func TestDoctorLiveServer(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	ctx := testutil.Context(t)
	logger := testutil.Logger(t)
	st, err := store.Open(ctx, cfg.DBPath(), cfg.DataKey, logger)
	require.NoError(t, err)
	svc := service.New(st, nudge.New("", "", logger), logger)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	cfg.Addr = ln.Addr().String()
	srv := server.New(cfg.Addr, svc, logger)
	serveCtx, cancel := context.WithCancel(ctx)
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(serveCtx, ln)
	}()

	require.Eventually(t, func() bool {
		return server.Probe(ctx, cfg.Addr) == nil
	}, 5*time.Second, 30*time.Millisecond)

	report, err := doctor.Run(ctx, cfg, logger)
	require.NoError(t, err)
	require.True(t, report.OK())
	require.Equal(t, "live server responded", detail(report, "mcp"))

	cancel()
	require.NoError(t, <-errCh)
	require.NoError(t, st.Close())
}

func TestDoctorFailuresDoNotEchoSecrets(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	cfg.Addr = "0.0.0.0:8765"
	const token = "super-secret-token"
	const topic = "topic-secret-value"
	cfg.NtfyURL = "https://ntfy.example/" + topic
	cfg.NtfyToken = token

	report, err := doctor.Run(testutil.Context(t), cfg, testutil.Logger(t))
	require.NoError(t, err)
	require.False(t, report.OK())
	require.Equal(t, "fail", status(report, "bind"))
	require.Equal(t, "pass", status(report, "ntfy"))
	require.Contains(t, detail(report, "ntfy"), "ntfy.example")
	require.Contains(t, detail(report, "ntfy"), "bearer token set")
	for _, check := range report.Checks {
		require.NotContains(t, check.Detail, token)
		require.NotContains(t, check.Detail, topic)
	}
}

func TestDoctorRejectsBadNtfyURL(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	cfg.NtfyURL = "ftp://ntfy.example/topic"
	report, err := doctor.Run(testutil.Context(t), cfg, testutil.Logger(t))
	require.NoError(t, err)
	require.False(t, report.OK())
	require.Equal(t, "fail", status(report, "ntfy"))
	require.NotContains(t, detail(report, "ntfy"), "ftp://")
}

func status(report doctor.Report, name string) string {
	for _, check := range report.Checks {
		if check.Name == name {
			return check.Status
		}
	}
	return ""
}

func detail(report doctor.Report, name string) string {
	for _, check := range report.Checks {
		if check.Name == name {
			return check.Detail
		}
	}
	return ""
}
