package controller

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
	"yatori-go-console/web/service"

	"github.com/gin-gonic/gin"
)

type UserApi struct{}

// logIDPattern 限制日志ID只能为字母/数字/下划线/连字符，防止路径穿越
var logIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// 获取账号列表
func (UserApi) AccountListController(c *gin.Context) {
	service.UserListService(c)
}

// 添加账号
func (UserApi) AddAccountController(c *gin.Context) {
	service.AddUserService(c)
}

// 删除账号
func (UserApi) DeleteAccountController(c *gin.Context) {
	service.DeleteUserService(c)
}

// 账号登录检测，用于检测账号密码是否正确
func (UserApi) AccountLoginCheckController(c *gin.Context) {
	service.AccountLoginCheckService(c)
}

// 获取账号信息
func (UserApi) GetAccountInformController(c *gin.Context) {
	service.GetAccountInformService(c)
}

// 修改账号基础信息
func (UserApi) UpdateAccountController(c *gin.Context) {
	service.UpdateUserService(c)
}

// 保存账号刷课配置
func (UserApi) SaveAccountConfigController(c *gin.Context) {
	service.SaveAccountConfigService(c)
}

// 获取账号刷课配置
func (UserApi) GetAccountConfigController(c *gin.Context) {
	service.GetAccountConfigService(c)
}

// 启用/停用账号
func (UserApi) ToggleAccountEnabledController(c *gin.Context) {
	service.ToggleAccountEnabledService(c)
}

// 获取课程列表
func (UserApi) AccountCourseListController(c *gin.Context) {
	service.AccountCourseListService(c)
}

// 获取账号日志
func (UserApi) AccountLogsController(c *gin.Context) {
	service.AccountLogsService(c)
}

// 启动刷课
func (UserApi) StartBrushController(c *gin.Context) {
	service.StartBrushService(c)
}

// 停止刷课
func (UserApi) StopBrushController(c *gin.Context) {
	service.StopBrushService(c)
}

// 保存AI配置
func (UserApi) SaveAiConfigController(c *gin.Context) {
	service.SaveAiConfigService(c)
}

// 测试AI配置
func (UserApi) TestAiConfigController(c *gin.Context) {
	service.TestAiConfigService(c)
}

// 加载AI配置
func (UserApi) GetAiConfigController(c *gin.Context) {
	service.GetAiConfigService(c)
}

// 日志同步接口（SSE）
func (UserApi) StreamLog(c *gin.Context) {
	logID := c.Param("id")
	if !logIDPattern.MatchString(logID) {
		c.String(400, "invalid log id")
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	file, err := os.Open(fmt.Sprintf(`./assets/log/log%s.txt`, logID))
	if err != nil {
		c.String(500, "error open log file")
		return
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	for {
		select {
		case <-c.Request.Context().Done():
			return // 客户端断开连接，退出
		default:
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return // 其他文件读取错误，退出
		}

		_, writeErr := c.Writer.Write([]byte("data: " + line + "\n\n"))
		if writeErr != nil {
			return // 写入失败（客户端断开），退出
		}
		c.Writer.Flush()
	}
}
