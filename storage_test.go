package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResolveSafePath(t *testing.T) {
	root := t.TempDir()

	path, err := resolveSafePath(root, "notes/today.md")
	if err != nil {
		t.Fatalf("resolve normal path: %v", err)
	}
	want := filepath.Join(root, "notes", "today.md")
	if path != want {
		t.Fatalf("resolved path = %q, want %q", path, want)
	}

	if _, err := resolveSafePath(root, "../outside.txt"); err == nil {
		t.Fatal("path traversal was accepted")
	}
}

func TestResolveSafePathRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "outside")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	_, err := resolveSafePath(root, "outside/secret.txt")
	if !errors.Is(err, errSymlinkPath) {
		t.Fatalf("resolve symlink path error = %v, want errSymlinkPath", err)
	}
}

type failingReader struct {
	sent bool
}

func (r *failingReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, "partial"), nil
	}
	return 0, errors.New("simulated read failure")
}

func TestAtomicWritePreservesDestinationOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := atomicWrite(path, &failingReader{}, 0644); err == nil {
		t.Fatal("atomicWrite unexpectedly succeeded")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original" {
		t.Fatalf("destination content = %q, want original", content)
	}
}

func TestAtomicWriteReplacesDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(path, strings.NewReader("new"), 0644); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("destination content = %q, want new", content)
	}
}

func TestLimitRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(LimitRequestBody(4))
	router.POST("/", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		var maxBytesError *http.MaxBytesError
		if !errors.As(err, &maxBytesError) {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusRequestEntityTooLarge)
	})

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestEmbeddedEditorAssetsAreServedLocally(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newRouter()

	for _, path := range []string{
		"/editor",
		"/assets/css/editor.css",
		"/assets/js/editor.js",
		"/assets/vendor/jquery/jquery.min.js",
		"/assets/vendor/jstree/jstree.min.js",
		"/assets/vendor/vditor/dist/index.min.js",
		"/assets/vendor/vditor/dist/js/lute/lute.min.js",
		"/assets/vendor/ace/ace.js",
		"/assets/vendor/ace/mode-golang.js",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusOK)
		}
		if response.Body.Len() == 0 {
			t.Errorf("GET %s returned an empty body", path)
		}
	}
}

func TestEmbeddedAssetsUseContentETags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newRouter()

	for _, requestPath := range []string{"/editor", "/assets/js/editor.js"} {
		firstRequest := httptest.NewRequest(http.MethodGet, requestPath, nil)
		firstResponse := httptest.NewRecorder()
		router.ServeHTTP(firstResponse, firstRequest)
		if firstResponse.Code != http.StatusOK {
			t.Fatalf("first GET %s status = %d, want %d", requestPath, firstResponse.Code, http.StatusOK)
		}
		etag := firstResponse.Header().Get("ETag")
		if etag == "" {
			t.Fatalf("GET %s has no ETag", requestPath)
		}
		if got := firstResponse.Header().Get("Cache-Control"); got != "public, max-age=0, must-revalidate" {
			t.Fatalf("GET %s Cache-Control = %q", requestPath, got)
		}

		cachedRequest := httptest.NewRequest(http.MethodGet, requestPath, nil)
		cachedRequest.Header.Set("If-None-Match", etag)
		cachedResponse := httptest.NewRecorder()
		router.ServeHTTP(cachedResponse, cachedRequest)
		if cachedResponse.Code != http.StatusNotModified {
			t.Fatalf("cached GET %s status = %d, want %d", requestPath, cachedResponse.Code, http.StatusNotModified)
		}
		if cachedResponse.Body.Len() != 0 {
			t.Fatalf("cached GET %s transferred %d body bytes", requestPath, cachedResponse.Body.Len())
		}
	}
}
