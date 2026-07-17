package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewIDIsUUIDShapedAndDisplayable(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 36 || strings.Count(id, "-") != 4 {
		t.Fatalf("id = %q, want UUID shape", id)
	}
	if got := DisplayID(id); got != id[:8] {
		t.Fatalf("display id = %q, want %q", got, id[:8])
	}
}

func TestEntityValidationSeparatesNotesAndTasks(t *testing.T) {
	base := Entity{ID: "id", Kind: KindNote, WorkspaceID: "local", Title: "thing"}
	if base.Validate() != nil {
		t.Fatal("note should validate")
	}
	base.Kind = KindTask
	base.Status = StatusReady
	if base.Validate() != nil {
		t.Fatal("task should validate")
	}
	base.Status = Status("wat")
	if base.Validate() == nil {
		t.Fatal("unknown task status should fail")
	}
	base.Kind = KindNote
	base.Status = StatusReady
	if base.Validate() == nil {
		t.Fatal("note task status should fail")
	}
}

func TestRuneContractExposesFacetsAndStableReferences(t *testing.T) {
	rune := Rune{ID: "abc123", Kind: KindTask, WorkspaceID: "local", Title: "ship"}
	if got := rune.Reference(); got != "rune://abc123" {
		t.Fatalf("Rune reference = %q", got)
	}
	if got := rune.Facets(); len(got) != 2 || got[0] != FacetDocument || got[1] != FacetTask {
		t.Fatalf("Rune facets = %#v", got)
	}

	payload, err := json.Marshal(rune)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"kind":"task"`, `"ref":"rune://abc123"`, `"facets":["document","task"]`} {
		if !strings.Contains(string(payload), want) {
			t.Fatalf("Rune JSON %q missing %s", payload, want)
		}
	}

	note := Rune{ID: "note", Kind: KindNote, WorkspaceID: "local", Title: "idea"}
	if got := note.Facets(); len(got) != 1 || got[0] != FacetDocument {
		t.Fatalf("note facets = %#v", got)
	}
}

func TestRuneNormalizePersistsFacetsAndCanonicalState(t *testing.T) {
	task := Rune{ID: "task", Kind: KindTask, WorkspaceID: "local", Title: "ship", State: StateReady}
	if err := task.Normalize(); err != nil {
		t.Fatal(err)
	}
	if task.State != StateReady || task.Status != StatusReady || !task.HasFacet(FacetTask) {
		t.Fatalf("normalized task = %#v", task)
	}
	if got := strings.Join([]string{string(task.Facets()[0]), string(task.Facets()[1])}, ","); got != "document,task" {
		t.Fatalf("task facets = %q", got)
	}

	note := Rune{ID: "note", Kind: KindNote, WorkspaceID: "local", Title: "work note", State: StateInProgress}
	if err := note.Normalize(); err != nil {
		t.Fatal(err)
	}
	if note.State != StateInProgress || note.Status != "" || note.IsTask() {
		t.Fatalf("normalized note = %#v", note)
	}

	payload, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip Rune
	if err := json.Unmarshal(payload, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if err := roundTrip.Normalize(); err != nil {
		t.Fatal(err)
	}
	if roundTrip.State != StateReady || !roundTrip.HasFacet(FacetTask) {
		t.Fatalf("round trip Rune = %#v", roundTrip)
	}
}

func TestNormalizeRuneStateSupportsCanonicalAliases(t *testing.T) {
	for _, test := range []struct {
		input string
		want  RuneState
	}{
		{input: "in-progress", want: StateInProgress},
		{input: "completed", want: StateComplete},
		{input: "ready", want: StateReady},
	} {
		got, err := NormalizeRuneState(test.input)
		if err != nil || got != test.want {
			t.Fatalf("NormalizeRuneState(%q) = %q, %v; want %q", test.input, got, err, test.want)
		}
	}
	if _, err := NormalizeRuneState("unknown"); err == nil {
		t.Fatal("unknown Rune state should fail")
	}
}

func TestResolveRuneIDAcceptsStableAndShortForms(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{input: "abc123", want: "abc123"},
		{input: " rune://abc123 ", want: "abc123"},
		{input: "", want: ""},
	} {
		got, err := ResolveRuneID(test.input)
		if err != nil || got != test.want {
			t.Fatalf("ResolveRuneID(%q) = %q, %v; want %q", test.input, got, err, test.want)
		}
	}
	for _, input := range []string{"http://abc123", "rune://", "rune://abc/child"} {
		if _, err := ResolveRuneID(input); err == nil {
			t.Fatalf("ResolveRuneID(%q) should fail", input)
		}
	}
}

func TestNormalizeRuneSortUsesStableNames(t *testing.T) {
	for _, test := range []struct {
		input string
		want  RuneSortField
	}{
		{input: "updated_at", want: RuneSortUpdatedAt},
		{input: "created-at", want: RuneSortCreatedAt},
		{input: "sibling_order", want: RuneSortOrder},
	} {
		got, err := NormalizeRuneSort(test.input)
		if err != nil || got != test.want {
			t.Fatalf("NormalizeRuneSort(%q) = %q, %v; want %q", test.input, got, err, test.want)
		}
	}
	if _, err := NormalizeRuneSort("made-up"); err == nil {
		t.Fatal("unknown Rune sort should fail")
	}
}

func TestLinkValidationRejectsInvalidEdges(t *testing.T) {
	link := Link{ID: "link", WorkspaceID: "local", FromID: "a", ToID: "b", Kind: "references"}
	if err := link.Validate(); err != nil {
		t.Fatal(err)
	}
	link.FromID = link.ToID
	if link.Validate() == nil {
		t.Fatal("self-link should fail")
	}
}

func TestNormalizeTagsDeduplicatesWithoutSortingContent(t *testing.T) {
	got := NormalizeTags([]string{"#ux", "ux", "agent"})
	if strings.Join(got, ",") != "ux,agent" {
		t.Fatalf("tags = %#v", got)
	}
}

func TestRunLifecycleAndArtifactValidation(t *testing.T) {
	run := Run{ID: "run", WorkspaceID: "local", TaskID: "task", Provider: "fake", Status: RunStatusQueued, PermissionPolicy: PermissionReadOnly, Revision: 1}
	if err := run.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, transition := range [][2]RunStatus{
		{RunStatusQueued, RunStatusRunning},
		{RunStatusRunning, RunStatusReview},
		{RunStatusReview, RunStatusCompleted},
	} {
		if !CanTransitionRun(transition[0], transition[1]) {
			t.Fatalf("transition %s -> %s should be valid", transition[0], transition[1])
		}
	}
	if CanTransitionRun(RunStatusCompleted, RunStatusRunning) {
		t.Fatal("completed run should be terminal")
	}
	artifact := Artifact{ID: "artifact", WorkspaceID: "local", Kind: "result", Name: "result.md", MediaType: "text/markdown", SizeBytes: 10, SHA256: strings.Repeat("a", 64), StorageKey: "sha256/aa/" + strings.Repeat("a", 64), Retention: "normal", SecretState: "clear"}
	if err := artifact.Validate(); err != nil {
		t.Fatal(err)
	}
	artifact.SecretState = "secret"
	if err := artifact.Validate(); err == nil {
		t.Fatal("unknown secret state should fail")
	}
}
