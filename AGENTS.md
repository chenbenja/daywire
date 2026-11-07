# AGENTS.md

Daywire is a phone-first life-context bridge. Week 1 is a localhost Go process: redacted events, an encrypted SQLite store, a loopback MCP server, ntfy nudges, and `daywire doctor`. Week 2 is a thin Android shell that uses that store on the phone. Week 3 adds on-device speech in that shell: enroll and forget an owner voiceprint, then diarize, embed, and transcribe a picked WAV. The laptop speaker-id spike stays behind the `speaker` build tag and is not linked into the Go archive.

These conventions follow the shape of Chen's private Go services (in particular the sanit-internal layout and review habits). They are adapted for Daywire. Do not copy Sanit product code into this repo.

## License

The intended licence for the hub and app core is AGPL, with a permissive adapter SDK later. That is intent only. Do not add a `LICENSE` file, AGPL text, or copyright headers until the founder adds them. Keep the README line **License: TBD**.

## Layout

- `cmd/daywire/` is a thin `main`. Flag parsing and subcommands live in `internal/cli`.
- Product code lives under `internal/`. One concern per package: `event`, `crypt`, `config`, `store`, `proposal`, `nudge`, `seed`, `service`, `server`, `doctor`, `speaker`, `voiceprint`, `speech`, `mobile`. `proposal` checks nudge text and does not import a network client. `speaker` is the laptop enroll-then-owner spike. The sherpa engine is behind the `speaker` build tag, so the default `daywire` binary stays pure Go. The spike does not add an audio field or a network bind. `voiceprint` is the encrypted embedding file and the owner/other/unknown cut. `speech` caps speaker-labeled transcript text. Neither imports sherpa or `net/http`. `internal/mobile` is the on-device store API. `mobile/` is the gomobile bind surface, because the generated JNI wrapper cannot import an `internal` package.
- `android/` is the phone shell. Week 3 speech inference is the sherpa-onnx Android archive, fetched by `./hack/fetch-speech-models.sh`, not the Go archive. `./hack/make.sh all` does not build the app and does not download models. `./hack/make.sh apk` is the debug APK: command-line SDK, pinned models, bind, unit tests, and `assembleDebug`. `./hack/android-bind.sh` writes `android/app/libs/daywire.aar` and must not pass `-tags speaker`.
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

- Assistants see redacted semantic events (`commitment`, `ask`, `visit`, `note`) over MCP. They do not receive raw call audio or speech transcripts. A speech turn is type `speech`, source id `android:speech`, with no file path. The phone UI can list it. `query_events` and subscribe omit it.
- Do not add an audio upload, an attachment route, or a field whose purpose is a recording.
- Bind MCP and the curl routes to loopback only. Reject other addresses in config validation before `net.Listen`. Reject a request whose Host header is not `localhost` or a loopback IP.
- Default nudge transport on the laptop is ntfy. Do not add Telegram as the default. The phone shell uses NotificationCompat and does not require ntfy.
- The Android shell asks at runtime for notification access and coarse location. Both default off. The notification allow-list starts empty. Do not store or log raw notification text, and do not log precise coordinates.

## Store

- SQLite through `modernc.org/sqlite` (pure Go).
- Sensitive columns use AES-256-GCM (`internal/crypt`) with a 32-byte key from `DAYWIRE_DATA_KEY`, `DAYWIRE_DATA_KEY_FILE`, or a generated `<data dir>/data.key` (mode `0600`).
- Type, time, and ids stay plaintext so queries work. Document this if the scheme changes.
- Data directory: `$XDG_CONFIG_HOME/daywire` or `~/.config/daywire`, mode `0700`. On the phone it is the app-private no-backup store directory.
- The owner voiceprint is `<data-dir>/speakers/owner.emb`, mode `0600`, directory `0700`, AES-256-GCM column `speaker.embedding`. Forgetting enrollment deletes that file.

## Tests

- `github.com/stretchr/testify/require` only. Do not use `assert`.
- Table-driven tests where the cases are the point.
- `internal/testutil.Context` for a cancelling test context.
- Synthetic fixtures only. No real personal data, no recorded audio, no live ntfy calls.
- `go test -race ./...` should pass. `hack/make.sh all` also runs `gofmt -l` and `go vet`.

## Out of scope unless a human asks

Telegram-as-default, Play Store listing, FCM, and writing licence files. Do not pass `-tags speaker` to the gomobile bind, and do not put sherpa or onnxruntime inside `daywire.aar`. Phone speech uses the separate sherpa-onnx Android archive. Hebrew STT is the pinned ivrit-ai Zipformer when those files were fetched at build time.
