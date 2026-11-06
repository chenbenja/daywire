// Package store persists redacted events in SQLite.
//
// Type, time, and identifiers are stored in the clear so they can be filtered.
// Summary text and nudge text are AES-256-GCM ciphertext (see internal/crypt).
// The database file is not SQLCipher; the sensitive columns are encrypted
// instead, which keeps the dependency pure Go. The file mode is 0600.
package store
