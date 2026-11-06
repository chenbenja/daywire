// Package voiceprint stores one enrolled speaker embedding.
//
// The file format is the laptop speaker spike: AES-256-GCM with authenticated
// data id + NUL + "speaker.embedding", under <data-dir>/speakers/<id>.emb.
// The directory is mode 0700 and the file is mode 0600. The vector is not
// logged. This package does not download models, open a socket, or import
// sherpa. The phone scores each window with LabelTurn (owner, other, or
// unknown) instead of clustering a whole recording.
package voiceprint
