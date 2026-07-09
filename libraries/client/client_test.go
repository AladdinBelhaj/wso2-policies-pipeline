package client

import (
	"reflect"
	"strings"
	"testing"
)

func TestFilterApiIdsByName(t *testing.T) {
	apis := []ApiSummary{
		{ID: "api-1", Name: "Customer API"},
		{ID: "api-2", Name: "Order API"},
	}

	got := FilterApiIdsByName(apis, "customer api")
	want := []string{"api-1"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterApiIdsByName() = %v, want %v", got, want)
	}
}

func TestBuildRollbackPreview(t *testing.T) {
	preview := BuildRollbackPreview([]string{"api-1", "api-2"}, "customer")

	if !strings.Contains(preview, "2 API(s)") {
		t.Fatalf("preview should include the API count, got %q", preview)
	}
	if !strings.Contains(preview, "customer") {
		t.Fatalf("preview should include the target filter, got %q", preview)
	}
}

func TestResolveRollbackTargetsForTwoRevisions(t *testing.T) {
	target, remove, ok := ResolveRollbackTargets([]string{"rev-1", "rev-2"})
	if !ok {
		t.Fatalf("expected rollback targets to be resolved")
	}
	if target != "rev-1" {
		t.Fatalf("expected rollback target rev-1, got %s", target)
	}
	if remove != "rev-2" {
		t.Fatalf("expected revision to remove rev-2, got %s", remove)
	}
}
