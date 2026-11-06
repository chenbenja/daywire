package doctor

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"

	"github.com/chenbenja/daywire/internal/config"
	"github.com/chenbenja/daywire/internal/nudge"
	"github.com/chenbenja/daywire/internal/server"
	"github.com/chenbenja/daywire/internal/service"
	"github.com/chenbenja/daywire/internal/store"
)

const (
	// StatusPass means the check succeeded.
	StatusPass = "pass"
	// StatusFail means the check failed.
	StatusFail = "fail"
	// StatusSkip means the check was not applicable.
	StatusSkip = "skip"
)

// Check is one doctor result. Detail is safe to print.
type Check struct {
	Name   string
	Status string
	Detail string
}

// Report is the full doctor result.
type Report struct {
	Checks []Check
}

// OK reports whether every check passed or was skipped.
func (r Report) OK() bool {
	for _, check := range r.Checks {
		if check.Status == StatusFail {
			return false
		}
	}
	return true
}

// Log writes the report with the injected logger. It does not print secrets.
func (r Report) Log(logger *slog.Logger) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	for _, check := range r.Checks {
		logger.Info("doctor", "check", check.Name, "status", check.Status, "detail", check.Detail)
	}
	logger.Info("doctor", "ok", r.OK())
}

// Run checks the bind address, the database, the MCP server, and ntfy config.
//
// The MCP check always exercises an in-process session. A live server that is
// not running is reported as a pass with that fact in the detail, because
// doctor is useful before `daywire serve`. A live server that accepts a
// connection but does not speak MCP fails the check.
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) (Report, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	checks := []Check{checkBind(cfg), checkNtfy(cfg)}

	st, err := store.Open(ctx, cfg.DBPath(), cfg.DataKey, logger)
	if err != nil {
		logger.ErrorContext(ctx, "doctor database", "err", err)
		checks = append(checks,
			Check{Name: "database", Status: StatusFail, Detail: "database did not open"},
			Check{Name: "mcp", Status: StatusSkip, Detail: "skipped because the database did not open"},
		)
		return Report{Checks: checks}, nil
	}
	defer st.Close()
	if err := st.Ping(ctx); err != nil {
		logger.ErrorContext(ctx, "doctor database", "err", err)
		checks = append(checks,
			Check{Name: "database", Status: StatusFail, Detail: "database ping failed"},
			Check{Name: "mcp", Status: StatusSkip, Detail: "skipped because the database ping failed"},
		)
		return Report{Checks: checks}, nil
	}
	checks = append(checks, Check{Name: "database", Status: StatusPass, Detail: "opened"})

	svc := service.New(st, nudge.New(cfg.NtfyURL, cfg.NtfyToken, logger), logger)
	checks = append(checks, checkMCP(ctx, cfg, svc, logger))
	return Report{Checks: checks}, nil
}

func checkBind(cfg config.Config) Check {
	if err := config.ValidateLoopback(cfg.Addr); err != nil {
		detail := "listen address is not loopback"
		if cfg.Addr != "" && len(cfg.Addr) < 128 {
			detail = cfg.Addr + " is not loopback"
		}
		return Check{Name: "bind", Status: StatusFail, Detail: detail}
	}
	return Check{Name: "bind", Status: StatusPass, Detail: cfg.Addr}
}

func checkNtfy(cfg config.Config) Check {
	raw := strings.TrimSpace(cfg.NtfyURL)
	if raw == "" {
		return Check{Name: "ntfy", Status: StatusSkip, Detail: "DAYWIRE_NTFY_URL unset; nudges log locally"}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return Check{Name: "ntfy", Status: StatusFail, Detail: "DAYWIRE_NTFY_URL must be an absolute http(s) URL"}
	}
	detail := "configured host " + u.Host
	if strings.TrimSpace(cfg.NtfyToken) != "" {
		detail += "; bearer token set"
	}
	return Check{Name: "ntfy", Status: StatusPass, Detail: detail}
}

func checkMCP(ctx context.Context, cfg config.Config, svc *service.Service, logger *slog.Logger) Check {
	if err := server.CheckInProcess(ctx, svc); err != nil {
		logger.ErrorContext(ctx, "doctor mcp", "err", err)
		return Check{Name: "mcp", Status: StatusFail, Detail: "in-process MCP failed"}
	}
	if err := config.ValidateLoopback(cfg.Addr); err != nil {
		return Check{Name: "mcp", Status: StatusPass, Detail: "in-process ok; live probe skipped because the bind address is not loopback"}
	}
	err := server.Probe(ctx, cfg.Addr)
	switch {
	case err == nil:
		return Check{Name: "mcp", Status: StatusPass, Detail: "live server responded"}
	case errors.Is(err, server.ErrNotRunning):
		return Check{Name: "mcp", Status: StatusPass, Detail: "in-process ok; live server not running"}
	default:
		logger.ErrorContext(ctx, "doctor mcp probe", "err", err)
		return Check{Name: "mcp", Status: StatusFail, Detail: "live MCP probe failed"}
	}
}
