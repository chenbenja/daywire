// Package config loads Daywire process settings.
//
// Secrets stay in the returned Config. Callers must not log Config itself:
// DataKey and NtfyToken are credentials, and NtfyURL can identify a private topic.
package config
