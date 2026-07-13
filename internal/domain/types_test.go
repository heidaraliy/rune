package domain

import (
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
