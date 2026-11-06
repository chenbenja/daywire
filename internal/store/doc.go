// Package store persists redacted events in SQLite.
//
// Type, time, and identifiers are stored in the clear so they can be filtered.
// Summary text and nudge text are AES-256-GCM ciphertext (see internal/crypt).
// The database file is not SQLCipher; the sensitive columns are encrypted
// instead, which keeps the dependency pure Go. The file mode is 0600.
// Nudge channels are log, ntfy, and local. The local channel is an on-device
// notification record and does not imply a network delivery. Speech turns
// are stored with the other events. Assistant queries set OccurredFilter.Assistant
// and use ListAssistantSince so those turns are omitted.
package store
