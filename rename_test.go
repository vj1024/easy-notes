package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleRenameFileAndDirectory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousBaseDir := baseDir
	baseDir = t.TempDir()
	t.Cleanup(func() { baseDir = previousBaseDir })

	if err := os.WriteFile(filepath.Join(baseDir, "old.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(baseDir, "old-folder"), 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path    string
		newName string
		want    string
	}{
		{path: "old.txt", newName: "new.txt", want: "new.txt"},
		{path: "old-folder", newName: "new-folder", want: "new-folder"},
	}
	for _, test := range tests {
		body, err := json.Marshal(RenameRequest{Path: test.path, NewName: test.newName})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/rename", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(response)
		context.Request = request

		handleRename(context)
		if response.Code != http.StatusOK {
			t.Fatalf("rename %q status = %d, body = %s", test.path, response.Code, response.Body.String())
		}
		if _, err := os.Stat(filepath.Join(baseDir, test.want)); err != nil {
			t.Fatalf("renamed path %q: %v", test.want, err)
		}
		if _, err := os.Stat(filepath.Join(baseDir, test.path)); !os.IsNotExist(err) {
			t.Fatalf("old path %q still exists", test.path)
		}
	}
}

func TestHandleRenameRejectsPathInName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousBaseDir := baseDir
	baseDir = t.TempDir()
	t.Cleanup(func() { baseDir = previousBaseDir })
	if err := os.WriteFile(filepath.Join(baseDir, "old.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	body := []byte(`{"path":"old.txt","newName":"../escape.txt"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/rename", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request
	handleRename(context)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
