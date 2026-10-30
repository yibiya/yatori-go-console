package web

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"yatori-go-console/config"
	"yatori-go-console/global"

	"github.com/gin-gonic/gin"
)

// ServiceInit 启动 Web 面板（账号库已由 InitAccountDB 初始化）。
// 阻塞直到进程退出。
func ServiceInit(basic config.BasicSetting) {
	if global.GlobalDB == nil {
		panic("账号数据库未初始化，请先调用 InitAccountDB")
	}

	// 解析监听地址（向后兼容：默认 0.0.0.0:8080）
	host := basic.WebHost
	if host == "" {
		host = "0.0.0.0"
	}
	port := basic.WebPort
	if port == 0 {
		port = 8080
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	// 初始化服务器
	initServer := serverInit(basic)
	if basic.AdminPassword != "" {
		log.Printf("Web 服务启动于 http://%s （已启用接口鉴权）", addr)
	} else {
		log.Printf("Web 服务启动于 http://%s （未设置 adminPassword，接口无鉴权，建议仅在受信网络使用）", addr)
	}
	initServer.Run(addr)
}

// Group 封装 gin.RouterGroup
type Group struct {
	*gin.RouterGroup
}

// serverInit 初始化 Gin 服务
func serverInit(basic config.BasicSetting) *gin.Engine {
	router := gin.Default()
	router.Use(Cors(basic.AllowOrigins))
	router.Use(LoggerMiddleware())

	// 1️⃣ API 路由 - 放在最前面以确保优先匹配
	apiGroup := router.Group("/api")
	apiGroup.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "API working"})
	})
	// 随机二次元背景：放在鉴权中间件之前。浏览器的 <img>/CSS 背景请求带不了
	// X-Admin-Pass 头，而它只是转发一张公开图，不涉及任何账号数据。
	apiGroup.GET("/v1/randomBackground", RandomBackgroundHandler)
	// 鉴权中间件：仅当配置了 adminPassword 时生效；注册顺序保证 /api/test 健康检查不被拦截
	apiGroup.Use(AuthMiddleware(basic.AdminPassword))
	Group{apiGroup}.ApiV1Router()

	// 2️⃣ 前端静态资源 + 页面路由：用 NoRoute 处理（避免与 /api 段冲突）
	router.GET("/web", func(c *gin.Context) {
		c.Redirect(301, "/")
	})
	router.GET("/web/*filepath", func(c *gin.Context) {
		filepathParam := c.Param("filepath")
		if filepathParam == "" {
			filepathParam = "/"
		}
		c.Redirect(301, filepathParam)
	})

	// 3️⃣ 处理其他未匹配的路由（SPA fallback）
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API 请求返回 404
		if strings.HasPrefix(path, "/api") {
			c.JSON(404, gin.H{"error": "API endpoint not found", "path": path})
			return
		}

		// 非 GET 请求返回 405
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" {
			c.JSON(405, gin.H{"error": "Method not allowed"})
			return
		}

		// 根路径返回 index.html
		if path == "/" || path == "" {
			serveIndex(c)
			return
		}

		// 静态资源（通过扩展名判断）
		ext := filepath.Ext(path)
		if ext != "" && ext != ".html" {
			staticPath := filepath.Join("./assets/web", path[1:]) // 去掉开头的 "/"
			if _, err := os.Stat(staticPath); err == nil {
				// 面板静态资源统一禁缓存：JS 缓存会导致行为旧，
				// CSS 缓存会导致 [hidden] 之类的修复不生效（弹窗关不掉）
				setNoCache(c)
				c.File(staticPath)
				return
			}
			c.JSON(404, gin.H{"error": "Static resource not found", "path": path})
			return
		}

		// 路由请求：尝试 page.html，否则 fallback 到 index.html（SPA）
		cleanPath := strings.TrimRight(path, "/")
		pagePath := "./assets/web" + cleanPath + ".html"
		if _, err := os.Stat(pagePath); err == nil {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.File(pagePath)
			return
		}

		serveIndex(c)
	})

	return router
}

// serveIndex 返回前端入口页
func serveIndex(c *gin.Context) {
	indexPath := "./assets/web/index.html"
	if _, err := os.Stat(indexPath); err != nil {
		c.JSON(500, gin.H{"error": "index.html not found，请确认前端已构建到 assets/web"})
		return
	}
	setNoCache(c)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.File(indexPath)
}

// setNoCache 禁止浏览器缓存入口页与全部静态资源，保证每次打开都拿到最新的面板与修复
func setNoCache(c *gin.Context) {
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

// Cors 跨域中间件。
// 若配置了 AllowOrigins 白名单：仅对白名单来源回显 Origin 并允许携带凭据。
// 若未配置：默认放行所有来源但不允许携带凭据，避免"回显任意 Origin + Allow-Credentials"的高危组合。
func Cors(allowOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowOrigins))
	for _, o := range allowOrigins {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = true
		}
	}
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if len(allowed) > 0 {
			if origin != "" && allowed[origin] {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Vary", "Origin")
			}
		} else {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Admin-Pass")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// AuthMiddleware 接口鉴权中间件。
// 仅当 adminPassword 非空时启用；口令可通过请求头 X-Admin-Pass 或查询参数 admin_pass 传递。
func AuthMiddleware(adminPassword string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if adminPassword == "" {
			c.Next()
			return
		}
		if c.Request.Method == "OPTIONS" { // 放行 CORS 预检
			c.Next()
			return
		}
		pass := c.GetHeader("X-Admin-Pass")
		if pass == "" {
			pass = c.Query("admin_pass")
		}
		if subtle.ConstantTimeCompare([]byte(pass), []byte(adminPassword)) == 1 {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未授权：请提供正确的 X-Admin-Pass"})
	}
}

// LoggerMiddleware 日志中间件（请求体脱敏后再落盘）
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		log.Printf("Request %s %s Body: %s", c.Request.Method, c.Request.URL.Path, redactSensitive(bodyBytes))
		c.Next()
	}
}

// sensitiveKeys 需要在日志中脱敏的字段名（统一按小写比较）
var sensitiveKeys = map[string]struct{}{
	"password": {}, "apikey": {}, "api_key": {}, "adminpassword": {}, "admin_pass": {}, "token": {},
}

// redactSensitive 对请求体中的敏感字段脱敏，避免明文凭据写入日志文件
func redactSensitive(body []byte) string {
	if len(bytes.TrimSpace(body)) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return fmt.Sprintf("[非JSON请求体, %d 字节]", len(body))
	}
	redactValue(v)
	out, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("[请求体, %d 字节]", len(body))
	}
	return string(out)
}

func redactValue(v any) {
	switch val := v.(type) {
	case map[string]any:
		for k, vv := range val {
			if _, ok := sensitiveKeys[strings.ToLower(k)]; ok {
				if s, isStr := vv.(string); isStr && s != "" {
					val[k] = "***"
				}
			} else {
				redactValue(vv)
			}
		}
	case []any:
		for _, item := range val {
			redactValue(item)
		}
	}
}
