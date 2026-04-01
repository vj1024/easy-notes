package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vj1024/easy-notes/web"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxUploadSize = 50 << 20
	tokenExpire   = 7 * 24 * time.Hour // token 有效期 7 天
)

var (
	baseDir       = "./data"
	jwtSecret     []byte
	adminUsername string
	adminPassword string
)

type FileInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
	IsDir     bool      `json:"isDir"`
	Type      string    `json:"type"`
	Extension string    `json:"extension,omitempty"`
}

type SearchResult struct {
	ID       string          `json:"id"`
	Text     string          `json:"text"`
	Type     string          `json:"type"`
	Icon     string          `json:"icon"`
	Path     string          `json:"path"`
	State    *NodeState      `json:"state,omitempty"`
	Children []*SearchResult `json:"children,omitempty"`
}

type DirectoryResponse struct {
	Path    string          `json:"path"`
	Files   []FileInfo      `json:"files"`
	Results []*SearchResult `json:"results,omitempty"`
	Success bool            `json:"success"`
	Message string          `json:"message,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func init() {
	// 从环境变量读取配置
	jwtSecret = []byte(getEnv("JWT_SECRET", "your-default-secret-key-change-in-production"))
	adminUsername = getEnv("ADMIN_USERNAME", "admin")
	adminPasswordHash := getEnv("ADMIN_PASSWORD_HASH", "")
	rawPassword := getEnv("ADMIN_PASSWORD", "")

	// 优先使用密码哈希，如果没有则使用明文密码并生成哈希
	if adminPasswordHash != "" {
		adminPassword = adminPasswordHash
	} else if rawPassword != "" {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
		adminPassword = string(hashedPassword)
		log.Printf("Admin password hash: %s", adminPassword)
		log.Println("Please set ADMIN_PASSWORD_HASH environment variable for production use")
	} else {
		log.Fatal("Please set ADMIN_PASSWORD or ADMIN_PASSWORD_HASH environment variable")
	}
}

func main() {
	dataDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	dataDir = filepath.Join(dataDir, "data")

	// 确保存储目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create storage directory: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// 中间件
	router.Use(CORSMiddleware())
	router.Use(ErrorRecovery())

	// 公开路由 - 登录相关
	router.GET("/", redirectToLogin)
	router.GET("/editor", serveEditorPage)
	router.GET("/login", serveLoginPage)
	router.GET("/favicon.ico", serveFavicon)
	router.POST("/api/login", loginHandler)
	router.GET("/api/check-auth", checkAuthHandler)

	// 需要认证的路由组
	authGroup := router.Group("/")
	authGroup.Use(JWTAuthMiddleware())
	{
		authGroup.GET("/api/files", handleFilesRequest)
		authGroup.GET("/api/files/*path", handleFileGet)
		authGroup.PUT("/api/files/*path", handleFilePut)
		authGroup.POST("/api/files/*path", handleFileUpload)
		authGroup.POST("/api/logout", logoutHandler)
	}

	log.Println("Server started on :8089")
	log.Printf("Admin username: %s", adminUsername)
	log.Fatal(router.Run(":8089"))
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// 生成带过滤和排序的JsTree
func GenerateJsTreeWithFilterAndSort(rootPath string) ([]*JsTreeNode, error) {
	return GenerateJsTree(rootPath)
}

// 文件编辑页面
func serveEditorPage(c *gin.Context) {
	c.FileFromFS("editor.html", http.FS(web.FS))
}

// 登录页面
func serveLoginPage(c *gin.Context) {
	c.FileFromFS("login.html", http.FS(web.FS))
}

// 登录页面
func serveFavicon(c *gin.Context) {
	c.FileFromFS("favicon.ico", http.FS(web.FS))
}

// 重定向到登录页
func redirectToLogin(c *gin.Context) {
	c.Redirect(http.StatusFound, "/login")
}

// 登录处理
func loginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, LoginResponse{
			Success: false,
			Message: "请求参数错误",
		})
		return
	}

	// 验证用户名
	if req.Username != adminUsername {
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Success: false,
			Message: "用户名或密码错误",
		})
		return
	}

	// 验证密码
	err := bcrypt.CompareHashAndPassword([]byte(adminPassword), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Success: false,
			Message: "用户名或密码错误",
		})
		return
	}

	// 生成 JWT token
	token, err := generateToken(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, LoginResponse{
			Success: false,
			Message: "生成token失败",
		})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Success: true,
		Message: "登录成功",
		Token:   token,
	})
}

// 检查认证状态
func checkAuthHandler(c *gin.Context) {
	tokenString := c.GetHeader("Authorization")
	if tokenString == "" {
		tokenString = c.Query("token")
	}

	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"authenticated": false,
		})
		return
	}

	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	claims, err := parseToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"authenticated": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"username":      claims.Username,
	})
}

// 登出处理
func logoutHandler(c *gin.Context) {
	c.JSON(http.StatusOK, LoginResponse{
		Success: true,
		Message: "登出成功",
	})
}

// 生成 JWT token
func generateToken(username string) (string, error) {
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenExpire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "file-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// 解析 JWT token
func parseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// JWT 认证中间件
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否是 API 请求
		isAPIRequest := strings.HasPrefix(c.Request.URL.Path, "/api/") ||
			strings.HasPrefix(c.Request.URL.Path, "/files/") ||
			strings.HasPrefix(c.Request.Header.Get("Accept"), "application/json")

		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			tokenString = c.Query("token")
		}

		tokenString = strings.TrimPrefix(tokenString, "Bearer ")

		if tokenString == "" {
			if isAPIRequest {
				c.JSON(http.StatusUnauthorized, LoginResponse{
					Success: false,
					Message: "未授权访问，请先登录",
				})
			} else {
				c.Redirect(http.StatusFound, "/login?redirect="+c.Request.URL.Path)
			}
			c.Abort()
			return
		}

		claims, err := parseToken(tokenString)
		if err != nil {
			if isAPIRequest {
				c.JSON(http.StatusUnauthorized, LoginResponse{
					Success: false,
					Message: "token无效或已过期",
				})
			} else {
				c.Redirect(http.StatusFound, "/login?redirect="+c.Request.URL.Path)
			}
			c.Abort()
			return
		}

		// 将用户名存入上下文
		c.Set("username", claims.Username)
		c.Next()
	}
}

// 中间件函数（保持不变）
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func ErrorRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				c.JSON(http.StatusInternalServerError, DirectoryResponse{
					Success: false,
					Message: "内部服务器错误",
				})
			}
		}()
		c.Next()
	}
}

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

	requestPath = strings.TrimPrefix(requestPath, "/")
	requestPath = filepath.Clean(requestPath)
	fullPath := filepath.Join(baseDir, requestPath)

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

	requestPath = strings.TrimPrefix(requestPath, "/")
	requestPath = filepath.Clean(requestPath)
	fullPath := filepath.Join(baseDir, requestPath)

	uploadFile(c, fullPath, requestPath)
}

func handleFileUpload(c *gin.Context) {
	requestPath := c.Param("path")
	requestPath = strings.TrimPrefix(requestPath, "/")
	fullPath := filepath.Join(baseDir, requestPath)
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
			c.JSON(http.StatusBadRequest, DirectoryResponse{
				Success: false,
				Message: "Failed to get uploaded file: " + err.Error(),
			})
			return
		}
		if err := c.SaveUploadedFile(file, fullPath); err != nil {
			c.JSON(http.StatusInternalServerError, DirectoryResponse{
				Success: false,
				Message: "Failed to save file: " + err.Error(),
			})
			return
		}
	} else {
		dst, err := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			c.JSON(http.StatusInternalServerError, DirectoryResponse{
				Success: false,
				Message: "Failed to create file: " + err.Error(),
			})
			return
		}
		defer dst.Close()

		// 复制文件内容
		if _, err := io.Copy(dst, c.Request.Body); err != nil {
			c.JSON(http.StatusInternalServerError, DirectoryResponse{
				Success: false,
				Message: "Failed to save file: " + err.Error(),
			})
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

		// 如果文件名不匹配所有关键字，则检查文件内容
		contentMatched := false
		if !filenameMatched {
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

func uploadFile(c *gin.Context, fullPath, requestPath string) {
	dir := filepath.Dir(fullPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, DirectoryResponse{
			Success: false,
			Message: "Parent directory does not exist",
		})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	dst, err := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		c.JSON(http.StatusInternalServerError, DirectoryResponse{
			Success: false,
			Message: "Failed to create file: " + err.Error(),
		})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, c.Request.Body); err != nil {
		c.JSON(http.StatusInternalServerError, DirectoryResponse{
			Success: false,
			Message: "Failed to save file: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, DirectoryResponse{
		Path:    filepath.ToSlash(requestPath),
		Success: true,
		Message: "File uploaded successfully",
	})
}
