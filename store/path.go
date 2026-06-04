package store

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Directory constants for the knowledge layout.
const (
	// KnowledgeDir is the subdirectory under a project root that holds all
	// knowledge cards. Every card path is rooted at <root>/KnowledgeDir.
	KnowledgeDir = "knowledge"

	// ArchiveDir is the subdirectory under KnowledgeDir where deleted cards
	// are moved by MoveToArchive.
	ArchiveDir = "archive"
)

// pathOptions carries optional identifiers for kinds that require extra context
// to compute their path (adr, task, meeting).
type pathOptions struct {
	ADRID     string // e.g. "ADR-0042"
	TaskID    string // e.g. "TASK-2026-0001"
	MeetingID string // e.g. "2026-06-03-abcd1234"
}

// PathOption is a functional option for PathFor.
type PathOption func(*pathOptions)

// WithADRID provides the ADR identifier (e.g. "ADR-0042") needed to
// compute the ADR file path. Required for kind "adr".
func WithADRID(id string) PathOption {
	return func(o *pathOptions) { o.ADRID = id }
}

// WithTaskID provides the task identifier (e.g. "TASK-2026-0001") needed
// to compute the task file path. Required for kind "task".
func WithTaskID(id string) PathOption {
	return func(o *pathOptions) { o.TaskID = id }
}

// WithMeetingID provides the meeting identifier (e.g. "2026-06-03-abcd1234")
// needed to compute the meeting file path. Required for kind "meeting".
func WithMeetingID(id string) PathOption {
	return func(o *pathOptions) { o.MeetingID = id }
}

// PathFor returns the absolute filesystem path for a card given its root,
// kind, slug, and any required PathOptions. Returns an error for unknown kinds
// or when a required option is missing.
//
// Path layout per kind:
//
//	person:   <root>/knowledge/people/<slug>/index.md
//	adr:      <root>/knowledge/decisions/<seq>-<slug>.md  (WithADRID required)
//	task:     <root>/knowledge/tasks/<year>/<seq>-<slug>.md (WithTaskID required)
//	meeting:  <root>/knowledge/docs/meetings/<meeting_id>.md (WithMeetingID required)
//	plan:     <root>/knowledge/plans/<slug>.md
//	doc:      <root>/knowledge/docs/<slug>.md
//	research: <root>/knowledge/research/<slug>.md
//	report:   <root>/knowledge/reports/<slug>.md
//	incident: <root>/knowledge/incidents/<slug>.md
//
// Returns:
//   - The absolute path on success.
//   - An error wrapping details when the kind is unknown or a required option is absent.
func PathFor(root, kind, slug string, opts ...PathOption) (string, error) {
	o := &pathOptions{}
	for _, fn := range opts {
		fn(o)
	}

	kdir := filepath.Join(root, KnowledgeDir)

	switch kind {
	case "person":
		return personPath(kdir, slug), nil
	case "adr":
		return adrPath(kdir, slug, o.ADRID)
	case "task":
		return taskPath(kdir, slug, o.TaskID)
	case "meeting":
		return meetingPath(kdir, o.MeetingID)
	case "plan":
		return filepath.Join(kdir, "plans", slug+".md"), nil
	case "doc":
		return filepath.Join(kdir, "docs", slug+".md"), nil
	case "research":
		return filepath.Join(kdir, "research", slug+".md"), nil
	case "report":
		return filepath.Join(kdir, "reports", slug+".md"), nil
	case "incident":
		return filepath.Join(kdir, "incidents", slug+".md"), nil
	default:
		return "", fmt.Errorf("store: unknown card kind %q", kind)
	}
}

// personPath returns <kdir>/people/<slug>/index.md.
func personPath(kdir, slug string) string {
	return filepath.Join(kdir, "people", slug, "index.md")
}

// adrPath returns <kdir>/decisions/<seq>-<slug>.md.
// adrID must match "ADR-NNNN"; the file uses "NNNN" as prefix.
func adrPath(kdir, slug, adrID string) (string, error) {
	if adrID == "" {
		return "", fmt.Errorf("store: ADR kind requires WithADRID option")
	}
	// adrID is "ADR-NNNN" - strip the "ADR-" prefix.
	if !strings.HasPrefix(adrID, "ADR-") {
		return "", fmt.Errorf("store: invalid ADR ID %q (must start with ADR-)", adrID)
	}
	seq := strings.TrimPrefix(adrID, "ADR-")
	return filepath.Join(kdir, "decisions", seq+"-"+slug+".md"), nil
}

// taskPath returns <kdir>/tasks/<year>/<seq>-<slug>.md.
// taskID must match "TASK-YYYY-NNNN".
func taskPath(kdir, slug, taskID string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("store: task kind requires WithTaskID option")
	}
	// taskID is "TASK-YYYY-NNNN"
	var year, seq string
	if _, err := fmt.Sscanf(taskID, "TASK-%4s-%4s", &year, &seq); err != nil || len(year) != 4 || len(seq) != 4 {
		return "", fmt.Errorf("store: invalid task ID %q (must match TASK-YYYY-NNNN)", taskID)
	}
	return filepath.Join(kdir, "tasks", year, seq+"-"+slug+".md"), nil
}

// meetingPath returns <kdir>/docs/meetings/<meeting_id>.md.
func meetingPath(kdir, meetingID string) (string, error) {
	if meetingID == "" {
		return "", fmt.Errorf("store: meeting kind requires WithMeetingID option")
	}
	return filepath.Join(kdir, "docs", "meetings", meetingID+".md"), nil
}

// KindFromPath reverse-maps a relative path (from the project root) to a card kind.
// The rel path must start with "knowledge/" to be recognized.
//
// Returns:
//   - The kind string (e.g. "person", "adr", "task") on success.
//   - An error when the path is not under "knowledge/" or does not match any known subdirectory.
func KindFromPath(rel string) (string, error) {
	// Normalize to forward slashes for matching.
	rel = filepath.ToSlash(rel)

	// Strip trailing .md and identify by prefix after "knowledge/".
	if !strings.HasPrefix(rel, "knowledge/") {
		return "", fmt.Errorf("store: path %q is not under knowledge/", rel)
	}
	rest := strings.TrimPrefix(rel, "knowledge/")

	switch {
	case strings.HasPrefix(rest, "people/"):
		return "person", nil
	case strings.HasPrefix(rest, "decisions/"):
		return "adr", nil
	case strings.HasPrefix(rest, "tasks/"):
		return "task", nil
	case strings.HasPrefix(rest, "docs/meetings/"):
		return "meeting", nil
	case strings.HasPrefix(rest, "plans/"):
		return "plan", nil
	case strings.HasPrefix(rest, "docs/"):
		return "doc", nil
	case strings.HasPrefix(rest, "research/"):
		return "research", nil
	case strings.HasPrefix(rest, "reports/"):
		return "report", nil
	case strings.HasPrefix(rest, "incidents/"):
		return "incident", nil
	default:
		return "", fmt.Errorf("store: cannot determine card kind from path %q", rel)
	}
}
