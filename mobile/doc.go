// Package mobile is the gomobile bind surface for the on-device core.
//
// The behaviour lives in internal/mobile. This package is a separate import
// path because gomobile generates its JNI wrapper outside the module, and Go
// rejects an import of an internal package from that wrapper. Bind this
// package, not internal/mobile. Do not pass the speaker build tag. Speech
// models stay in the sherpa-onnx Android archive, not in this bind.
package mobile
