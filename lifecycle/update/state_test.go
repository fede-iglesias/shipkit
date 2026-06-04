package update_test

import (
	"slices"
	"testing"

	"github.com/fede-iglesias/shipkit/lifecycle/update"
)

// TestEvent_Constants verifies all Event constants have the expected string values.
func TestEvent_Constants(t *testing.T) {
	tests := []struct {
		event update.Event
		want  string
	}{
		{update.EventSuccess, "success"},
		{update.EventFailure, "failure"},
		{update.EventCancel, "cancel"},
	}
	for _, tc := range tests {
		if string(tc.event) != tc.want {
			t.Errorf("Event constant: got %q, want %q", tc.event, tc.want)
		}
	}
}

// TestTransitions_FullTable checks that the table has the expected size and no duplicate (From,Event) pairs.
func TestTransitions_FullTable(t *testing.T) {
	table := update.Transitions()

	// Expect at least one transition per non-terminal state.
	if len(table) == 0 {
		t.Fatal("Transitions() returned empty table")
	}

	// No duplicate (From, Event) pairs.
	seen := make(map[struct {
		From  update.State
		Event update.Event
	}]update.State)
	for _, tr := range table {
		key := struct {
			From  update.State
			Event update.Event
		}{tr.From, tr.Event}
		if prev, exists := seen[key]; exists {
			t.Errorf("duplicate transition: From=%s Event=%s appears as To=%s and To=%s", tr.From, tr.Event, prev, tr.To)
		}
		seen[key] = tr.To
	}
}

// TestStateOrder_ForwardPath verifies the forward path states have strictly increasing order values.
func TestStateOrder_ForwardPath(t *testing.T) {
	forwardOrder := []update.State{
		update.StatePreUpdate,
		update.StateSnapshotTree,
		update.StateDownloadBinary,
		update.StateVerifyCosign,
		update.StateAtomicReplace,
		update.StateMigrateTree,
		update.StateHealthCheck,
		update.StateCommitted,
	}
	for i := 1; i < len(forwardOrder); i++ {
		prev := update.StateOrder(forwardOrder[i-1])
		curr := update.StateOrder(forwardOrder[i])
		if curr <= prev {
			t.Errorf("forward path order not strictly increasing: StateOrder(%s)=%d >= StateOrder(%s)=%d",
				forwardOrder[i-1], prev, forwardOrder[i], curr)
		}
	}
}

// TestStateOrder_RollbackPath verifies rollback path states have strictly increasing order values.
func TestStateOrder_RollbackPath(t *testing.T) {
	rollbackOrder := []update.State{
		update.StateRollingBack,
		update.StateRestoreTree,
		update.StateRestoreOldBinary,
		update.StateVerifyRollback,
		update.StateRolledBack,
	}
	for i := 1; i < len(rollbackOrder); i++ {
		prev := update.StateOrder(rollbackOrder[i-1])
		curr := update.StateOrder(rollbackOrder[i])
		if curr <= prev {
			t.Errorf("rollback path order not strictly increasing: StateOrder(%s)=%d >= StateOrder(%s)=%d",
				rollbackOrder[i-1], prev, rollbackOrder[i], curr)
		}
	}
}

// TestStateOrder_UnknownStateReturnsNegative verifies an unknown state returns -1.
func TestStateOrder_UnknownStateReturnsNegative(t *testing.T) {
	got := update.StateOrder(update.State("nonexistent"))
	if got != -1 {
		t.Errorf("StateOrder(unknown): got %d, want -1", got)
	}
}

// TestIsTerminal_TerminalsTrue verifies all terminal states are identified correctly.
func TestIsTerminal_TerminalsTrue(t *testing.T) {
	terminals := []update.State{
		update.StateCommitted,
		update.StateRolledBack,
		update.StateFailedUnrecoverable,
	}
	for _, s := range terminals {
		if !update.IsTerminal(s) {
			t.Errorf("IsTerminal(%s) = false, want true", s)
		}
	}
}

// TestIsTerminal_NonTerminalsFalse verifies non-terminal states return false.
func TestIsTerminal_NonTerminalsFalse(t *testing.T) {
	nonTerminals := []update.State{
		update.StatePreUpdate,
		update.StateSnapshotTree,
		update.StateDownloadBinary,
		update.StateVerifyCosign,
		update.StateAtomicReplace,
		update.StateMigrateTree,
		update.StateHealthCheck,
		update.StateRollingBack,
		update.StateRestoreTree,
		update.StateRestoreOldBinary,
		update.StateVerifyRollback,
	}
	for _, s := range nonTerminals {
		if update.IsTerminal(s) {
			t.Errorf("IsTerminal(%s) = true, want false", s)
		}
	}
}

// TestIsForwardPath verifies forward-path states and excludes rollback/terminal states.
func TestIsForwardPath(t *testing.T) {
	forwardStates := []update.State{
		update.StatePreUpdate,
		update.StateSnapshotTree,
		update.StateDownloadBinary,
		update.StateVerifyCosign,
		update.StateAtomicReplace,
		update.StateMigrateTree,
		update.StateHealthCheck,
		update.StateCommitted,
	}
	notForward := []update.State{
		update.StateRollingBack,
		update.StateRestoreTree,
		update.StateRestoreOldBinary,
		update.StateVerifyRollback,
		update.StateRolledBack,
		update.StateFailedUnrecoverable,
	}

	for _, s := range forwardStates {
		if !update.IsForwardPath(s) {
			t.Errorf("IsForwardPath(%s) = false, want true", s)
		}
	}
	for _, s := range notForward {
		if update.IsForwardPath(s) {
			t.Errorf("IsForwardPath(%s) = true, want false", s)
		}
	}
}

// TestValidateTransitions_NoOrphans verifies every non-terminal state has at least one outgoing transition.
func TestValidateTransitions_NoOrphans(t *testing.T) {
	if err := update.ValidateTransitions(); err != nil {
		t.Errorf("ValidateTransitions() returned error: %v", err)
	}
}

// TestValidateTransitions_NoDeadlocks verifies every state is reachable from the initial state.
func TestValidateTransitions_NoDeadlocks(t *testing.T) {
	table := update.Transitions()

	// Collect all states that appear as a destination.
	reachable := map[update.State]bool{
		update.StatePreUpdate: true, // start
	}
	changed := true
	for changed {
		changed = false
		for _, tr := range table {
			if reachable[tr.From] && !reachable[tr.To] {
				reachable[tr.To] = true
				changed = true
			}
		}
	}

	// Every state referenced in the table must be reachable.
	for _, tr := range table {
		if !reachable[tr.From] {
			t.Errorf("state %s has outgoing transitions but is not reachable from StatePreUpdate", tr.From)
		}
		if !reachable[tr.To] {
			t.Errorf("state %s is a target but not reachable from StatePreUpdate", tr.To)
		}
	}
}

// TestState_StringRoundtrip verifies State string values match their constant declarations.
func TestState_StringRoundtrip(t *testing.T) {
	all := []struct {
		s    update.State
		want string
	}{
		{update.StatePreUpdate, "pre-upgrade"},
		{update.StateSnapshotTree, "snapshot-tree"},
		{update.StateDownloadBinary, "download-binary"},
		{update.StateVerifyCosign, "verify-cosign"},
		{update.StateAtomicReplace, "atomic-replace"},
		{update.StateMigrateTree, "migrate-tree"},
		{update.StateHealthCheck, "health-check"},
		{update.StateCommitted, "committed"},
		{update.StateRollingBack, "rolling-back"},
		{update.StateRestoreTree, "restore-tree"},
		{update.StateRestoreOldBinary, "restore-old-binary"},
		{update.StateVerifyRollback, "verify-rollback"},
		{update.StateRolledBack, "rolled-back"},
		{update.StateFailedUnrecoverable, "failed-unrecoverable"},
	}
	for _, tc := range all {
		if string(tc.s) != tc.want {
			t.Errorf("State %q: got %q, want %q", tc.s, string(tc.s), tc.want)
		}
	}
}

// TestValidateTransitionsTable_OrphanDetected verifies that a table missing transitions
// for a non-terminal state returns an error.
func TestValidateTransitionsTable_OrphanDetected(t *testing.T) {
	// Build a table that covers everything EXCEPT StateRollingBack outgoing transitions.
	// That makes StateRollingBack an orphan (no outgoing edges).
	var reduced []update.Transition
	for _, tr := range update.Transitions() {
		if tr.From != update.StateRollingBack {
			reduced = append(reduced, tr)
		}
	}
	err := update.ValidateTransitionsTable(reduced)
	if err == nil {
		t.Error("ValidateTransitionsTable(table-missing-rollback-outgoing) = nil, want error")
	}
}

// TestValidateTransitionsTable_UnreachableStateDetected verifies that a table with a
// transition whose source state is not reachable from StatePreUpdate returns an error.
func TestValidateTransitionsTable_UnreachableStateDetected(t *testing.T) {
	// Start with the full table and add a transition from a phantom unreachable state.
	phantom := update.State("phantom-unreachable")
	augmented := append(update.Transitions(), update.Transition{
		From:  phantom,
		Event: update.EventSuccess,
		To:    update.StateCommitted,
	})
	err := update.ValidateTransitionsTable(augmented)
	if err == nil {
		t.Error("ValidateTransitionsTable(table-with-unreachable-state) = nil, want error")
	}
}

// TestTransitions_TerminalStatesHaveNoOutgoing verifies terminal states have no outgoing transitions.
func TestTransitions_TerminalStatesHaveNoOutgoing(t *testing.T) {
	table := update.Transitions()
	terminals := []update.State{
		update.StateCommitted,
		update.StateRolledBack,
		update.StateFailedUnrecoverable,
	}
	for _, tr := range table {
		if slices.Contains(terminals, tr.From) {
			t.Errorf("terminal state %s has outgoing transition to %s", tr.From, tr.To)
		}
	}
}
