package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vj1024/easy-notes/web"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxUploadSize = 50 << 20
	tokenExpire   = 30 * 24 * time.Hour // token 有效期 30 天
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

func loadConfiguration() error {
	// 从环境变量读取配置
	jwtSecret = []byte(getEnv("JWT_SECRET", "your-default-secret-key-change-in-production"))
	adminUsername = getEnv("ADMIN_USERNAME", "admin")
	adminPasswordHash := getEnv("ADMIN_PASSWORD_HASH", "")
	rawPassword := getEnv("ADMIN_PASSWORD", "")

	// 优先使用密码哈希，如果没有则使用明文密码并生成哈希
	if adminPasswordHash != "" {
		adminPassword = adminPasswordHash
	} else if rawPassword != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash admin password: %w", err)
		}
		adminPassword = string(hashedPassword)
		log.Printf("Admin password hash: %s", adminPassword)
		log.Println("Please set ADMIN_PASSWORD_HASH environment variable for production use")
	} else {
		return fmt.Errorf("please set ADMIN_PASSWORD or ADMIN_PASSWORD_HASH environment variable")
	}
	return nil
}

func main() {
	if err := loadConfiguration(); err != nil {
		log.Fatal(err)
	}

	dataDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	dataDir = filepath.Join(dataDir, "data")
	baseDir = dataDir

	// 确保存储目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create storage directory: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := newRouter()

	log.Println("Server started on :8089")
	log.Printf("Admin username: %s", adminUsername)
	log.Fatal(router.Run(":8089"))
}

func newRouter() *gin.Engine {
	router := gin.Default()
	webFiles, err := web.NewCachingFileHandler(web.FS)
	if err != nil {
		panic(err)
	}
	assetFiles, err := web.NewCachingFileHandler(web.Assets())
	if err != nil {
		panic(err)
	}

	// 中间件
	router.Use(CORSMiddleware())
	router.Use(ErrorRecovery())

	// 公开路由 - 登录相关
	router.GET("/", redirectToLogin)
	router.GET("/editor", serveEmbeddedFile(webFiles, "editor.html"))
	router.GET("/login", serveEmbeddedFile(webFiles, "login.html"))
	router.GET("/favicon.ico", serveEmbeddedFile(webFiles, "favicon.ico"))
	router.GET("/manifest.webmanifest", serveEmbeddedFile(webFiles, "manifest.webmanifest"))
	router.GET("/sw.js", serveEmbeddedFile(webFiles, "sw.js"))
	router.GET("/assets/*filepath", gin.WrapH(http.StripPrefix("/assets", assetFiles)))
	router.POST("/api/login", loginHandler)
	router.GET("/api/check-auth", checkAuthHandler)

	// 需要认证的路由组
	authGroup := router.Group("/")
	authGroup.Use(JWTAuthMiddleware())
	authGroup.Use(LimitRequestBody(maxUploadSize))
	{
		authGroup.GET("/api/files", handleFilesRequest)
		authGroup.GET("/api/files/*path", handleFileGet)
		authGroup.PUT("/api/files/*path", handleFilePut)
		authGroup.POST("/api/files/*path", handleFileUpload)
		authGroup.DELETE("/api/files/*path", handleFileDelete)
		authGroup.POST("/api/mkdir", handleMkdir)
		authGroup.POST("/api/create-file", handleCreateFile)
		authGroup.POST("/api/rename", handleRename)
		authGroup.POST("/api/logout", logoutHandler)
	}

	return router
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

func serveEmbeddedFile(handler http.Handler, filePath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.URL.Path = "/" + filePath
		handler.ServeHTTP(c.Writer, c.Request)
	}
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

// LimitRequestBody applies the same limit to every mutating endpoint, including
// JSON, raw text and multipart requests.
func LimitRequestBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
