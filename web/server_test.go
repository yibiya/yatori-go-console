package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFrontendAssetsRootUsesEnvironmentOverride(t *testing.T) {
	t.Setenv("YATORI_WEB_ASSETS_DIR", "/tmp/yatori-assets")

	if got := frontendAssetsRoot(); got != "/tmp/yatori-assets" {
		t.Fatalf("expected env override, got %q", got)
	}
}

func TestServerInitServesAPIAndFrontendRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	assetsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(assetsDir, "index.html"), []byte("<html>ok</html>"), 0644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "app.js"), []byte("console.log('ok')"), 0644); err != nil {
		t.Fatalf("write app.js: %v", err)
	}
	t.Setenv("YATORI_WEB_ASSETS_DIR", assetsDir)

	router := serverInit()

	apiResp := httptest.NewRecorder()
	apiReq := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	router.ServeHTTP(apiResp, apiReq)
	if apiResp.Code != http.StatusOK {
		t.Fatalf("expected API 200, got %d body=%s", apiResp.Code, apiResp.Body.String())
	}

	spaResp := httptest.NewRecorder()
	spaReq := httptest.NewRequest(http.MethodGet, "/web/dashboard", nil)
	router.ServeHTTP(spaResp, spaReq)
	if spaResp.Code != http.StatusOK {
		t.Fatalf("expected SPA route 200, got %d body=%s", spaResp.Code, spaResp.Body.String())
	}
	if !strings.Contains(spaResp.Body.String(), "<html>ok</html>") {
		t.Fatalf("expected SPA response to serve index.html, got %q", spaResp.Body.String())
	}

	assetResp := httptest.NewRecorder()
	assetReq := httptest.NewRequest(http.MethodGet, "/web/app.js", nil)
	router.ServeHTTP(assetResp, assetReq)
	if assetResp.Code != http.StatusOK {
		t.Fatalf("expected asset route 200, got %d body=%s", assetResp.Code, assetResp.Body.String())
	}
	if !strings.Contains(assetResp.Body.String(), "console.log('ok')") {
		t.Fatalf("expected asset response to serve JS file, got %q", assetResp.Body.String())
	}
}
