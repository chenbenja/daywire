package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/chenbenja/daywire/internal/buildinfo"
	"github.com/chenbenja/daywire/internal/config"
	"github.com/chenbenja/daywire/internal/service"
)

// ErrNotRunning is returned by Probe when nothing is accepting connections.
var ErrNotRunning = errors.New("mcp server not running")

// Server is the loopback HTTP and MCP server.
type Server struct {
	addr    string
	handler http.Handler
	log     *slog.Logger
}

// New builds a server. It does not listen.
func New(addr string, svc *service.Service, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	mcpServer := NewMCP(svc, logger)
	return &Server{
		addr:    addr,
		handler: newHTTP(addr, svc, mcpServer, logger),
		log:     logger,
	}
}

// Handler returns the HTTP handler. The listener must still be loopback.
func (s *Server) Handler() http.Handler {
	return s.handler
}

// Listen binds addr and serves until ctx is cancelled. addr must be loopback.
func (s *Server) Listen(ctx context.Context) error {
	if err := config.ValidateLoopback(s.addr); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.log.InfoContext(ctx, "listening", "addr", ln.Addr().String(), "mcp", "http://"+ln.Addr().String()+"/mcp")
	return s.Serve(ctx, ln)
}

// Serve accepts connections on ln until ctx is cancelled.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	hs := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      40 * time.Second,
		IdleTimeout:       2 * time.Minute,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	errCh := make(chan error, 1)
	go func() {
		err := hs.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr := hs.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			// A client can hold a streamable HTTP connection past the graceful
			// window. Close drops it so the process can exit.
			s.log.Info("closing listener after shutdown", "err", shutdownErr.Error())
			_ = hs.Close()
		}
		if err := <-errCh; err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	}
}

// NewMCP returns the MCP server with the Week 1 tools.
func NewMCP(svc *service.Service, logger *slog.Logger) *mcp.Server {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	m := mcp.NewServer(&mcp.Implementation{
		Name:    "daywire",
		Title:   "Daywire",
		Version: buildinfo.Version,
	}, &mcp.ServerOptions{
		Instructions: "Daywire returns redacted semantic events only. There is no call audio, no raw message body, and no speech transcript. Use query_events, subscribe (poll), and propose_nudge.",
		Logger:       logger,
	})
	h := handlers{svc: svc}
	mcp.AddTool(m, &mcp.Tool{
		Name:        "query_events",
		Description: "List redacted semantic events by time range and type.",
	}, h.query)
	mcp.AddTool(m, &mcp.Tool{
		Name:        "subscribe",
		Description: "Poll for redacted events created after a cursor. wait_ms long-polls up to 25 seconds. This is not a push channel.",
	}, h.subscribe)
	mcp.AddTool(m, &mcp.Tool{
		Name:        "propose_nudge",
		Description: "Deliver a redacted nudge through ntfy, or the log fallback when ntfy is unset. Returns delivery status.",
	}, h.propose)
	return m
}

type handlers struct {
	svc *service.Service
}

func (h handlers) query(ctx context.Context, _ *mcp.CallToolRequest, in service.QueryInput) (*mcp.CallToolResult, service.QueryOutput, error) {
	out, err := h.svc.Query(ctx, in)
	return nil, out, err
}

func (h handlers) subscribe(ctx context.Context, _ *mcp.CallToolRequest, in service.SubscribeInput) (*mcp.CallToolResult, service.SubscribeOutput, error) {
	out, err := h.svc.Subscribe(ctx, in)
	return nil, out, err
}

func (h handlers) propose(ctx context.Context, _ *mcp.CallToolRequest, in service.ProposeInput) (*mcp.CallToolResult, service.ProposeOutput, error) {
	out, err := h.svc.Propose(ctx, in)
	return nil, out, err
}

// CheckInProcess lists tools and calls query_events on an in-memory session.
func CheckInProcess(ctx context.Context, svc *service.Service) error {
	m := NewMCP(svc, nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ss, err := m.Connect(ctx, serverTransport, nil)
	if err != nil {
		return fmt.Errorf("connect mcp server: %w", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "daywire-doctor", Version: buildinfo.Version}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		return fmt.Errorf("connect mcp client: %w", err)
	}
	defer func() {
		_ = cs.Close()
		ss.Wait()
	}()
	listed, err := cs.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("list tools: %w", err)
	}
	have := map[string]bool{}
	for _, tool := range listed.Tools {
		have[tool.Name] = true
	}
	for _, name := range []string{"query_events", "subscribe", "propose_nudge"} {
		if !have[name] {
			return fmt.Errorf("mcp tool %s missing", name)
		}
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "query_events",
		Arguments: map[string]any{"limit": 1},
	})
	if err != nil {
		return fmt.Errorf("query_events: %w", err)
	}
	if res != nil && res.IsError {
		return fmt.Errorf("query_events failed")
	}
	return nil
}

// Probe dials the live MCP endpoint at addr. Connection refusal returns ErrNotRunning.
func Probe(ctx context.Context, addr string) error {
	if err := config.ValidateLoopback(addr); err != nil {
		return err
	}
	endpoint := (&url.URL{Scheme: "http", Host: addr, Path: "/mcp"}).String()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "daywire-probe", Version: buildinfo.Version}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
		HTTPClient:           &http.Client{Timeout: 3 * time.Second},
	}, nil)
	if err != nil {
		if connRefused(err) {
			return ErrNotRunning
		}
		return fmt.Errorf("probe mcp: %w", err)
	}
	defer func() { _ = session.Close() }()
	if _, err := session.ListTools(ctx, nil); err != nil {
		return fmt.Errorf("probe list tools: %w", err)
	}
	return nil
}

func connRefused(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var op *net.OpError
	if errors.As(err, &op) && errors.Is(op.Err, syscall.ECONNREFUSED) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") || strings.Contains(msg, "actively refused")
}
