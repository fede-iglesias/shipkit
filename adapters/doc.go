// Package adapters provides production implementations for all 11 port
// interfaces declared in [github.com/fede-iglesias/shipkit/ports].
//
// # Design
//
// shipkit follows the hexagonal / ports-and-adapters architecture pattern.
// Port interfaces are defined in shipkit/ports and contain no implementation
// details. This package provides the concrete implementations that wire real
// operating-system calls, network clients, and third-party libraries to those
// interfaces.
//
// Five adapters (HTTP, FS, Cosign, Spawn) delegate to the implementations
// already shipped in [github.com/fede-iglesias/shipkit/lifecycle/update] by
// re-exporting their constructors. This avoids duplication while keeping the
// shipkit/adapters module as the single import path for consumer wiring.
//
// The six new adapters (Paths, Env, ShellRc, Completion, Autostart, Prompt)
// are implemented here from scratch.
//
// # Production wiring pattern
//
// Consumer cmd packages wire adapters once at startup:
//
//	import (
//	    "github.com/fede-iglesias/shipkit/adapters"
//	    "github.com/fede-iglesias/shipkit"
//	)
//
//	func main() {
//	    http := adapters.NewGitHubHTTP()
//	    fs   := adapters.NewRealFs()
//	    cos  := adapters.NewSigstoreCosign()
//	    cos.SetVerifyCore(sigstoreRealVerify) // wired in cmd layer
//	    // ... remaining ports ...
//	    cfg := shipkit.Config{...}
//	    shipkit.RegisterLifecycle(root, cfg,
//	        shipkit.WithHTTPPort(http),
//	        shipkit.WithFsPort(fs),
//	        shipkit.WithCosignPort(cos),
//	    )
//	}
//
// # See also
//
//   - [github.com/fede-iglesias/shipkit/ports] for the interface definitions.
//   - [github.com/fede-iglesias/shipkit/lifecycle/update] for the update verb.
//   - [github.com/fede-iglesias/shipkit] for RegisterLifecycle and Option DI.
package adapters
