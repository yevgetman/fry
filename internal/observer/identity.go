package observer

import (
	"github.com/yevgetman/fry/internal/consciousness"
)

// ReadIdentity returns Fry's canonical identity from the embedded template
// files. The identity is read-only during builds — it is compiled into the
// binary and updated only by the Reflection process between builds.
func ReadIdentity() (string, error) {
	return consciousness.LoadCoreIdentity()
}
