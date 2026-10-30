package web

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// 二次元背景源：公网免费图 API，逐个回退。
// 浏览器直连这些源常常超时或被防盗链拦截，服务端代理一次能显著提高成功率。
var bgSources = []string{
	"https://www.loliapi.com/acg/pe/",
	"https://t.alcy.cc/moez/",
	"https://t.alcy.cc/fj/",
	"https://api.mtyqx.cn/api/random.php",
	"https://www.loliapi.com/acg/pc/",
}

// bgClient 直连图源：短超时、限制体积，避免任一图源卡住整次请求
var bgClient = &http.Client{
	Timeout: 8 * time.Second,
	Transport: &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		TLSHandshakeTimeout: 6 * time.Second,
		DisableKeepAlives:   true,
	},
}

// RandomBackgroundHandler 随机二次元背景代理。
// 每次请求随机选一个起点，失败按序回退；全部失败返回 502，
// 前端据此切到自带的渐变兜底，不会出现"背景空白"。
func RandomBackgroundHandler(c *gin.Context) {
	start := c.Query("seed")
	order := shuffledSources(start)

	var lastErr error
	for _, src := range order {
		resp, err := bgClient.Get(src)
		if err != nil {
			lastErr = err
			continue
		}
		ct := resp.Header.Get("Content-Type")
		// 只接受图片，避免把图源的错误页当背景返回
		if resp.StatusCode != http.StatusOK || len(ct) < 6 || ct[:6] != "image/" {
			resp.Body.Close()
			lastErr = fmt.Errorf("图源 %s 返回 %d %s", src, resp.StatusCode, ct)
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil || len(body) == 0 {
			lastErr = fmt.Errorf("图源 %s 读取失败: %v", src, err)
			continue
		}
		c.Header("Content-Type", ct)
		c.Header("X-Bg-Source", src)
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, ct, body)
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"code": 502, "msg": "所有背景图源均不可用", "error": errString(lastErr)})
}

// shuffledSources 生成图源顺序：seed 相同则顺序稳定（便于复现），否则随机起点
func shuffledSources(seed string) []string {
	n := len(bgSources)
	out := make([]string, n)
	start := 0
	if seed != "" {
		start = hashSeed(seed) % n
	} else {
		start = int(time.Now().UnixNano() % int64(n))
	}
	for i := 0; i < n; i++ {
		out[i] = bgSources[(start+i)%n]
	}
	return out
}

func hashSeed(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		return -h
	}
	return h
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
