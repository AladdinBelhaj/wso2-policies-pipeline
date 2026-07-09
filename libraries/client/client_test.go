package client

import (
	"reflect"
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
