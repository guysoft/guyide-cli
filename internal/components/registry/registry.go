// Package registry blank-imports every component driver so that their
// init() functions run and register themselves with the components
// registry. Other code (cmd/guyide, install manager, doctor) imports
// this package once and gets the full driver set.
//
// Adding a new driver: write the package under
// internal/components/{slot}/, then add a blank import here.
package registry

import (
	// Editor drivers.
	_ "github.com/guysoft/guyide-cli/internal/components/editor"
	// Multiplexer drivers.
	_ "github.com/guysoft/guyide-cli/internal/components/multiplexer"
	// Agent drivers will land here in M2 ckpt 5.
)
