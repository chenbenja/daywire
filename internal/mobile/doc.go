// Package mobile is the on-device Daywire core used by the Android shell.
//
// It opens the same SQLite store as the Week 1 process and does not listen on
// a socket. Summaries and nudge text are AES-256-GCM ciphertext. Identifiers,
// types, times, and source ids stay plaintext. Notification events are notes built from the app label,
// package name, and an allow-listed platform category. The notification title
// and body are not parameters of this API. Visit summaries use a 0.01 degree
// grid. Raw coordinates are not stored and are not logged. Nudges recorded
// here use the local channel and do not call ntfy. Speech turns are notes
// whose summary is a speaker label plus transcript text. The audio, the
// file path, and the voiceprint vector are not parameters of that call.
// The voiceprint file uses the same AES-256-GCM column as the laptop spike.
package mobile
