package web

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"yatori-go-console/dao"
	"yatori-go-console/global"
	"yatori-go-console/web/service"

	"github.com/gin-gonic/gin"
)

// ServiceInit 统一初始化
func ServiceInit() {
	dbInit, err := dao.SqliteInit()
	if err != nil {
		panic(err)
	}
	global.GlobalDB = dbInit
	service.StartAutoExecutionScheduler()

	serverInit().Run(":8080")
}

// Group 封装 gin.RouterGroup
type Group struct {
	*gin.RouterGroup
}

// serverInit 初始化 Gin 服务
func serverInit() *gin.Engine {
	router := newServer()
	registerRoutes(router)
	return router
}

func newServer() *gin.Engine {
	router := gin.Default()
	router.Use(Cors())
	router.Use(LoggerMiddleware())
	return router
}

func registerRoutes(router *gin.Engine) {
	registerAPIRoutes(router)
	registerFrontendRoutes(router)
	registerNoRouteHandler(router)
}

func registerAPIRoutes(router *gin.Engine) {
	apiGroup := router.Group("/api")
	apiGroup.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "API working"})
	})
	Group{apiGroup}.ApiV1Router()
}

// Cors 跨域中间件
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// LoggerMiddleware 日志中间件
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		log.Printf("Request Body: %s", string(bodyBytes))
		c.Next()
	}
}
