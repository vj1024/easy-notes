package web

import (
	"embed"
	_ "embed"
	"io/fs"
)

//go:embed login.html
var LoginPage string

//go:embed editor.html
var EditorPage string

//go:embed *
var FS embed.FS

func Assets() fs.FS {
	assets, err := fs.Sub(FS, "assets")
	if err != nil {
		panic(err)
	}
	return assets
}
