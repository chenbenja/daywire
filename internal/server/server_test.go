package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/chenbenja/daywire/internal/buildinfo"
	"github.com/chenbenja/daywire/internal/event"
	"github.com/chenbenja/daywire/internal/nudge"
	"github.com/chenbenja/daywire/internal/seed"
	"github.com/chenbenja/daywire/internal/server"
	"github.com/chenbenja/daywire/internal/service"
	"github.com/chenbenja/daywire/internal/testutil"
)

func TestRejectsNonLoopbackListenAndRemoteAddr(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	st := testutil.OpenStore(t, cfg)
	svc := service.New(st, nudge.New("", "", testutil.Logger(t)), testutil.Logger(t))
	srv := server.New("0.0.0.0:8765", svc, testutil.Logger(t))
	err := srv.Listen(testutil.Context(t))
	require.Error(t, err)
	require.NotContains(t, err.Error(), "summary")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "203.0.113.8:4000"
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	require.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRejectsRebindingHost(t *testing.T) {
	const marker = "REDACTED-SUMMARY-MARKER-rebinding"
	cfg := testutil.LoadConfig(t)
	ctx := testutil.Context(t)
	st := testutil.OpenStore(t, cfg)
	now := time.Date(2026, 11, 5, 16, 0, 0, 0, time.UTC)
	require.NoError(t, st.InsertEvent(ctx, event.Event{
		ID:         "rebinding-note",
		Type:       event.TypeNote,
		SourceID:   "synthetic:note",
		OccurredAt: now,
		CreatedAt:  now,
		Summary:    marker,
	}))

	var nudgeHits int
	ntfySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nudgeHits++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ntfySrv.Close)

	logger := testutil.Logger(t)
	svc := service.New(st, nudge.New(ntfySrv.URL+"/topic", "secret-token", logger), logger)
	srv := server.New("127.0.0.1:9", svc, logger)
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(httpSrv.Close)

	for _, path := range []string{"/health", "/events", "/events/poll", "/mcp"} {
		req, err := http.NewRequest(http.MethodGet, httpSrv.URL+path, nil)
		require.NoError(t, err)
		req.Host = "evil.example"
		res, err := httpSrv.Client().Do(req)
		require.NoError(t, err)
		body, err := io.ReadAll(res.Body)
		require.NoError(t, res.Body.Close())
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, res.StatusCode, path)
		require.NotContains(t, string(body), marker, path)
		require.NotContains(t, string(body), "secret-token", path)
	}

	nudgeReq, err := http.NewRequest(http.MethodPost, httpSrv.URL+"/nudges", strings.NewReader(`{"title":"Synthetic nudge","message":"Pack the spare charger."}`))
	require.NoError(t, err)
	nudgeReq.Host = "evil.example:8765"
	nudgeReq.Header.Set("Content-Type", "application/json")
	nudgeRes, err := httpSrv.Client().Do(nudgeReq)
	require.NoError(t, err)
	nudgeBody, err := io.ReadAll(nudgeRes.Body)
	require.NoError(t, nudgeRes.Body.Close())
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, nudgeRes.StatusCode)
	require.NotContains(t, string(nudgeBody), "Pack the spare charger.")
	require.Zero(t, nudgeHits)

	for _, host := range []string{"localhost", "localhost:8765", "127.0.0.1", "127.0.0.1:8765", "[::1]:8765"} {
		req, err := http.NewRequest(http.MethodGet, httpSrv.URL+"/health", nil)
		require.NoError(t, err)
		req.Host = host
		res, err := httpSrv.Client().Do(req)
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, res.Body)
		require.NoError(t, res.Body.Close())
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode, host)
	}
}

func TestHTTPAndMCP(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	ctx := testutil.Context(t)
	st := testutil.OpenStore(t, cfg)
	_, err := seed.Apply(ctx, st, time.Date(2026, 11, 5, 16, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	var posted string
	ntfySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, readErr := io.ReadAll(r.Body)
		require.NoError(t, readErr)
		posted = string(body)
		require.Equal(t, "Bearer secret-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ntfySrv.Close)

	logger := testutil.Logger(t)
	svc := service.New(st, nudge.New(ntfySrv.URL+"/topic", "secret-token", logger), logger)
	srv := server.New("127.0.0.1:9", svc, logger)
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(httpSrv.Close)

	healthRes, err := httpSrv.Client().Get(httpSrv.URL + "/health")
	require.NoError(t, err)
	defer healthRes.Body.Close()
	require.Equal(t, http.StatusOK, healthRes.StatusCode)

	eventsRes, err := httpSrv.Client().Get(httpSrv.URL + "/events?type=visit&limit=5")
	require.NoError(t, err)
	defer eventsRes.Body.Close()
	eventsBody, err := io.ReadAll(eventsRes.Body)
	require.NoError(t, err)
	require.Contains(t, string(eventsBody), "seed-visit-1")
	require.NotContains(t, string(eventsBody), "audio")
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(eventsBody, &decoded))
	rows, ok := decoded["events"].([]any)
	require.True(t, ok)
	require.Len(t, rows, 1)
	row := rows[0].(map[string]any)
	require.ElementsMatch(t, []string{"id", "type", "source_id", "occurred_at", "created_at", "summary"}, mapKeys(row))

	client := mcp.NewClient(&mcp.Implementation{Name: "daywire-test", Version: buildinfo.Version}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             httpSrv.URL + "/mcp",
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, session.Close()) })

	listed, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	names := make([]string, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
	}
	require.ElementsMatch(t, []string{"query_events", "subscribe", "propose_nudge"}, names)

	queryRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "query_events",
		Arguments: map[string]any{"type": "ask", "limit": 5},
	})
	require.NoError(t, err)
	require.False(t, queryRes.IsError)
	require.Contains(t, toolJSON(t, queryRes), "seed-ask-1")

	subRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "subscribe",
		Arguments: map[string]any{"limit": 10},
	})
	require.NoError(t, err)
	require.False(t, subRes.IsError)
	require.Contains(t, toolJSON(t, subRes), "poll")

	nudgeRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "propose_nudge",
		Arguments: map[string]any{
			"title":   "Synthetic nudge",
			"message": "Pack the spare charger.",
		},
	})
	require.NoError(t, err)
	require.False(t, nudgeRes.IsError)
	nudgeJSON := toolJSON(t, nudgeRes)
	require.Contains(t, nudgeJSON, "ntfy")
	require.Contains(t, nudgeJSON, "true")
	require.Equal(t, "Pack the spare charger.", posted)

	httpNudge, err := httpSrv.Client().Post(httpSrv.URL+"/nudges", "application/json", strings.NewReader(`{"title":"HTTP nudge","message":"Confirm the meeting place."}`))
	require.NoError(t, err)
	defer httpNudge.Body.Close()
	require.Equal(t, http.StatusOK, httpNudge.StatusCode)
	body, err := io.ReadAll(httpNudge.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `"delivered":true`)
	require.Equal(t, "Confirm the meeting place.", posted)
}

func TestServeShutdown(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	st := testutil.OpenStore(t, cfg)
	svc := service.New(st, nudge.New("", "", testutil.Logger(t)), testutil.Logger(t))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := server.New(ln.Addr().String(), svc, testutil.Logger(t))
	ctx, cancel := context.WithCancel(testutil.Context(t))
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ctx, ln)
	}()
	require.Eventually(t, func() bool {
		res, err := http.Get("http://" + ln.Addr().String() + "/health")
		if err != nil {
			return false
		}
		defer res.Body.Close()
		return res.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond)
	cancel()
	require.NoError(t, <-errCh)
}

func TestProbeNotRunning(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	err = server.Probe(testutil.Context(t), addr)
	require.ErrorIs(t, err, server.ErrNotRunning)
}

func toolJSON(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res.StructuredContent != nil {
		b, err := json.Marshal(res.StructuredContent)
		require.NoError(t, err)
		return string(b)
	}
	var b strings.Builder
	for _, content := range res.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
