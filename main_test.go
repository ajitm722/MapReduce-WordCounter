package main

import (
	"os"
	"runtime"
	"testing"
)

func TestProcessFiles(t *testing.T) {
	// Prepare a temporary file to test
	testFile := "testfile.txt"
	content := "apple orange banana apple banana apple apple banana"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Process the files
	files := []string{testFile}
	result, err := processFiles(files, runtime.NumCPU())

	if err != nil {
		t.Fatalf("Error processing files: %v", err)
	}

	// Validate the result
	expectedResult := map[string]int{
		"apple":  4,
		"orange": 1,
		"banana": 3,
	}

	for word, expectedCount := range expectedResult {
		if count, found := result[word]; !found || count != expectedCount {
			t.Errorf("For word '%s: expected count %d, but got %d", word, expectedCount, count)
		}
	}
}
