package ports_test

import (
	"errors"
	"testing"

	"github.com/fede-iglesias/shipkit/ports"
)

// Compile-time proof that MockAutostartPort satisfies AutostartPort.
var _ ports.AutostartPort = (*ports.MockAutostartPort)(nil)

func TestMockAutostartPort_Install_default(t *testing.T) {
	m := ports.NewMockAutostartPort()
	unit := ports.AutostartUnit{
		Label:   "com.fede-iglesias.kt",
		Program: "/usr/local/bin/kt",
		Args:    []string{"daemon", "run"},
	}
	if err := m.Install(unit); err != nil {
		t.Fatal(err)
	}
	if len(m.InstallCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.InstallCalls))
	}
	if m.InstallCalls[0].Label != "com.fede-iglesias.kt" {
		t.Errorf("expected label recorded, got %q", m.InstallCalls[0].Label)
	}
}

func TestMockAutostartPort_Install_error(t *testing.T) {
	m := ports.NewMockAutostartPort()
	sentinel := errors.New("launchctl failed")
	m.InstallFunc = func(_ ports.AutostartUnit) error { return sentinel }
	err := m.Install(ports.AutostartUnit{Label: "com.test"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

func TestMockAutostartPort_Uninstall_default(t *testing.T) {
	m := ports.NewMockAutostartPort()
	if err := m.Uninstall("com.fede-iglesias.kt"); err != nil {
		t.Fatal(err)
	}
	if len(m.UninstallCalls) != 1 || m.UninstallCalls[0] != "com.fede-iglesias.kt" {
		t.Error("expected label recorded")
	}
}

func TestMockAutostartPort_Status_default(t *testing.T) {
	m := ports.NewMockAutostartPort()
	st, err := m.Status("com.fede-iglesias.kt")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Installed {
		t.Error("expected Installed=true by default")
	}
	if st.Running {
		t.Error("expected Running=false by default")
	}
	if len(m.StatusCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.StatusCalls))
	}
}

func TestMockAutostartPort_Status_func(t *testing.T) {
	m := ports.NewMockAutostartPort()
	m.StatusFunc = func(_ string) (ports.AutostartStatus, error) {
		return ports.AutostartStatus{Installed: true, Running: true, PID: 42}, nil
	}
	st, err := m.Status("label")
	if err != nil || !st.Running || st.PID != 42 {
		t.Errorf("unexpected: %+v, %v", st, err)
	}
}

func TestMockAutostartPort_Stop_default(t *testing.T) {
	m := ports.NewMockAutostartPort()
	if err := m.Stop("com.fede-iglesias.kt"); err != nil {
		t.Fatal(err)
	}
	if len(m.StopCalls) != 1 || m.StopCalls[0] != "com.fede-iglesias.kt" {
		t.Error("expected label recorded")
	}
}

func TestMockAutostartPort_Stop_error(t *testing.T) {
	m := ports.NewMockAutostartPort()
	sentinel := errors.New("service not running")
	m.StopFunc = func(_ string) error { return sentinel }
	err := m.Stop("label")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}

func TestMockAutostartPort_Uninstall_func(t *testing.T) {
	m := ports.NewMockAutostartPort()
	sentinel := errors.New("bootout failed")
	m.UninstallFunc = func(_ string) error { return sentinel }
	err := m.Uninstall("com.fede-iglesias.kt")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
}
