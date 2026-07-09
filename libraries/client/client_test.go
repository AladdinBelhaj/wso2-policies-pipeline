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
