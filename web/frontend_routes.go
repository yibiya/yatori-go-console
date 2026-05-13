package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func registerFrontendRoutes(router *gin.Engine) {
	router.GET("/web/*filepath", serveFrontendRequest)
}

func registerNoRouteHandler(router *gin.Engine) {
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "API endpoint not found",
				"path":  path,
			})
			return
		}

		if strings.HasPrefix(path, "/web") {
			serveFrontendIndex(c)
			return
		}

		c.JSON(http.StatusNotFound, gin.H{
			"error": "Page not found",
			"path":  path,
		})
	})
}

func serveFrontendRequest(c *gin.Context) {
	filepathParam := c.Param("filepath")
	if filepathParam == "/" || filepathParam == "" {
		serveFrontendIndex(c)
		return
	}

	if isStaticAssetRequest(filepathParam) {
		staticPath := frontendStaticPath(filepathParam)
		if _, err := os.Stat(staticPath); err == nil {
			c.File(staticPath)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Static resource not found",
			"path":  filepathParam,
		})
		return
	}

	serveFrontendIndex(c)
}

func serveFrontendIndex(c *gin.Context) {
	indexPath := frontendIndexPath()
	if _, err := os.Stat(indexPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Main index.html file not found"})
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.File(indexPath)
}

func isStaticAssetRequest(filepathParam string) bool {
	ext := filepath.Ext(filepathParam)
	return ext != "" && ext != ".html"
}

func frontendAssetsRoot() string {
	if dir := os.Getenv("YATORI_WEB_ASSETS_DIR"); dir != "" {
		return dir
	}
	return "./assets/web"
}

func frontendIndexPath() string {
	return filepath.Join(frontendAssetsRoot(), "index.html")
}

func frontendStaticPath(filepathParam string) string {
	return filepath.Join(frontendAssetsRoot(), strings.TrimPrefix(filepathParam, "/"))
}

// checkAssetsDir 检查静态资源目录是否存在
func checkAssetsDir() error {
	assetsPath := frontendAssetsRoot()
	if _, err := os.Stat(assetsPath); os.IsNotExist(err) {
		return fmt.Errorf("静态资源目录不存在: %s，请确保 Next.js 项目已构建并输出到该目录", assetsPath)
	}

	requiredFiles := []string{
		"index.html",
		"_next/static",
	}

	for _, file := range requiredFiles {
		fullPath := filepath.Join(assetsPath, file)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return fmt.Errorf("必要的静态资源文件不存在: %s", fullPath)
		}
	}

	return nil
}
