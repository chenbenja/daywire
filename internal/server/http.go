package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/chenbenja/daywire/internal/service"
)

func newHTTP(addr string, svc *service.Service, mcpServer *mcp.Server, logger *slog.Logger) http.Handler {
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return mcpServer
	}, &mcp.StreamableHTTPOptions{
		Logger:         logger,
		SessionTimeout: 30 * time.Minute,
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "addr": addr})
	})
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		limit, err := optionalInt(r.URL.Query().Get("limit"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		out, err := svc.Query(r.Context(), service.QueryInput{
			Type:  r.URL.Query().Get("type"),
			Since: r.URL.Query().Get("since"),
			Until: r.URL.Query().Get("until"),
			Limit: limit,
		})
		if err != nil {
			status, msg := clientError(err)
			logger.ErrorContext(r.Context(), "query events", "status", status)
			writeError(w, status, msg)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("GET /events/poll", func(w http.ResponseWriter, r *http.Request) {
		limit, err := optionalInt(r.URL.Query().Get("limit"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		wait, err := optionalInt(r.URL.Query().Get("wait_ms"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid wait_ms")
			return
		}
		out, err := svc.Subscribe(r.Context(), service.SubscribeInput{
			Cursor: r.URL.Query().Get("cursor"),
			Limit:  limit,
			WaitMS: wait,
		})
		if err != nil {
			status, msg := clientError(err)
			logger.ErrorContext(r.Context(), "poll events", "status", status)
			writeError(w, status, msg)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /nudges", func(w http.ResponseWriter, r *http.Request) {
		var in service.ProposeInput
		if err := readJSON(w, r, &in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		out, err := svc.Propose(r.Context(), in)
		if err != nil {
			status, msg := clientError(err)
			logger.ErrorContext(r.Context(), "propose nudge", "status", status)
			writeError(w, status, msg)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	mux.Handle("/mcp", mcpHandler)
	return loopbackOnly(logRequests(logger, mux))
}

// loopbackOnly allows a request only when the TCP peer is loopback and the
// Host header is localhost or a loopback address. A browser DNS-rebinding
// attack connects to 127.0.0.1 with the attacker's domain in Host; rejecting
// that header keeps event summaries and nudges on the machine.
func loopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !remoteLoopback(r.RemoteAddr) || !hostLoopback(r.Host) {
			http.Error(w, "loopback only", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func remoteLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func hostLoopback(hostport string) bool {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return false
	}
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = strings.Trim(hostport, "[]")
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		logger.InfoContext(r.Context(), "http", "method", r.Method, "path", r.URL.Path, "status", sw.status)
	})
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid json")
	}
	if dec.More() {
		return fmt.Errorf("invalid json")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func optionalInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func clientError(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "invalid event type"),
		strings.Contains(msg, "invalid since"),
		strings.Contains(msg, "invalid until"),
		strings.Contains(msg, "until is before since"),
		strings.Contains(msg, "invalid limit"),
		strings.Contains(msg, "invalid cursor"),
		strings.Contains(msg, "invalid wait_ms"),
		strings.Contains(msg, "wait_ms exceeds"),
		strings.Contains(msg, "nudge title"),
		strings.Contains(msg, "nudge message"),
		strings.Contains(msg, "nudge priority"),
		msg == "invalid json":
		return http.StatusBadRequest, msg
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return http.StatusRequestTimeout, "cancelled"
	default:
		if strings.Contains(msg, "context canceled") || strings.Contains(msg, "context deadline exceeded") {
			return http.StatusRequestTimeout, "cancelled"
		}
		return http.StatusInternalServerError, "request failed"
	}
}
