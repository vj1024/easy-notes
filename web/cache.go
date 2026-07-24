package web

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// NewCachingFileHandler serves an fs.FS with strong, content-based ETags.
// Clients may retain responses but must cheaply revalidate them; unchanged
// files receive a 304 response without transferring the body.
func NewCachingFileHandler(fileSystem fs.FS) (http.Handler, error) {
	etags := make(map[string]string)
	err := fs.WalkDir(fileSystem, ".", func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := fs.ReadFile(fileSystem, filePath)
		if err != nil {
			return err
		}
		etags[filePath] = fmt.Sprintf(`"sha256-%x"`, sha256.Sum256(content))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("build static asset ETags: %w", err)
	}

	fileServer := http.FileServer(http.FS(fileSystem))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filePath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		etag, ok := etags[filePath]
		if ok {
			w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
			w.Header().Set("ETag", etag)
			if etagMatches(r.Header.Get("If-None-Match"), etag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}

func etagMatches(header, current string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == current || strings.TrimPrefix(candidate, "W/") == current {
			return true
		}
	}
	return false
}
