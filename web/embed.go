package web

import (
	"embed"
	_ "embed"
)

//go:embed login.html
var LoginPage string

//go:embed editor.html
var EditorPage string

//go:embed *
var FS embed.FS
