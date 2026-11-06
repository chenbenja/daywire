//go:build !cgo

package speaker

import "log/slog"

const sherpaAvailable = false

func newSherpaEngine(Models, *slog.Logger) (Engine, error) {
	return nil, ErrEngineUnavailable
}
