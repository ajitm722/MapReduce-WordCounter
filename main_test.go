package main

import (
	"os"
	"runtime"
	"testing"
)

func TestProcessFiles(t *testing.T) {
	// Prepare temporary files to test
	testFile1 := "testfile1.txt"
	testFile2 := "testfile2.txt"

	content := "apple orange! banana? apple.\n banana apple: apple. banana..."

	if err := os.WriteFile(testFile1, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file1: %v", err)
	}
	defer os.Remove(testFile1)

	if err := os.WriteFile(testFile2, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file2: %v", err)
	}
	defer os.Remove(testFile2)
	// Process the files
	files := []string{testFile1, testFile2}
	result, err := processFiles(files, runtime.NumCPU())

	if err != nil {
		t.Fatalf("Error processing files: %v", err)
	}

	// Validate the result
	expectedResult := map[string]int{
		"apple":  8,
		"orange": 2,
		"banana": 6,
	}
	printResult(result)

	for word, expectedCount := range expectedResult {
		if count, found := result[word]; !found || count != expectedCount {
			t.Errorf("For word '%s: expected count %d, but got %d", word, expectedCount, count)
		}
	}
}
