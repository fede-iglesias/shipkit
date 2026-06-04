package store_test

import (
	"path/filepath"
	"testing"

	"github.com/fede-iglesias/shipkit/store"
)

func TestPathForPerson(t *testing.T) {
	t.Parallel()

	root := "/project"
	got, err := store.PathFor(root, "person", "lena-mueller")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "knowledge", "people", "lena-mueller", "index.md")
	if got != want {
		t.Errorf("PathFor person: got %q, want %q", got, want)
	}
}

func TestPathForADR(t *testing.T) {
	t.Parallel()

	root := "/project"
	got, err := store.PathFor(root, "adr", "use-postgres",
		store.WithADRID("ADR-0042"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ADR-0042 -> strip "ADR-" -> "0042-use-postgres.md"
	want := filepath.Join(root, "knowledge", "decisions", "0042-use-postgres.md")
	if got != want {
		t.Errorf("PathFor adr: got %q, want %q", got, want)
	}
}

func TestPathForADRMissingID(t *testing.T) {
	t.Parallel()

	_, err := store.PathFor("/project", "adr", "use-postgres")
	if err == nil {
		t.Error("expected error when ADR ID is missing")
	}
}

func TestPathForTask(t *testing.T) {
	t.Parallel()

	root := "/project"
	got, err := store.PathFor(root, "task", "fix-login",
		store.WithTaskID("TASK-2026-0001"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// TASK-2026-0001 -> year=2026, seq=0001
	want := filepath.Join(root, "knowledge", "tasks", "2026", "0001-fix-login.md")
	if got != want {
		t.Errorf("PathFor task: got %q, want %q", got, want)
	}
}

func TestPathForTaskMissingID(t *testing.T) {
	t.Parallel()

	_, err := store.PathFor("/project", "task", "fix-login")
	if err == nil {
		t.Error("expected error when task ID is missing")
	}
}

func TestPathForMeeting(t *testing.T) {
	t.Parallel()

	root := "/project"
	got, err := store.PathFor(root, "meeting", "standup",
		store.WithMeetingID("2026-06-03-abcd1234"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "knowledge", "docs", "meetings", "2026-06-03-abcd1234.md")
	if got != want {
		t.Errorf("PathFor meeting: got %q, want %q", got, want)
	}
}

func TestPathForMeetingMissingID(t *testing.T) {
	t.Parallel()

	_, err := store.PathFor("/project", "meeting", "standup")
	if err == nil {
		t.Error("expected error when meeting ID is missing")
	}
}

func TestPathForPlan(t *testing.T) {
	t.Parallel()

	root := "/project"
	got, err := store.PathFor(root, "plan", "q3-roadmap")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "knowledge", "plans", "q3-roadmap.md")
	if got != want {
		t.Errorf("PathFor plan: got %q, want %q", got, want)
	}
}

func TestPathForDoc(t *testing.T) {
	t.Parallel()

	root := "/project"
	got, err := store.PathFor(root, "doc", "onboarding")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "knowledge", "docs", "onboarding.md")
	if got != want {
		t.Errorf("PathFor doc: got %q, want %q", got, want)
	}
}

func TestPathForResearch(t *testing.T) {
	t.Parallel()

	root := "/project"
	got, err := store.PathFor(root, "research", "flink-vs-kafka")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "knowledge", "research", "flink-vs-kafka.md")
	if got != want {
		t.Errorf("PathFor research: got %q, want %q", got, want)
	}
}

func TestPathForReport(t *testing.T) {
	t.Parallel()

	root := "/project"
	got, err := store.PathFor(root, "report", "perf-2026-q1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "knowledge", "reports", "perf-2026-q1.md")
	if got != want {
		t.Errorf("PathFor report: got %q, want %q", got, want)
	}
}

func TestPathForIncident(t *testing.T) {
	t.Parallel()

	root := "/project"
	got, err := store.PathFor(root, "incident", "db-outage-june")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "knowledge", "incidents", "db-outage-june.md")
	if got != want {
		t.Errorf("PathFor incident: got %q, want %q", got, want)
	}
}

func TestPathForUnknownKind(t *testing.T) {
	t.Parallel()

	_, err := store.PathFor("/project", "unknown-kind", "some-slug")
	if err == nil {
		t.Error("expected error for unknown kind")
	}
}

func TestKindFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rel     string
		want    string
		wantErr bool
	}{
		{"knowledge/people/lena-mueller/index.md", "person", false},
		{"knowledge/decisions/0042-use-postgres.md", "adr", false},
		{"knowledge/tasks/2026/0001-fix-login.md", "task", false},
		{"knowledge/docs/meetings/2026-06-03-abcd1234.md", "meeting", false},
		{"knowledge/plans/q3-roadmap.md", "plan", false},
		{"knowledge/docs/onboarding.md", "doc", false},
		{"knowledge/research/flink-vs-kafka.md", "research", false},
		{"knowledge/reports/perf-2026-q1.md", "report", false},
		{"knowledge/incidents/db-outage-june.md", "incident", false},
		{"some/unknown/path.md", "", true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.rel, func(t *testing.T) {
			t.Parallel()
			got, err := store.KindFromPath(tc.rel)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for path %q", tc.rel)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("KindFromPath(%q) = %q, want %q", tc.rel, got, tc.want)
			}
		})
	}
}
