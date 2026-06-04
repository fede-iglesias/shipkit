package update

import "fmt"

// State represents a step in the update or rollback lifecycle.
type State string

// State constants - forward path then rollback path then terminal states.
const (
	// Forward path.
	StatePreUpdate      State = "pre-upgrade"
	StateSnapshotTree   State = "snapshot-tree"
	StateDownloadBinary State = "download-binary"
	StateVerifyCosign   State = "verify-cosign"
	StateAtomicReplace  State = "atomic-replace"
	StateMigrateTree    State = "migrate-tree"
	StateHealthCheck    State = "health-check"
	StateCommitted      State = "committed"

	// Rollback path.
	StateRollingBack      State = "rolling-back"
	StateRestoreTree      State = "restore-tree"
	StateRestoreOldBinary State = "restore-old-binary"
	StateVerifyRollback   State = "verify-rollback"

	// Terminal states.
	StateRolledBack          State = "rolled-back"
	StateFailedUnrecoverable State = "failed-unrecoverable"
)

// Event represents a trigger that drives a state transition.
type Event string

const (
	// EventSuccess drives a forward transition on step completion.
	EventSuccess Event = "success"
	// EventFailure drives entry into the rollback path on step failure.
	EventFailure Event = "failure"
	// EventCancel drives entry into the rollback path on context cancellation.
	EventCancel Event = "cancel"
)

// Transition is a single row in the state machine table.
type Transition struct {
	// From is the source state.
	From State
	// Event is the trigger.
	Event Event
	// To is the destination state.
	To State
}

// Transitions returns the complete transition table for the update state machine.
//
// Forward path: each state can succeed (to the next forward state) or fail
// (entering the rollback path). Terminal states have no outgoing transitions.
//
// Rollback path: once rolling-back is entered, each step can succeed (moving
// forward through rollback) or fail (entering failed-unrecoverable).
func Transitions() []Transition {
	return []Transition{
		// Forward path - success edges.
		{From: StatePreUpdate, Event: EventSuccess, To: StateSnapshotTree},
		{From: StateSnapshotTree, Event: EventSuccess, To: StateDownloadBinary},
		{From: StateDownloadBinary, Event: EventSuccess, To: StateVerifyCosign},
		{From: StateVerifyCosign, Event: EventSuccess, To: StateAtomicReplace},
		{From: StateAtomicReplace, Event: EventSuccess, To: StateMigrateTree},
		{From: StateMigrateTree, Event: EventSuccess, To: StateHealthCheck},
		{From: StateHealthCheck, Event: EventSuccess, To: StateCommitted},

		// Forward path - failure edges (enter rollback).
		{From: StatePreUpdate, Event: EventFailure, To: StateRollingBack},
		{From: StateSnapshotTree, Event: EventFailure, To: StateRollingBack},
		{From: StateDownloadBinary, Event: EventFailure, To: StateRollingBack},
		{From: StateVerifyCosign, Event: EventFailure, To: StateRollingBack},
		{From: StateAtomicReplace, Event: EventFailure, To: StateRollingBack},
		{From: StateMigrateTree, Event: EventFailure, To: StateRollingBack},
		{From: StateHealthCheck, Event: EventFailure, To: StateRollingBack},

		// Forward path - cancel edges (enter rollback).
		{From: StatePreUpdate, Event: EventCancel, To: StateRollingBack},
		{From: StateSnapshotTree, Event: EventCancel, To: StateRollingBack},
		{From: StateDownloadBinary, Event: EventCancel, To: StateRollingBack},
		{From: StateVerifyCosign, Event: EventCancel, To: StateRollingBack},

		// Rollback path - success edges.
		{From: StateRollingBack, Event: EventSuccess, To: StateRestoreTree},
		{From: StateRestoreTree, Event: EventSuccess, To: StateRestoreOldBinary},
		{From: StateRestoreOldBinary, Event: EventSuccess, To: StateVerifyRollback},
		{From: StateVerifyRollback, Event: EventSuccess, To: StateRolledBack},

		// Rollback path - failure edges (unrecoverable).
		{From: StateRollingBack, Event: EventFailure, To: StateFailedUnrecoverable},
		{From: StateRestoreTree, Event: EventFailure, To: StateFailedUnrecoverable},
		{From: StateRestoreOldBinary, Event: EventFailure, To: StateFailedUnrecoverable},
		{From: StateVerifyRollback, Event: EventFailure, To: StateFailedUnrecoverable},
	}
}

// stateOrderMap maps each State to a monotonically increasing integer.
// Forward path: 1-8. Rollback path: 10-14. Unknown: -1.
var stateOrderMap = map[State]int{
	// Forward path (1-8).
	StatePreUpdate:      1,
	StateSnapshotTree:   2,
	StateDownloadBinary: 3,
	StateVerifyCosign:   4,
	StateAtomicReplace:  5,
	StateMigrateTree:    6,
	StateHealthCheck:    7,
	StateCommitted:      8,

	// Rollback path (10-14).
	StateRollingBack:      10,
	StateRestoreTree:      11,
	StateRestoreOldBinary: 12,
	StateVerifyRollback:   13,
	StateRolledBack:       14,

	// Unrecoverable terminal (15).
	StateFailedUnrecoverable: 15,
}

// StateOrder returns the ordering integer for state s (1-N for known states, -1 for unknown).
// Used by the orchestrator to compare phase progression and determine rollback scope.
func StateOrder(s State) int {
	if n, ok := stateOrderMap[s]; ok {
		return n
	}
	return -1
}

// terminalStates is the set of states that have no outgoing transitions.
var terminalStates = map[State]bool{
	StateCommitted:           true,
	StateRolledBack:          true,
	StateFailedUnrecoverable: true,
}

// IsTerminal returns true if s is a terminal state (committed, rolled-back, or failed-unrecoverable).
// Terminal states have no outgoing transitions; the state machine stops here.
func IsTerminal(s State) bool {
	return terminalStates[s]
}

// forwardPathStates is the set of states that belong to the forward (happy-path) branch.
var forwardPathStates = map[State]bool{
	StatePreUpdate:      true,
	StateSnapshotTree:   true,
	StateDownloadBinary: true,
	StateVerifyCosign:   true,
	StateAtomicReplace:  true,
	StateMigrateTree:    true,
	StateHealthCheck:    true,
	StateCommitted:      true,
}

// IsForwardPath returns true if s belongs to the forward (non-rollback) path.
// Note: StateCommitted is considered forward even though it is terminal.
func IsForwardPath(s State) bool {
	return forwardPathStates[s]
}

// ValidateTransitions performs a structural integrity check of the canonical
// transition table (Transitions()). It is a convenience wrapper around
// ValidateTransitionsTable that is intended to be called at init time or in tests.
func ValidateTransitions() error {
	return ValidateTransitionsTable(Transitions())
}

// ValidateTransitionsTable performs a structural integrity check of the given
// transition table. It verifies:
//  1. No non-terminal state is an orphan (has at least one outgoing transition).
//  2. Every state referenced as a source is reachable from StatePreUpdate.
//
// Accepting a table parameter makes the error paths testable without mutating
// package-level state.
func ValidateTransitionsTable(table []Transition) error {
	// Build adjacency: which states have at least one outgoing transition.
	hasOutgoing := make(map[State]bool)
	for _, tr := range table {
		hasOutgoing[tr.From] = true
	}

	// All known non-terminal states must have at least one outgoing transition.
	allStates := []State{
		StatePreUpdate, StateSnapshotTree, StateDownloadBinary, StateVerifyCosign,
		StateAtomicReplace, StateMigrateTree, StateHealthCheck,
		StateRollingBack, StateRestoreTree, StateRestoreOldBinary, StateVerifyRollback,
		// Terminal states intentionally excluded.
	}
	for _, s := range allStates {
		if !hasOutgoing[s] {
			return fmt.Errorf("state %q is not terminal but has no outgoing transitions (orphan)", s)
		}
	}

	// BFS reachability from StatePreUpdate.
	reachable := map[State]bool{StatePreUpdate: true}
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

	// Every state used as a source must be reachable.
	for _, tr := range table {
		if !reachable[tr.From] {
			return fmt.Errorf("state %q has transitions but is not reachable from %q", tr.From, StatePreUpdate)
		}
	}

	return nil
}
