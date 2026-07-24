package cli

import (
	"errors"
	"strings"

	"github.com/yasyf/daemonkit/artifact"
)

// Message renders err as a terse, single-line human message for stderr. It maps
// the artifact sentinels to fixed strings and surfaces a ManualUpgradeError's
// brew-upgrade handoff verbatim; anything else falls through to the wrapped
// message with the library's "artifact: " prefix stripped.
func Message(err error) string {
	var manual *artifact.ManualUpgradeError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &manual):
		return clean(manual)
	case errors.Is(err, artifact.ErrSchemaVersion):
		return "descriptor needs a newer binrun; upgrade binrun (brew upgrade binrun)"
	case errors.Is(err, artifact.ErrDynamicIntegrity):
		return "dynamic version requires an independent integrity gate (python-tool or signed-app only)"
	case errors.Is(err, artifact.ErrUnsupportedPlatform):
		return "no artifact for this platform"
	case errors.Is(err, artifact.ErrChecksumMismatch):
		return "artifact checksum mismatch"
	case errors.Is(err, artifact.ErrSizeMismatch):
		return "artifact size mismatch"
	case errors.Is(err, artifact.ErrUnsupportedFormat):
		return "unsupported artifact format"
	case errors.Is(err, artifact.ErrUnsafeArchive):
		return "unsafe archive entry"
	default:
		return clean(err)
	}
}

func clean(err error) string {
	return strings.TrimPrefix(err.Error(), "artifact: ")
}
