package mobile_test

import (
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// TestGobindCanExportCoreMethods fails when an exported Core method uses a
// type the pinned gobind skips. That generator accepts []byte and rejects
// every other slice, including []float32, then omits the Java method.
func TestGobindCanExportCoreMethods(t *testing.T) {
	t.Parallel()
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedTypes}, "github.com/chenbenja/daywire/mobile")
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	require.Empty(t, pkgs[0].Errors)

	obj := pkgs[0].Types.Scope().Lookup("Core")
	require.NotNil(t, obj)
	named, ok := obj.Type().(*types.Named)
	require.True(t, ok)

	var sawSave, sawLabel bool
	for i := 0; i < named.NumMethods(); i++ {
		method := named.Method(i)
		if !method.Exported() {
			continue
		}
		sig := method.Type().(*types.Signature)
		require.Truef(t, gobindSigSupported(sig), "gobind would skip Core.%s", method.Name())
		switch method.Name() {
		case "SaveOwnerVoiceprint", "LabelOwner":
			require.Equal(t, 1, sig.Params().Len())
			require.Truef(t, isByteSlice(sig.Params().At(0).Type()), "%s must take []byte", method.Name())
			if method.Name() == "SaveOwnerVoiceprint" {
				sawSave = true
			} else {
				sawLabel = true
			}
		}
	}
	require.True(t, sawSave)
	require.True(t, sawLabel)
}

func gobindSigSupported(sig *types.Signature) bool {
	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		if !gobindSupported(params.At(i).Type()) {
			return false
		}
	}
	results := sig.Results()
	for i := 0; i < results.Len(); i++ {
		if !gobindSupported(results.At(i).Type()) {
			return false
		}
	}
	return true
}

// gobindSupported matches golang.org/x/mobile/bind.Generator.isSupported for
// the types this bind surface uses. Slices other than []byte are rejected.
func gobindSupported(t types.Type) bool {
	if types.Identical(t, types.Universe.Lookup("error").Type()) {
		return true
	}
	switch t := types.Unalias(t).(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.Bool, types.Int, types.Int8, types.Uint8, types.Int16, types.Int32, types.Int64, types.Float32, types.Float64, types.String:
			return true
		default:
			return false
		}
	case *types.Slice:
		return isByteSlice(t)
	case *types.Pointer:
		named, ok := types.Unalias(t.Elem()).(*types.Named)
		return ok && named.Obj().Pkg() != nil
	case *types.Named:
		switch t.Underlying().(type) {
		case *types.Interface, *types.Pointer:
			return t.Obj().Pkg() != nil
		default:
			return false
		}
	default:
		return false
	}
}

func isByteSlice(t types.Type) bool {
	slice, ok := t.Underlying().(*types.Slice)
	if !ok {
		return false
	}
	elem, ok := slice.Elem().Underlying().(*types.Basic)
	return ok && elem.Kind() == types.Uint8
}
