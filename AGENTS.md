# AGENTS.md

Daywire is a phone-first life-context bridge. Week 1 in this repository is a localhost Go process: redacted events, an encrypted SQLite store, a loopback MCP server, ntfy nudges, and `daywire doctor`.

These conventions follow the shape of Chen's private Go services (in particular the sanit-internal layout and review habits). They are adapted for Daywire. Do not copy Sanit product code into this repo.

## License

The intended licence for the hub and app core is AGPL, with a permissive adapter SDK later. That is intent only. Do not add a `LICENSE` file, AGPL text, or copyright headers until the founder adds them. Keep the README line **License: TBD**.

## Layout

- `cmd/daywire/` is a thin `main`. Flag parsing and subcommands live in `internal/cli`.
- Product code lives under `internal/`. One concern per package: `event`, `crypt`, `config`, `store`, `nudge`, `seed`, `service`, `server`, `doctor`.
- `hack/make.sh` is the build entry. The Makefile only calls it.
- Binaries go to `output/`. Do not commit `output/`.

## Go

- Module path: `github.com/chenbenja/daywire`.
- `go` directive: 1.26.
- Inject `*slog.Logger`. Do not use `fmt.Println` or package-level loggers.
- `context.Context` is the first argument of any function that does I/O.
- The long-running server stops on `signal.NotifyContext` (SIGINT, SIGTERM) and then `http.Server.Shutdown`.
- Wrap errors with `%w`. Do not put event summaries, nudge bodies, data keys, ntfy tokens, or ntfy topic paths in error strings or ordinary logs.
- The log nudge channel is the exception: when `DAYWIRE_NTFY_URL` is unset, the delivery log line includes the nudge text on purpose.
- No `init()`. No mutable package-level state. Sentinel errors and constants are fine.
- Each package has `doc.go`. Comment exported types and functions.
- CLI library is `github.com/urfave/cli/v3`. Keep `cmd/daywire` small.

## Privacy boundary

- Assistants see redacted semantic events (`commitment`, `ask`, `visit`, `note`) over MCP. They do not receive raw call audio.
- Do not add an audio upload, an attachment route, or a field whose purpose is a recording.
- Bind MCP and the curl routes to loopback only. Reject other addresses in config validation before `net.Listen`. Reject a request whose Host header is not `localhost` or a loopback IP.
- Default nudge transport is ntfy. Do not add Telegram as the default.

## Store

- SQLite through `modernc.org/sqlite` (pure Go).
- Sensitive columns use AES-256-GCM (`internal/crypt`) with a 32-byte key from `DAYWIRE_DATA_KEY`, `DAYWIRE_DATA_KEY_FILE`, or a generated `<data dir>/data.key` (mode `0600`).
- Type, time, and ids stay plaintext so queries work. Document this if the scheme changes.
- Data directory: `$XDG_CONFIG_HOME/daywire` or `~/.config/daywire`, mode `0700`.

## Tests

- `github.com/stretchr/testify/require` only. Do not use `assert`.
- Table-driven tests where the cases are the point.
- `internal/testutil.Context` for a cancelling test context.
- Synthetic fixtures only. No real personal data, no recorded audio, no live ntfy calls.
- `go test -race ./...` should pass. `hack/make.sh all` also runs `gofmt -l` and `go vet`.

## Out of scope unless a human asks

Android / Kotlin / gomobile, Telegram-as-default, Play Store, FCM, Hebrew STT, on-device speech models, and writing licence files.
