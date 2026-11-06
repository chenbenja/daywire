# Daywire Week 1 POC

**Status:** localhost Go core, November 2026.

Week 1 is a single Go process. It stores redacted semantic events, serves them to an assistant over MCP on `127.0.0.1`, and sends a nudge to ntfy or to the process log. The concept report's Week 1 row named Telegram as the nudge channel. This slice uses **ntfy** instead, and does not ship a Telegram bot.

## What shipped

| Piece | Where | Behavior |
|---|---|---|
| Event schema | `internal/event` | `commitment`, `ask`, `visit`, `note`. Fields: id, type, source id, occurred_at, created_at, redacted summary. |
| Encrypted store | `internal/store`, `internal/crypt` | SQLite via `modernc.org/sqlite`. Summary and nudge text are AES-256-GCM. Type and time stay queryable. Data directory follows XDG (`~/.config/daywire` by default), mode `0700`. |
| MCP | `internal/server` | Streamable HTTP at `http://127.0.0.1:8765/mcp`. Tools: `query_events`, `subscribe`, `propose_nudge`. |
| Subscribe | `internal/service` | Poll. `wait_ms` long-polls up to 25 seconds. `GET /events/poll` is the same call for curl. |
| Nudge | `internal/nudge` | POST to `DAYWIRE_NTFY_URL`. Unset URL logs the nudge and reports channel `log`. |
| Doctor | `daywire doctor` | Bind address is loopback, database opens, MCP answers an in-process `query_events`, live server is probed when one is up, ntfy URL is present or skipped. |
| Demo seed | `daywire seed` | Four synthetic events (calendar, mail, visit, note). Fixed ids, so running it again inserts nothing. No real personal data and no audio. |

The HTTP routes `GET /health`, `GET /events`, `GET /events/poll`, and `POST /nudges` are the curl equivalent of the MCP tools. They are mounted on the same loopback listener. A non-loopback `RemoteAddr` is rejected.

Build and the client snippet are in the [README](../README.md).

## What is explicitly not in Week 1

- Android, Kotlin, or gomobile
- A notification listener, location visits from a phone, or a Storage Access Framework folder
- On-device speech-to-text, speaker identification, or Hebrew STT models
- Any path that accepts or forwards raw call audio, images, or unredacted message bodies
- Telegram as a default nudge channel (no bot token, no Telegram client)
- Play Store packaging, FCM, or a required push provider
- Live IMAP, calendar, or desktop OCR connectors (the seed only pretends those sources)
- Tailscale, OAuth, or any bind address that is not loopback
- A `LICENSE` file (the founder is holding the licence; the README stays **License: TBD**)

The store is append-only in this slice. There is no delete API, no key rotation, and no retention job yet.

## Safety choices already in the binary

- `serve` refuses to listen unless the address is `localhost`, `127.0.0.0/8`, or `::1`.
- Errors and ordinary logs do not include event summaries or nudge bodies. The log nudge channel is the exception: that line is the delivery itself.
- Doctor output includes the ntfy host, not the topic path or bearer token.
- Ntfy redirects are refused, so a topic URL cannot bounce the body to another host.
- The MCP instructions tell the client that audio is not available.
