package shipkit

import (
	"github.com/fede-iglesias/shipkit/ports"
	"github.com/spf13/cobra"
)

// Option is a functional option that modifies the DI configuration for
// [RegisterLifecycle] and the per-verb getters. Apply options after creating
// a [Config] to inject test doubles or selectively disable verbs.
type Option func(*optionState)

// optionState holds the accumulated option settings. All fields are zero-valued
// (nil / false) by default, meaning production adapters are used.
type optionState struct {
	// withoutInstall disables the install verb when RegisterLifecycle is called.
	withoutInstall bool
	// withoutUpdate disables the update verb when RegisterLifecycle is called.
	withoutUpdate bool
	// withoutUninstall disables the uninstall verb when RegisterLifecycle is called.
	withoutUninstall bool
	// withoutDoctor disables the doctor verb when RegisterLifecycle is called.
	withoutDoctor bool
	// withoutClean disables the clean verb when RegisterLifecycle is called.
	withoutClean bool

	// Port overrides - nil means "use the production default".
	http       ports.HTTPPort
	fs         ports.FsPort
	cosign     ports.CosignPort
	spawn      ports.SpawnPort
	clock      ports.ClockPort
	paths      ports.PathsPort
	env        ports.EnvPort
	shellRc    ports.ShellRcPort
	completion ports.CompletionPort
	autostart  ports.AutostartPort
	prompt     ports.PromptPort

	// Verb builder overrides. nil means use the real implementation.
	// These are package-internal and used only in tests to simulate errors.
	installCmdFn   func(Config, ...Option) (*cobra.Command, error)
	updateCmdFn    func(Config, ...Option) (*cobra.Command, error)
	uninstallCmdFn func(Config, ...Option) (*cobra.Command, error)
	doctorCmdFn    func(Config, ...Option) (*cobra.Command, error)
	cleanCmdFn     func(Config, ...Option) (*cobra.Command, error)
}

// applyOptions returns an optionState after applying all opts in order.
func applyOptions(opts []Option) *optionState {
	o := &optionState{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithoutInstall disables the install subcommand when using [RegisterLifecycle].
func WithoutInstall() Option {
	return func(o *optionState) { o.withoutInstall = true }
}

// WithoutUpdate disables the update subcommand when using [RegisterLifecycle].
func WithoutUpdate() Option {
	return func(o *optionState) { o.withoutUpdate = true }
}

// WithoutUninstall disables the uninstall subcommand when using [RegisterLifecycle].
func WithoutUninstall() Option {
	return func(o *optionState) { o.withoutUninstall = true }
}

// WithoutDoctor disables the doctor subcommand when using [RegisterLifecycle].
func WithoutDoctor() Option {
	return func(o *optionState) { o.withoutDoctor = true }
}

// WithoutClean disables the clean subcommand when using [RegisterLifecycle].
func WithoutClean() Option {
	return func(o *optionState) { o.withoutClean = true }
}

// WithHTTPPort injects a custom [ports.HTTPPort] in place of the production
// GitHub HTTP adapter. Used in tests to avoid real network calls in the
// doctor verb.
func WithHTTPPort(p ports.HTTPPort) Option {
	return func(o *optionState) { o.http = p }
}

// WithFsPort injects a custom [ports.FsPort] in place of the production
// filesystem adapter. Applies to install, uninstall, doctor, and clean.
func WithFsPort(p ports.FsPort) Option {
	return func(o *optionState) { o.fs = p }
}

// WithCosignPort injects a custom [ports.CosignPort] in place of the
// production sigstore adapter.
func WithCosignPort(p ports.CosignPort) Option {
	return func(o *optionState) { o.cosign = p }
}

// WithSpawnPort injects a custom [ports.SpawnPort] in place of the production
// spawn adapter. Applies to the doctor verb.
func WithSpawnPort(p ports.SpawnPort) Option {
	return func(o *optionState) { o.spawn = p }
}

// WithClockPort injects a custom [ports.ClockPort] in place of the production
// clock adapter. Applies to all verbs.
func WithClockPort(p ports.ClockPort) Option {
	return func(o *optionState) { o.clock = p }
}

// WithPathsPort injects a custom [ports.PathsPort] in place of the production
// XDG adapter. Applies to install, uninstall, doctor, and clean.
func WithPathsPort(p ports.PathsPort) Option {
	return func(o *optionState) { o.paths = p }
}

// WithEnvPort injects a custom [ports.EnvPort] in place of the production
// OS environment adapter. Applies to install and doctor.
func WithEnvPort(p ports.EnvPort) Option {
	return func(o *optionState) { o.env = p }
}

// WithShellRcPort injects a custom [ports.ShellRcPort] in place of the
// production shell RC adapter. Applies to install and uninstall.
func WithShellRcPort(p ports.ShellRcPort) Option {
	return func(o *optionState) { o.shellRc = p }
}

// WithCompletionPort injects a custom [ports.CompletionPort] in place of the
// production cobra completion adapter. Applies to install, uninstall, and
// doctor.
func WithCompletionPort(p ports.CompletionPort) Option {
	return func(o *optionState) { o.completion = p }
}

// WithAutostartPort injects a custom [ports.AutostartPort] in place of the
// production autostart adapter. Applies to install, uninstall, and doctor.
func WithAutostartPort(p ports.AutostartPort) Option {
	return func(o *optionState) { o.autostart = p }
}

// WithPromptPort injects a custom [ports.PromptPort] in place of the
// production terminal prompt adapter. Applies to install, uninstall, and
// clean.
func WithPromptPort(p ports.PromptPort) Option {
	return func(o *optionState) { o.prompt = p }
}

// withInstallCmdFn replaces the install verb builder. Package-internal; used
// in tests to simulate builder failures without requiring an invalid config.
func withInstallCmdFn(fn func(Config, ...Option) (*cobra.Command, error)) Option {
	return func(o *optionState) { o.installCmdFn = fn }
}

// withUpdateCmdFn replaces the update verb builder. Package-internal.
func withUpdateCmdFn(fn func(Config, ...Option) (*cobra.Command, error)) Option {
	return func(o *optionState) { o.updateCmdFn = fn }
}

// withUninstallCmdFn replaces the uninstall verb builder. Package-internal.
func withUninstallCmdFn(fn func(Config, ...Option) (*cobra.Command, error)) Option {
	return func(o *optionState) { o.uninstallCmdFn = fn }
}

// withDoctorCmdFn replaces the doctor verb builder. Package-internal.
func withDoctorCmdFn(fn func(Config, ...Option) (*cobra.Command, error)) Option {
	return func(o *optionState) { o.doctorCmdFn = fn }
}

// withCleanCmdFn replaces the clean verb builder. Package-internal.
func withCleanCmdFn(fn func(Config, ...Option) (*cobra.Command, error)) Option {
	return func(o *optionState) { o.cleanCmdFn = fn }
}
