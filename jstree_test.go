package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestGenerateJsTreeIncludesUnknownAndExtensionlessFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"README", "archive.unknown", "notes.md", ".secret"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, ".hidden-dir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden-dir", "visible.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	nodes, err := GenerateJsTree(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, node := range nodes {
		names = append(names, node.Text)
	}
	sort.Strings(names)
	want := []string{"README", "archive.unknown", "notes.md"}
	for i := range want {
		if i >= len(names) || names[i] != want[i] {
			t.Fatalf("tree files = %v, want %v", names, want)
		}
	}
	if len(names) != len(want) {
		t.Fatalf("tree files = %v, want %v", names, want)
	}
}
