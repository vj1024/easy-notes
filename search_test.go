package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSearchDefaultsToFileNamesAndSlashEnablesContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	files := map[string]string{
		"mysql-guide.md": "database notes",
		"notes.txt":      "connect to mysql here",
		"unrelated.txt":  "nothing to see",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name      string
		term      string
		wantFiles []string
	}{
		{name: "file name only", term: "mysql", wantFiles: []string{"mysql-guide.md"}},
		{name: "file name and content", term: "/mysql", wantFiles: []string{"mysql-guide.md", "notes.txt"}},
		{name: "empty content term", term: "/", wantFiles: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			performSearch(context, root, test.term)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			var result DirectoryResponse
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			gotFiles := collectSearchFileNames(result.Results)
			sort.Strings(gotFiles)
			sort.Strings(test.wantFiles)
			if len(gotFiles) != len(test.wantFiles) {
				t.Fatalf("files = %v, want %v", gotFiles, test.wantFiles)
			}
			for i := range gotFiles {
				if gotFiles[i] != test.wantFiles[i] {
					t.Fatalf("files = %v, want %v", gotFiles, test.wantFiles)
				}
			}
		})
	}
}

func collectSearchFileNames(results []*SearchResult) []string {
	var names []string
	for _, result := range results {
		if result.Type == typeFile {
			names = append(names, result.Text)
		} else {
			names = append(names, collectSearchFileNames(result.Children)...)
		}
	}
	return names
}
