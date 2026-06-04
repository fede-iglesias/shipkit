module github.com/fede-iglesias/shipkit/lifecycle/update

go 1.26.3

require (
	github.com/fede-iglesias/shipkit/lifecycle/migrations v0.1.0
	github.com/fede-iglesias/shipkit/lifecycle/recovery v0.0.0
)

// Local development replace for the shipkit mono-repo. Strip this before
// creating a release tag for lifecycle/update.
replace github.com/fede-iglesias/shipkit/lifecycle/recovery => ../recovery
