package client

import (
	"reflect"
	"testing"
)

func TestFilterApiIdsByName_ExactMatchOnly(t *testing.T) {
	apis := []ApiSummary{
		{ID: "api-1", Name: "Orders"},
		{ID: "api-2", Name: "Billing"},
	}

	got := FilterApiIdsByName(apis, "Order")
	want := []string{}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterApiIdsByName() = %v, want %v", got, want)
	}
}
