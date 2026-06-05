module github.com/fede-iglesias/shipkit

go 1.26.3

require (
	github.com/fede-iglesias/shipkit/adapters v0.2.0
	github.com/fede-iglesias/shipkit/lifecycle/clean v0.2.0
	github.com/fede-iglesias/shipkit/lifecycle/doctor v0.2.0
	github.com/fede-iglesias/shipkit/lifecycle/install v0.2.3
	github.com/fede-iglesias/shipkit/lifecycle/migrations v0.1.0
	github.com/fede-iglesias/shipkit/lifecycle/uninstall v0.1.3
	github.com/fede-iglesias/shipkit/lifecycle/update v0.2.2
	github.com/fede-iglesias/shipkit/ports v0.2.0
	github.com/spf13/cobra v1.10.2
)

require (
	github.com/fede-iglesias/shipkit/lifecycle/recovery v0.1.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/term v0.32.0 // indirect
)

retract v0.2.1 // missing go.sum entries for v0.2.1 submodule pins; use v0.2.2
