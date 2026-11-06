// Package crypt encrypts individual column values with AES-256-GCM.
//
// The key is 32 bytes supplied by the caller. This package does not read the
// environment or the filesystem.
package crypt
