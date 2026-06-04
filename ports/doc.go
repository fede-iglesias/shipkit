// Package ports declares the 11 port interfaces (Hexagonal Architecture) used
// by the shipkit lifecycle verbs: install, update, uninstall, doctor, and clean.
//
// # Design
//
// shipkit follows the Ports and Adapters (Hexagonal Architecture) pattern:
//
//   - Ports (this package) own the contracts - they define what a capability
//     looks like, independent of how it is implemented.
//   - Adapters (shipkit/adapters) own the production wiring - they satisfy the
//     interfaces by delegating to real OS calls, HTTP clients, or third-party
//     libraries.
//   - Consumers (lifecycle verbs) declare their dependencies as port interfaces
//     in a Deps struct and accept adapters via constructor injection.
//
// This separation enables 100% unit-testable verb logic: every test replaces
// the real adapters with lightweight fakes (Mock helpers are provided in each
// file below the interface declaration).
//
// Five ports mirror the existing lifecycle/update/ports interfaces structurally
// to ensure adapters from that module remain compatible:
//
//   - HTTPPort, FsPort (extended), CosignPort, SpawnPort, ClockPort
//
// Six ports are new, required by install/uninstall/doctor/clean:
//
//   - PathsPort, EnvPort, ShellRcPort, CompletionPort, AutostartPort, PromptPort
//
// # Usage
//
// Import only the interfaces your verb needs. Declare them in a Deps struct and
// accept a concrete implementation via constructor injection:
//
//	package install
//
//	import (
//		"context"
//		"github.com/fede-iglesias/shipkit/ports"
//	)
//
//	type Deps struct {
//		FS         ports.FsPort
//		Paths      ports.PathsPort
//		Env        ports.EnvPort
//		ShellRc    ports.ShellRcPort
//		Completion ports.CompletionPort
//		Autostart  ports.AutostartPort
//		Prompt     ports.PromptPort
//		Clock      ports.ClockPort
//	}
//
//	func Run(ctx context.Context, deps Deps, opts Options, ...) (Result, error) {
//		dirs, err := deps.Paths.DataDir("myapp")
//		// ...
//	}
//
// In production main, inject real adapters from shipkit/adapters. In tests,
// inject the MockXxx helpers provided in each file of this package.
//
// # See also
//
// [github.com/fede-iglesias/shipkit/lifecycle/install] for the install verb.
// [github.com/fede-iglesias/shipkit/lifecycle/uninstall] for the uninstall verb.
// [github.com/fede-iglesias/shipkit/lifecycle/doctor] for the doctor verb.
// [github.com/fede-iglesias/shipkit/lifecycle/clean] for the clean verb.
// [github.com/fede-iglesias/shipkit/lifecycle/update] for the update verb.
package ports
