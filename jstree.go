package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
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

	//return []*JsTreeNode{rootNode}, nil
	return rootNode.Children, nil
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
