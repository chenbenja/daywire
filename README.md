# Daywire

**Your AI finally knows what happened today — and the raw data never left your phone.**

Daywire is an open-source, phone-first life-context bridge. It turns the day into redacted semantic events and hands those events to the assistant you already use.

**Status:** Week 1 localhost Go core, plus a Week 2 Android shell. The laptop process binds to loopback, stores redacted events in SQLite, and speaks MCP. The phone app stores the same events on the device. A laptop speaker-identification spike (`./hack/make.sh speaker`) enrolls a voice locally and labels that cluster `owner`. It does not send audio to assistants, and it is not in the phone app. On-device speech-to-text is still later.

**License:** TBD

## Week 1

The `daywire` binary:

- stores `commitment`, `ask`, `visit`, and `note` events
- encrypts summary and nudge text with AES-256-GCM
- serves MCP on `127.0.0.1` only (`query_events`, `subscribe`, `propose_nudge`)
- delivers nudges to [ntfy](https://ntfy.sh) or, when no topic is set, writes the nudge to the log
- checks itself with `daywire doctor`
- seeds a few synthetic calendar and mail events (`daywire seed`)

Assistants receive the redacted summary. There is no audio field and no upload route.

What is intentionally absent from the laptop core is listed in [docs/poc-week1.md](docs/poc-week1.md). The laptop speaker spike is [docs/speaker-spike.md](docs/speaker-spike.md). The phone shell is [docs/poc-week2-android.md](docs/poc-week2-android.md).

## Speaker spike

`daywire speaker` on the Week 1 binary (`./hack/make.sh all`, `CGO_ENABLED=0`) reports that the engine is unavailable. The enroll-then-label check is a separate build so `serve` does not link sherpa:

```bash
./hack/make.sh speaker
./output/daywire-speaker speaker demo --data-dir /tmp/daywire-speaker
```

That build downloads the pinned models into `output/speaker-models/` (gitignored) and logs `label=owner` on the matching LibriSpeech reader. It is not the MCP server. Details are in [docs/speaker-spike.md](docs/speaker-spike.md).

## Week 2 Android shell

`android/` is a project Android Studio can open. Summaries and nudge text are AES-256-GCM on the phone; identifiers, types, times, and source names stay readable, and the database file is not encrypted as a whole. The key is a file in app-private storage, not the Android Keystore. The app asks before reading notifications or location, lists a folder you pick, and shows nudges with the phone's own notifications. ntfy is not used on the phone. Speech models are not in the app.

Open the project only after the Go archive exists:

```bash
export ANDROID_HOME="$HOME/Android/Sdk"   # Android Studio's SDK path
./hack/android-bind.sh
```

Then in Android Studio: File → Open → the `android/` directory. The click-by-click for notification access, location, and the folder picker is in [docs/poc-week2-android.md](docs/poc-week2-android.md).

## Build

Requires Go 1.26 (the `go` directive in `go.mod`; the toolchain downloads it when `GOTOOLCHAIN=auto`).

```bash
./hack/make.sh all
```

That runs `gofmt -l`, `go vet`, `go test -race ./...`, and writes `output/daywire` with `CGO_ENABLED=0`. The Makefile calls the same script: `make all`, `make test`, `make build`. `./hack/make.sh speaker` is the opt-in sherpa build (`output/daywire-speaker`).

## Run

```bash
./output/daywire seed
./output/daywire doctor
./output/daywire serve
```

`serve` listens on `127.0.0.1:8765` until SIGINT or SIGTERM. Any other bind address is refused. Data lives in `$XDG_CONFIG_HOME/daywire` or `~/.config/daywire` (`events.db` and `data.key`, directory mode `0700`, files mode `0600`).

In another shell, the same operations are available over HTTP on that loopback port. These routes call the same code as the MCP tools. A request is rejected unless its `Host` header is `localhost` or a loopback address:

```bash
curl -sS http://127.0.0.1:8765/health
curl -sS 'http://127.0.0.1:8765/events?limit=10'
curl -sS 'http://127.0.0.1:8765/events?type=visit'
curl -sS 'http://127.0.0.1:8765/events/poll?limit=10'
curl -sS -X POST http://127.0.0.1:8765/nudges \
  -H 'content-type: application/json' \
  -d '{"title":"Synthetic nudge","message":"Pack the spare charger."}'
```

With `DAYWIRE_NTFY_URL` unset, the nudge is the log line `nudge logged` on the server's stderr. That log line is the desktop fallback, so it includes the nudge text. Other logs do not include event or nudge bodies.

### ntfy

```bash
export DAYWIRE_NTFY_URL='https://ntfy.sh/your-private-topic'
# optional, for a topic that requires auth; never logged
export DAYWIRE_NTFY_TOKEN='your-token'
./output/daywire serve
```

`propose_nudge` then POSTs the title and message to that URL. `daywire doctor` reports the host only. It does not print the topic path or the token. An unset URL is a skip, not a failure.

## MCP client

The streamable HTTP endpoint is `http://127.0.0.1:8765/mcp`.

Cursor (or another client that takes a URL):

```json
{
  "mcpServers": {
    "daywire": {
      "url": "http://127.0.0.1:8765/mcp"
    }
  }
}
```

Tools:

| Tool | Arguments | Result |
|---|---|---|
| `query_events` | `type`, `since`, `until` (RFC3339), `limit` (default 50, max 200) | redacted events, newest `occurred_at` first |
| `subscribe` | `cursor`, `limit`, `wait_ms` (max 25000) | events created after the cursor, plus the next cursor. Week 1 is a poll / long-poll, not a push stream |
| `propose_nudge` | `title`, `message`, optional `priority` 1–5 | `{id, channel, delivered, detail}` |

`GET /events/poll` is the curl form of `subscribe`.

## Configuration

| Variable | Purpose |
|---|---|
| `DAYWIRE_ADDR` | Listen address. Default `127.0.0.1:8765`. Must be loopback. |
| `DAYWIRE_DATA_DIR` | Data directory. Default `$XDG_CONFIG_HOME/daywire` or `~/.config/daywire`. |
| `DAYWIRE_DATA_KEY` | Base64-encoded 32-byte AES key. Not written to disk. |
| `DAYWIRE_DATA_KEY_FILE` | File containing that key (base64 or 32 raw bytes). Missing file is an error. |
| `DAYWIRE_NTFY_URL` | Full ntfy topic URL. Unset means log the nudge. |
| `DAYWIRE_NTFY_TOKEN` | Optional bearer token for ntfy. |
| `DAYWIRE_LOG_LEVEL` | `debug`, `info`, `warn`, or `error`. |
| `DAYWIRE_SPEAKER_MODEL_DIR` | Cache for the laptop speaker-id models. Default `output/speaker-models` inside a checkout, otherwise `$XDG_CACHE_HOME/daywire/speaker`. |

If neither key variable is set, the first command creates `<data dir>/data.key` with mode `0600` and reuses it. Flags `--addr` and `--data-dir` override the matching variables. The key and the ntfy token are environment-only so they do not appear in the process list.

### Encryption

The database is [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, not SQLCipher). `type`, timestamps, and ids stay in the clear so `query_events` can filter. The summary, nudge title, and nudge message are AES-256-GCM ciphertext, each bound to its row id. Changing the key does not rewrite old rows; opens of those rows fail until the original key is restored. There is no key rotation in Week 1.

## Docs

- [Week 1 POC notes](docs/poc-week1.md)
- [Week 2 Android shell](docs/poc-week2-android.md)
- [Laptop speaker-id spike](docs/speaker-spike.md)
- [Concept report (v5)](docs/research/daywire-concept-report.md)
- [Research notes](docs/research/notes/)

## Naming history

Earlier working titles: *life-context bridge*; *Lifebus* was a candidate name. The project is now **Daywire**.
