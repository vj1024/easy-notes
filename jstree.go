package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// JsTreeNode 表示jstree的节点结构
type JsTreeNode struct {
	ID       string        `json:"id"`
	Text     string        `json:"text"`
	Type     string        `json:"type"`
	Icon     string        `json:"icon"`
	Children []*JsTreeNode `json:"children,omitempty"`
	State    *NodeState    `json:"-"`
	//State    *NodeState    `json:"state,omitempty"`
}

const (
	typeFolder = "folder"
	typeFile   = "file"
)

// NodeState 表示节点的状态（可选）
type NodeState struct {
	Opened   bool `json:"opened,omitempty"`
	Disabled bool `json:"disabled,omitempty"`
	Selected bool `json:"selected,omitempty"`
}

func GenerateJsTree(rootPath string) ([]*JsTreeNode, error) {
	var rootNode *JsTreeNode
	nodeID := 1

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 跳过隐藏文件
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 如果是文件且不是支持的文本格式，则跳过
		if !d.IsDir() && !isSupportedTextFile(d.Name()) {
			return nil
		}

		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}

		// 根目录处理
		if relPath == "." {
			rootNode = &JsTreeNode{
				ID:   "root",
				Text: filepath.Base(rootPath),
				Type: typeFolder,
				Icon: "jstree-folder",
				State: &NodeState{
					Opened: true,
				},
			}
			return nil
		}

		// 创建节点
		node := &JsTreeNode{
			ID:   fmt.Sprintf("node-%d", nodeID),
			Text: d.Name(),
			Type: getType(d),
			Icon: getIcon(d),
		}
		nodeID++

		// 找到父节点并添加
		parent := findParentNode(rootNode, filepath.Dir(relPath))
		if parent != nil {
			parent.Children = append(parent.Children, node)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if rootNode != nil && rootNode.Children != nil {
		// 排序节点
		sortTreeNodes(rootNode.Children)
		return rootNode.Children, nil
	}

	return []*JsTreeNode{}, nil
}

// getType 获取文件类型
func getType(d fs.DirEntry) string {
	if d.IsDir() {
		return typeFolder
	}
	return typeFile
}

// getIcon 根据文件类型获取图标
func getIcon(d fs.DirEntry) string {
	return "jstree-" + getType(d)
}

// findParentNode 查找父节点（辅助函数）
func findParentNode(root *JsTreeNode, relPath string) *JsTreeNode {
	if relPath == "." {
		return root
	}

	current := root
	parts := strings.Split(relPath, string(filepath.Separator))

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		found := false
		for _, child := range current.Children {
			if child.Text == part {
				current = child
				found = true
				break
			}
		}

		if !found {
			return nil
		}
	}

	return current
}

// 检查文件扩展名是否为支持的文本格式
func isSupportedTextFile(filename string) bool {
	supportedExtensions := map[string]bool{
		".md":    true,  // Markdown
		".txt":   true,  // Text
		".rtf":   true,  // Rich Text Format
		".csv":   true,  // Comma Separated Values
		".log":   true,  // Log files
		".json":  true,  // JSON
		".xml":   true,  // XML
		".yaml":  true,  // YAML
		".yml":   true,  // YAML
		".toml":  true,  // TOML
		".ini":   true,  // INI files
		".conf":  true,  // Configuration files
		".cfg":   true,  // Configuration files
		".sql":   true,  // SQL files
		".html":  true,  // HTML
		".htm":   true,  // HTML
		".css":   true,  // CSS
		".js":    true,  // JavaScript
		".ts":    true,  // TypeScript
		".py":    true,  // Python
		".go":    true,  // Go
		".java":  true,  // Java
		".cpp":   true,  // C++
		".c":     true,  // C
		".h":     true,  // Header files
		".hpp":   true,  // Header files
		".rb":    true,  // Ruby
		".php":   true,  // PHP
		".sh":    true,  // Shell scripts
		".bat":   true,  // Batch files
		".ps1":   true,  // PowerShell scripts
		".pl":    true,  // Perl
		".swift": true,  // Swift
		".kt":    true,  // Kotlin
		".rs":    true,  // Rust
		".scala": true,  // Scala
		".dart":  true,  // Dart
		".vue":   true,  // Vue
		".jsx":   true,  // JSX
		".tsx":   true,  // TSX
		".less":  true,  // LESS
		".scss":  true,  // SCSS
		".sass":  true,  // SASS
		".coffee": true, // CoffeeScript
		".elm":   true,  // Elm
		".erl":   true,  // Erlang
		".ex":    true,  // Elixir
		".exs":   true,  // Elixir script
		".hs":    true,  // Haskell
		".lhs":   true,  // Literate Haskell
		".jl":    true,  // Julia
		".lua":   true,  // Lua
		".ml":    true,  // OCaml
		".mli":   true,  // OCaml interface
		".nim":   true,  // Nim
		".r":     true,  // R
		".R":     true,  // R
		".vb":    true,  // Visual Basic
		".vbs":   true,  // VBScript
		".fs":    true,  // F#
		".fsx":   true,  // F# script
		".fsi":   true,  // F# signature
		".clj":   true,  // Clojure
		".cljs":  true,  // ClojureScript
		".edn":   true,  // EDN
		".groovy": true, // Groovy
		".gradle": true, // Gradle
		".properties": true, // Properties files
		".env":   true,  // Environment files
		".gitignore": true, // Git ignore files
		".dockerignore": true, // Docker ignore files
		".npmignore": true, // NPM ignore files
		".editorconfig": true, // Editor config files
		".bowerrc": true, // Bower config
		".jshintrc": true, // JSHint config
		".eslintrc": true, // ESLint config
		".babelrc": true, // Babel config
		".prettierrc": true, // Prettier config
		".stylelintrc": true, // Stylelint config
		".markdown": true, // Markdown
		".mkd":    true,  // Markdown
		".mdown":  true,  // Markdown
		".mkdn":   true,  // Markdown
		".mdwn":   true,  // Markdown
		".mdtxt":  true,  // Markdown
		".mdtext": true,  // Markdown
		".rmd":    true,  // R Markdown
		".bib":    true,  // BibTeX
		".bst":    true,  // BibTeX style
		".tex":    true,  // LaTeX
		".cls":    true,  // LaTeX class
		".sty":    true,  // LaTeX style
		".idx":    true,  // LaTeX index
		".latex":  true,  // LaTeX
		".tikz":   true,  // TikZ graphics
		".svg":    true,  // SVG (XML-based)
		".dot":    true,  // Graphviz DOT
		".gv":     true,  // Graphviz
		".plantuml": true, // PlantUML
		".puml":   true,  // PlantUML
		".iuml":   true,  // PlantUML
		".wsd":    true,  // WebSequenceDiagrams
		".adoc":   true,  // AsciiDoc
		".asciidoc": true, // AsciiDoc
		".asc":    true,  // AsciiDoc
	}

	ext := strings.ToLower(filepath.Ext(filename))
	return supportedExtensions[ext]
}

// 按类型和名称排序节点
func sortTreeNodes(nodes []*JsTreeNode) {
	// 先排序：文件夹在前，文件在后；同类型按名称排序
	sort.Slice(nodes, func(i, j int) bool {
		// 如果类型不同，文件夹在前
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == "folder"
		}
		// 类型相同，按名称排序（忽略大小写）
		return strings.ToLower(nodes[i].Text) < strings.ToLower(nodes[j].Text)
	})

	// 递归排序子节点
	for _, node := range nodes {
		if node.Children != nil && len(node.Children) > 0 {
			sortTreeNodes(node.Children)
		}
	}
}
