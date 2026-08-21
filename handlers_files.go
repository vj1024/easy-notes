package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// 文件操作处理函数（保持不变）
func handleFilesRequest(c *gin.Context) {
	list := c.DefaultQuery("list", "false")
	search := c.Query("search") // 使用Query而不是DefaultQuery

	if search != "" {
		performSearch(c, baseDir, search)
		return
	}

	if list == "true" {
		listDirectory(c, baseDir)
		return
	}

	c.JSON(http.StatusOK, DirectoryResponse{
		Path:    "",
		Success: true,
		Message: "Use ?list=true to list directory contents",
	})
}

func handleFileGet(c *gin.Context) {
	requestPath := c.Param("path")
	if requestPath == "" {
		handleFilesRequest(c)
		return
	}

	fullPath, err := safePath(requestPath)
	if err != nil {
		c.JSON(http.StatusForbidden, DirectoryResponse{
			Success: false,
			Message: "非法路径: " + err.Error(),
		})
		return
	}

	requestPath = strings.TrimPrefix(requestPath, "/")
	serveFile(c, fullPath, requestPath)
}

func handleFilePut(c *gin.Context) {
	requestPath := c.Param("path")
	if requestPath == "" {
		c.JSON(http.StatusBadRequest, DirectoryResponse{
			Success: false,
			Message: "Path is required",
		})
		return
	}

	fullPath, err := safePath(requestPath)
	if err != nil {
		c.JSON(http.StatusForbidden, DirectoryResponse{
			Success: false,
			Message: "非法路径: " + err.Error(),
		})
		return
	}

	requestPath = strings.TrimPrefix(requestPath, "/")
	uploadFile(c, fullPath, requestPath)
}

func handleFileUpload(c *gin.Context) {
	requestPath := c.Param("path")

	fullPath, err := safePath(requestPath)
	if err != nil {
		c.JSON(http.StatusForbidden, DirectoryResponse{
			Success: false,
			Message: "非法路径: " + err.Error(),
		})
		return
	}

	requestPath = strings.TrimPrefix(requestPath, "/")
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, DirectoryResponse{
			Success: false,
			Message: "Failed to create directory: " + err.Error(),
		})
		return
	}

	if c.ContentType() == "multipart/form-data" {
		file, err := c.FormFile("file")
		if err != nil {
			respondBodyError(c, err, "Failed to get uploaded file")
			return
		}
		source, err := file.Open()
		if err != nil {
			respondBodyError(c, err, "Failed to open uploaded file")
			return
		}
		defer source.Close()
		if err := atomicWrite(fullPath, source, 0644); err != nil {
			respondBodyError(c, err, "Failed to save file")
			return
		}
	} else {
		if err := atomicWrite(fullPath, c.Request.Body, 0644); err != nil {
			respondBodyError(c, err, "Failed to save file")
			return
		}
	}

	c.JSON(http.StatusOK, DirectoryResponse{
		Path:    requestPath,
		Success: true,
		Message: "File uploaded successfully",
	})
}

func listDirectory(c *gin.Context, dirPath string) {
	tree, err := GenerateJsTreeWithFilterAndSort(dirPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, DirectoryResponse{
			Success: false,
			Message: "Failed to generate tree: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, tree)
}

func serveFile(c *gin.Context, fullPath, requestPath string) {
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, DirectoryResponse{
				Success: false,
				Message: "File not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, DirectoryResponse{
			Success: false,
			Message: "Failed to access file: " + err.Error(),
		})
		return
	}

	if fileInfo.IsDir() {
		listDirectory(c, fullPath)
		return
	}

	c.File(fullPath)
}

func performSearch(c *gin.Context, rootPath, searchTerm string) {
	searchContent := strings.HasPrefix(searchTerm, "/")
	if searchContent {
		searchTerm = strings.TrimSpace(strings.TrimPrefix(searchTerm, "/"))
	}

	// 将搜索词按空格分割为多个关键字
	keywords := strings.Fields(searchTerm)
	if len(keywords) == 0 {
		c.JSON(http.StatusOK, DirectoryResponse{
			Path:    "",
			Results: []*SearchResult{},
			Success: true,
			Message: "找到 0 个匹配项",
		})
		return
	}

	// 使用 map 来构建树形结构
	rootNode := &SearchResult{
		ID:    "search-root",
		Text:  "搜索结果",
		Type:  "folder",
		Icon:  "jstree-folder",
		State: &NodeState{Opened: true}, // 默认展开
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录和隐藏文件
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// 只搜索支持的文本格式文件
		if !isSupportedTextFile(info.Name()) {
			return nil
		}

		// 检查文件名是否包含所有关键字
		filenameMatched := true
		for _, keyword := range keywords {
			if !strings.Contains(strings.ToLower(info.Name()), strings.ToLower(keyword)) {
				filenameMatched = false
				break
			}
		}

		// 仅当搜索词以 / 开头且文件名未匹配时，才读取文件内容。
		contentMatched := false
		if searchContent && !filenameMatched {
			content, err := os.ReadFile(path)
			if err != nil {
				// 如果无法读取文件内容，跳过
				return nil
			}

			// 检查文件内容是否包含所有关键字
			contentStr := strings.ToLower(string(content))
			contentMatched = true
			for _, keyword := range keywords {
				if !strings.Contains(contentStr, strings.ToLower(keyword)) {
					contentMatched = false
					break
				}
			}
		}

		// 如果文件名或内容匹配所有关键字，添加到树形结构中
		matched := filenameMatched || contentMatched

		if matched {
			relPath, err := filepath.Rel(rootPath, path)
			if err != nil {
				return err
			}

			// 构建路径，确保使用正斜杠
			relPath = filepath.ToSlash(relPath)

			// 分割路径，构建树形结构
			parts := strings.Split(relPath, "/")

			currentNode := rootNode
			// 遍历路径的每个部分，创建目录节点
			for i, part := range parts {
				if i == len(parts)-1 { // 最后一个是文件名
					break
				}

				// 查找或创建目录节点
				childFound := false
				for _, child := range currentNode.Children {
					if child.Text == part {
						currentNode = child
						childFound = true
						break
					}
				}

				if !childFound {
					newDirNode := &SearchResult{
						ID:    "search-dir-" + filepath.Join(parts[:i+1]...),
						Text:  part,
						Type:  "folder",
						Icon:  "jstree-folder",
						Path:  filepath.Join(parts[:i+1]...),
						State: &NodeState{Opened: true}, // 展开目录节点
					}
					currentNode.Children = append(currentNode.Children, newDirNode)
					currentNode = newDirNode
				} else {
					// 如果节点已存在，确保它是展开的
					if currentNode.State == nil {
						currentNode.State = &NodeState{Opened: true}
					} else {
						currentNode.State.Opened = true
					}
				}
			}

			// 添加文件节点到最后一个目录
			fileNode := &SearchResult{
				ID:   "search-file-" + relPath,
				Text: info.Name(),
				Type: "file",
				Icon: "jstree-file",
				Path: relPath,
			}
			currentNode.Children = append(currentNode.Children, fileNode)
		}

		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, DirectoryResponse{
			Success: false,
			Message: "搜索过程中发生错误: " + err.Error(),
		})
		return
	}

	// 如果根节点没有任何子节点，返回空数组
	var results []*SearchResult
	if len(rootNode.Children) > 0 {
		// 对搜索结果进行排序
		sortSearchResults(rootNode.Children)
		results = rootNode.Children
	}

	c.JSON(http.StatusOK, DirectoryResponse{
		Path:    "",
		Results: results,
		Success: true,
		Message: fmt.Sprintf("找到 %d 个匹配项", countSearchResults(results)),
	})
}

// 按类型和名称排序搜索结果
func sortSearchResults(nodes []*SearchResult) {
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
			sortSearchResults(node.Children)
		}
	}
}

// 辅助函数：计算搜索结果中的文件数量
func countSearchResults(results []*SearchResult) int {
	count := 0
	for _, result := range results {
		if result.Type == "file" {
			count++
		} else if len(result.Children) > 0 {
			count += countSearchResults(result.Children)
		}
	}
	return count
}

// handleFileDelete 删除文件或文件夹
func handleFileDelete(c *gin.Context) {
	requestPath := c.Param("path")
	if requestPath == "" || requestPath == "/" {
		c.JSON(http.StatusBadRequest, DirectoryResponse{
			Success: false,
			Message: "路径不能为空",
		})
		return
	}

	fullPath, err := safePath(requestPath)
	if err != nil {
		c.JSON(http.StatusForbidden, DirectoryResponse{
			Success: false,
			Message: "非法路径: " + err.Error(),
		})
		return
	}

	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, DirectoryResponse{
				Success: false,
				Message: "文件或目录不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, DirectoryResponse{
			Success: false,
			Message: "访问失败: " + err.Error(),
		})
		return
	}

	if fileInfo.IsDir() {
		if err := os.RemoveAll(fullPath); err != nil {
			c.JSON(http.StatusInternalServerError, DirectoryResponse{
				Success: false,
				Message: "删除目录失败: " + err.Error(),
			})
			return
		}
	} else {
		if err := os.Remove(fullPath); err != nil {
			c.JSON(http.StatusInternalServerError, DirectoryResponse{
				Success: false,
				Message: "删除文件失败: " + err.Error(),
			})
			return
		}
	}

	requestPath = strings.TrimPrefix(requestPath, "/")
	c.JSON(http.StatusOK, DirectoryResponse{
		Path:    filepath.ToSlash(requestPath),
		Success: true,
		Message: "删除成功",
	})
}

// MkdirRequest 创建目录请求
type MkdirRequest struct {
	Path string `json:"path" binding:"required"` // 相对于 data 的路径
}

// CreateFileRequest 创建文件请求
type CreateFileRequest struct {
	Path    string `json:"path" binding:"required"` // 相对于 data 的路径，含文件名
	Content string `json:"content"`                 // 初始内容
}

// handleMkdir 创建目录
func handleMkdir(c *gin.Context) {
	var req MkdirRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBodyError(c, err, "请求参数错误")
		return
	}

	fullPath, err := safePath(req.Path)
	if err != nil {
		c.JSON(http.StatusForbidden, DirectoryResponse{
			Success: false,
			Message: "非法路径: " + err.Error(),
		})
		return
	}

	if _, err := os.Stat(fullPath); err == nil {
		c.JSON(http.StatusConflict, DirectoryResponse{
			Success: false,
			Message: "目录已存在",
		})
		return
	}

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, DirectoryResponse{
			Success: false,
			Message: "创建目录失败: " + err.Error(),
		})
		return
	}

	req.Path = strings.TrimPrefix(req.Path, "/")
	c.JSON(http.StatusOK, DirectoryResponse{
		Path:    filepath.ToSlash(req.Path),
		Success: true,
		Message: "目录创建成功",
	})
}

// handleCreateFile 创建新文件
func handleCreateFile(c *gin.Context) {
	var req CreateFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBodyError(c, err, "请求参数错误")
		return
	}

	fullPath, err := safePath(req.Path)
	if err != nil {
		c.JSON(http.StatusForbidden, DirectoryResponse{
			Success: false,
			Message: "非法路径: " + err.Error(),
		})
		return
	}

	if _, err := os.Stat(fullPath); err == nil {
		c.JSON(http.StatusConflict, DirectoryResponse{
			Success: false,
			Message: "文件已存在",
		})
		return
	}

	// 确保父目录存在
	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, DirectoryResponse{
			Success: false,
			Message: "创建父目录失败: " + err.Error(),
		})
		return
	}

	if err := atomicWrite(fullPath, strings.NewReader(req.Content), 0644); err != nil {
		respondBodyError(c, err, "创建文件失败")
		return
	}

	req.Path = strings.TrimPrefix(req.Path, "/")
	c.JSON(http.StatusOK, DirectoryResponse{
		Path:    filepath.ToSlash(req.Path),
		Success: true,
		Message: "文件创建成功",
	})
}

func uploadFile(c *gin.Context, fullPath, requestPath string) {
	dir := filepath.Dir(fullPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, DirectoryResponse{
			Success: false,
			Message: "父目录不存在",
		})
		return
	}

	if err := atomicWrite(fullPath, c.Request.Body, 0644); err != nil {
		respondBodyError(c, err, "Failed to save file")
		return
	}

	c.JSON(http.StatusOK, DirectoryResponse{
		Path:    filepath.ToSlash(requestPath),
		Success: true,
		Message: "File uploaded successfully",
	})
}

func respondBodyError(c *gin.Context, err error, message string) {
	var maxBytesError *http.MaxBytesError
	status := http.StatusInternalServerError
	if errors.As(err, &maxBytesError) {
		status = http.StatusRequestEntityTooLarge
		message = fmt.Sprintf("请求内容超过 %d MB 限制", maxUploadSize>>20)
	} else if errors.Is(err, io.EOF) || strings.Contains(message, "请求参数") ||
		strings.Contains(message, "get uploaded file") {
		status = http.StatusBadRequest
	}
	c.JSON(status, DirectoryResponse{
		Success: false,
		Message: message,
	})
}
