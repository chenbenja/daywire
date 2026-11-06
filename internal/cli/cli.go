package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/chenbenja/daywire/internal/buildinfo"
	"github.com/chenbenja/daywire/internal/config"
	"github.com/chenbenja/daywire/internal/doctor"
	"github.com/chenbenja/daywire/internal/nudge"
	"github.com/chenbenja/daywire/internal/seed"
	"github.com/chenbenja/daywire/internal/server"
	"github.com/chenbenja/daywire/internal/service"
	"github.com/chenbenja/daywire/internal/store"
)

// NewLogger returns the process logger. DAYWIRE_LOG_LEVEL may be debug, info, warn, or error.
func NewLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: logLevel()}))
}

// Run executes the daywire CLI. args[0] is the program name.
func Run(ctx context.Context, args []string, logger *slog.Logger) error {
	if logger == nil {
		logger = NewLogger(os.Stderr)
	}
	cmd := &cli.Command{
		Name:    "daywire",
		Usage:   "localhost life-context bridge",
		Version: buildinfo.Version,
		ExitErrHandler: func(context.Context, *cli.Command, error) {
		},
		Commands: []*cli.Command{
			{
				Name:  "serve",
				Usage: "run the loopback MCP server",
				Flags: configFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return serve(ctx, cmd, logger)
				},
			},
			{
				Name:  "doctor",
				Usage: "check bind address, database, MCP, and ntfy config",
				Flags: configFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runDoctor(ctx, cmd, logger)
				},
			},
			{
				Name:  "seed",
				Usage: "insert synthetic redacted events",
				Flags: configFlags(),
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return runSeed(ctx, cmd, logger)
				},
			},
			{
				Name:  "version",
				Usage: "print the version",
				Action: func(context.Context, *cli.Command) error {
					logger.Info("version", "version", buildinfo.Version)
					return nil
				},
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return cli.DefaultShowRootCommandHelp(cmd)
		},
	}
	return cmd.Run(ctx, args)
}

func serve(ctx context.Context, cmd *cli.Command, logger *slog.Logger) error {
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	if err := config.ValidateLoopback(cfg.Addr); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	st, err := store.Open(ctx, cfg.DBPath(), cfg.DataKey, logger)
	if err != nil {
		return err
	}
	defer st.Close()
	svc := service.New(st, nudge.New(cfg.NtfyURL, cfg.NtfyToken, logger), logger)
	logger.InfoContext(ctx, "starting", "addr", cfg.Addr, "key_source", cfg.KeySource, "ntfy", cfg.NtfyURL != "")
	return server.New(cfg.Addr, svc, logger).Listen(ctx)
}

func runDoctor(ctx context.Context, cmd *cli.Command, logger *slog.Logger) error {
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	report, err := doctor.Run(ctx, cfg, logger)
	if err != nil {
		return err
	}
	report.Log(logger)
	if !report.OK() {
		return errors.New("doctor found failing checks")
	}
	return nil
}

func runSeed(ctx context.Context, cmd *cli.Command, logger *slog.Logger) error {
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, cfg.DBPath(), cfg.DataKey, logger)
	if err != nil {
		return err
	}
	defer st.Close()
	n, err := seed.Apply(ctx, st, time.Now().UTC())
	if err != nil {
		return err
	}
	logger.InfoContext(ctx, "seed", "inserted", n)
	return nil
}

func loadConfig(cmd *cli.Command) (config.Config, error) {
	cfg, err := config.Load(config.Options{
		Addr:    cmd.String("addr"),
		DataDir: cmd.String("data-dir"),
	})
	if err != nil {
		return config.Config{}, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

func configFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "addr",
			Usage:   "loopback listen address (env DAYWIRE_ADDR)",
			Sources: cli.EnvVars("DAYWIRE_ADDR"),
		},
		&cli.StringFlag{
			Name:    "data-dir",
			Usage:   "data directory (env DAYWIRE_DATA_DIR, default $XDG_CONFIG_HOME/daywire)",
			Sources: cli.EnvVars("DAYWIRE_DATA_DIR"),
		},
	}
}

func logLevel() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DAYWIRE_LOG_LEVEL"))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
