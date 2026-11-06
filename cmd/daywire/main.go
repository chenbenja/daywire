// Command daywire is the Week 1 localhost life-context bridge.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/chenbenja/daywire/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := cli.NewLogger(os.Stderr)
	if err := cli.Run(ctx, os.Args, logger); err != nil {
		logger.Error("command failed", "err", err.Error())
		os.Exit(1)
	}
}
