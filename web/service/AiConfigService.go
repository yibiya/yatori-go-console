package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"yatori-go-console/config"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// AI 配置在面板中的结构化展示
type aiConfigView struct {
	Provider        string `json:"provider"`
	RuntimeProvider string `json:"runtimeProvider"`
	Model           string `json:"model"`
	ApiKey          string `json:"apiKey"`
	BaseUrl         string `json:"baseUrl"`
	Endpoint        string `json:"endpoint"`
	CustomEndpoint  string `json:"customEndpoint"`
	AiUrl           string `json:"aiUrl"`
}

// 保存AI配置（写入 config.yaml，全局生效）
func SaveAiConfigService(c *gin.Context) {
	var req struct {
		Provider       string `json:"provider"`
		Model          string `json:"model"`
		ApiKey         string `json:"apiKey"`
		BaseUrl        string `json:"baseUrl"`
		Endpoint       string `json:"endpoint"`
		CustomEndpoint string `json:"customEndpoint"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请求参数错误: " + err.Error()})
		return
	}

	endpointPath, err := normalizeEndpoint(req.Endpoint, req.CustomEndpoint)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	fullUrl := endpointPath
	if base := strings.TrimSpace(req.BaseUrl); base != "" {
		fullUrl, err = joinBaseAndEndpoint(base, endpointPath)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
	}

	configPath := "./config.yaml"
	data, err := os.ReadFile(configPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "读取配置文件失败: " + err.Error()})
		return
	}
	raw := make(map[string]any)
	if err := yaml.Unmarshal(data, &raw); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析配置文件失败: " + err.Error()})
		return
	}

	setting, _ := raw["setting"].(map[string]any)
	if setting == nil {
		setting = map[string]any{}
	}
	aiSetting, _ := setting["aiSetting"].(map[string]any)
	if aiSetting == nil {
		aiSetting = map[string]any{}
	}
	if provider := strings.TrimSpace(req.Provider); provider != "" {
		aiSetting["aiType"] = provider
		aiSetting["provider"] = provider
	} else if runtime, ok := aiSetting["aiType"].(string); ok {
		aiSetting["provider"] = runtime
	}
	aiSetting["model"] = req.Model
	aiSetting["API_KEY"] = req.ApiKey
	aiSetting["aiUrl"] = fullUrl
	setting["aiSetting"] = aiSetting
	raw["setting"] = setting

	out, err := yaml.Marshal(raw)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "序列化配置失败: " + err.Error()})
		return
	}
	if err := config.SaveRawConfigAtomic(configPath, out); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "保存配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "保存成功"})
}

// 测试AI配置连通性
func TestAiConfigService(c *gin.Context) {
	var req struct {
		Provider       string `json:"provider"`
		Model          string `json:"model"`
		ApiKey         string `json:"apiKey"`
		BaseUrl        string `json:"baseUrl"`
		Endpoint       string `json:"endpoint"`
		CustomEndpoint string `json:"customEndpoint"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请求参数错误: " + err.Error()})
		return
	}
	endpointPath, err := normalizeEndpoint(req.Endpoint, req.CustomEndpoint)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	fullUrl, err := joinBaseAndEndpoint(req.BaseUrl, endpointPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	body, err := makeAiTestBody(req.Endpoint, req.Model)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	httpReq, err := http.NewRequest(http.MethodPost, fullUrl, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "构造请求失败: " + err.Error()})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if key := strings.TrimSpace(req.ApiKey); key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请求失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode == http.StatusOK {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "AI 配置可用"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("AI 返回 %d：%s", resp.StatusCode, trimResponseBody(respBody))})
}

// 加载AI配置
func GetAiConfigService(c *gin.Context) {
	configPath := "./config.yaml"
	data, err := os.ReadFile(configPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "读取配置文件失败: " + err.Error()})
		return
	}
	raw := make(map[string]any)
	if err := yaml.Unmarshal(data, &raw); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析配置文件失败: " + err.Error()})
		return
	}

	view := aiConfigView{Endpoint: "chat", CustomEndpoint: ""}
	if setting, ok := raw["setting"].(map[string]any); ok {
		if a, ok := setting["aiSetting"].(map[string]any); ok {
			if v, ok := a["aiType"].(string); ok {
				view.Provider = v
				view.RuntimeProvider = v
			}
			if v, ok := a["provider"].(string); ok && v != "" {
				view.Provider = v
			}
			if v, ok := a["model"].(string); ok {
				view.Model = v
			}
			if v, ok := a["API_KEY"].(string); ok {
				view.ApiKey = v
			}
			if v, ok := a["aiUrl"].(string); ok {
				baseURL, endpoint, customEndpoint := decomposeAiUrl(v)
				view.AiUrl = v
				view.BaseUrl = baseURL
				view.Endpoint = endpoint
				view.CustomEndpoint = customEndpoint
			}
		}
	}
	var externalBankUrl string
	if setting, ok := raw["setting"].(map[string]any); ok {
		if q, ok := setting["apiQueSetting"].(map[string]any); ok {
			if v, ok := q["url"].(string); ok {
				externalBankUrl = v
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"data":            view,
		"externalBankUrl": externalBankUrl,
	})
}

// normalizeEndpoint 归一化端点路径
func normalizeEndpoint(endpoint, customEp string) (string, error) {
	switch strings.TrimSpace(endpoint) {
	case "responses":
		return "/v1/responses", nil
	case "", "chat":
		return "/v1/chat/completions", nil
	case "custom":
		customEp = strings.Trim(strings.TrimSpace(customEp), "/")
		if customEp == "" {
			return "", errors.New("自定义端点路径不能为空")
		}
		return "/" + customEp, nil
	default:
		return "/v1/chat/completions", nil
	}
}

// joinBaseAndEndpoint 拼接基础 URL 与端点路径
func joinBaseAndEndpoint(baseURL, endpointPath string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", errors.New("基础 URL 不能为空")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("基础 URL 格式错误")
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(endpointPath, "/"), nil
}

// decomposeAiUrl 拆分完整 AI URL 为基础 URL + 端点类型
func decomposeAiUrl(fullUrl string) (string, string, string) {
	fullUrl = strings.TrimSpace(fullUrl)
	if fullUrl == "" {
		return "", "chat", ""
	}
	if strings.HasSuffix(fullUrl, "/v1/responses") {
		return strings.TrimSuffix(fullUrl, "/v1/responses"), "responses", ""
	}
	if strings.HasSuffix(fullUrl, "/v1/chat/completions") {
		return strings.TrimSuffix(fullUrl, "/v1/chat/completions"), "chat", ""
	}
	parsed, err := url.Parse(fullUrl)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		base := parsed.Scheme + "://" + parsed.Host
		customEp := strings.TrimLeft(parsed.EscapedPath(), "/")
		if parsed.RawQuery != "" {
			customEp += "?" + parsed.RawQuery
		}
		return base, "custom", customEp
	}
	return fullUrl, "custom", ""
}

// makeAiTestBody 构造测试请求体
func makeAiTestBody(endpoint, model string) ([]byte, error) {
	if endpoint == "responses" {
		return json.Marshal(map[string]any{
			"model": model,
			"input": "hi",
		})
	}
	return json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
	})
}

// trimResponseBody 截断过长的响应体，避免面板展示失控
func trimResponseBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 1000 {
		return text[:1000] + "..."
	}
	return text
}
