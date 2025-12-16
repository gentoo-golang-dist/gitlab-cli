package api

import (
	"encoding/json"
	"testing"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func TestScanAndCollect_EmptyResults(t *testing.T) {
	// Test that empty results return an empty slice instead of nil
	result, err := ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]string, *gitlab.Response, error) {
		// Return empty slice to simulate no results
		return []string{}, &gitlab.Response{}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil slice, got nil")
	}

	if len(result) != 0 {
		t.Fatalf("expected empty slice with length 0, got length %d", len(result))
	}

	// Verify JSON marshaling produces [] instead of null
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	expectedJSON := "[]"
	actualJSON := string(jsonBytes)
	if actualJSON != expectedJSON {
		t.Errorf("expected JSON %q, got %q", expectedJSON, actualJSON)
	}
}

func TestScanAndCollect_WithResults(t *testing.T) {
	// Test that results with data are returned correctly
	testData := []string{"item1", "item2", "item3"}
	result, err := ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]string, *gitlab.Response, error) {
		return testData, &gitlab.Response{}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != len(testData) {
		t.Fatalf("expected %d items, got %d", len(testData), len(result))
	}

	for i, item := range result {
		if item != testData[i] {
			t.Errorf("expected item %d to be %q, got %q", i, testData[i], item)
		}
	}
}

func TestScanAndCollect_Error(t *testing.T) {
	// Test that errors are properly propagated
	expectedErr := gitlab.ErrNotFound
	result, err := ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]string, *gitlab.Response, error) {
		return nil, nil, expectedErr
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if result != nil {
		t.Errorf("expected nil result on error, got %v", result)
	}
}

func TestScanAndCollect_JSONMarshaling_Comparison(t *testing.T) {
	// Demonstrate the difference between nil slice and empty slice in JSON
	type Response struct {
		Items []string `json:"items"`
	}

	// nil slice case (what gitlab.ScanAndCollect returns)
	nilResponse := Response{Items: nil}
	nilJSON, _ := json.Marshal(nilResponse)

	// empty slice case (what our wrapper returns)
	emptyResponse := Response{Items: make([]string, 0)}
	emptyJSON, _ := json.Marshal(emptyResponse)

	// Verify they're different
	if string(nilJSON) == string(emptyJSON) {
		t.Error("nil slice and empty slice should produce different JSON")
	}

	// nil slice produces: {"items":null}
	expectedNil := `{"items":null}`
	if string(nilJSON) != expectedNil {
		t.Errorf("nil slice JSON: expected %q, got %q", expectedNil, string(nilJSON))
	}

	// empty slice produces: {"items":[]}
	expectedEmpty := `{"items":[]}`
	if string(emptyJSON) != expectedEmpty {
		t.Errorf("empty slice JSON: expected %q, got %q", expectedEmpty, string(emptyJSON))
	}
}
