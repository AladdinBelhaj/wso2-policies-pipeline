package client

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCollectApiProductRefs(t *testing.T) {
	testProduct := map[string]any{
		"apis": []any{
			map[string]any{"apiId": "api-1"},
			map[string]any{"apiId": "api-2"},
		},
	}
	payload, err := json.Marshal(testProduct)
	if err != nil {
		t.Fatalf("marshal product payload: %v", err)
	}

	summaries := []ApiProductSummary{{ID: "prod-1", Name: "Prod One"}}
	refs := collectApiProductRefsFromSummaries([]string{"api-1"}, summaries, func(productID string) ([]byte, error) {
		if productID != "prod-1" {
			t.Fatalf("unexpected product lookup %q", productID)
		}
		return payload, nil
	}, func(productID string, matched []string) ApiProductRef {
		return ApiProductRef{ProductID: productID, ApiIDs: matched}
	})

	want := []ApiProductRef{{ProductID: "prod-1", ApiIDs: []string{"api-1"}}}
	if !reflect.DeepEqual(refs, want) {
		t.Fatalf("collectApiProductRefsFromSummaries() = %#v, want %#v", refs, want)
	}
}
