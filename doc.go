// Package shipkit is a toolkit for building self-contained terminal apps in Go:
// the same binary installs, updates, diagnoses and uninstalls itself, and ships
// with a set of composable terminal UI components.
//
// # Layered architecture
//
// Dependencies always point downward. The base layer (the ui sub-packages and
// theme) is agnostic of everything above it; screens and lifecycle compose the
// base. A widget in ui must never import lifecycle or screens.
//
//	app/         App config + Build()    — the consuming app configures here
//	screens/     menu, dashboard, logs   — TUIs composing the layers below
//	lifecycle/   install, update, uninstall, doctor, clean
//	ui/ + theme/ base widgets (palette-agnostic) + styles
//
// shipkit authenticates against GitHub releases transparently through the
// user's gh CLI session, so distribution works for private and public repos
// alike.
//
// This module is in early development; the API is unstable until the first v1
// tag is cut.
package shipkit
