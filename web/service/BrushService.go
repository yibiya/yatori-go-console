package service

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"yatori-go-console/dao"
	"yatori-go-console/entity/pojo"
	"yatori-go-console/entity/vo"
	"yatori-go-console/global"
	"yatori-go-console/web/activity"

	"github.com/gin-gonic/gin"
)

// 启动单个账号的刷课任务
func StartBrushService(c *gin.Context) {
	uid := c.Param("uid")
	userPo, err := dao.QueryUser(global.GlobalDB, pojo.UserPO{Uid: uid})
	if err != nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: err.Error()})
		return
	}
	if userPo == nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: "该账号不存在"})
		return
	}

	act := activity.Ensure(*userPo)
	if act == nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: "暂不支持该平台类型的启动：" + userPo.AccountType})
		return
	}
	// 已在运行则直接返回，避免重复启动多个刷课协程
	if act.IsActive() {
		c.JSON(http.StatusOK, vo.Response{Code: 200, Message: "任务已在运行中"})
		return
	}

	// 账号被停用时拒绝启动，避免面板与命令行行为不一致
	userConfig := userPo.UserConfigTurnEntity()
	if !userConfig.IsEnabled() {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: "该账号已停用，请先启用后再启动"})
		return
	}
	// 同步校验运行时间窗口：配置了窗口但当前不在窗口内时直接拒绝，
	// 避免面板显示"启动成功"而任务其实没跑。
	if err := activity.CheckRunWindow(userConfig); err != nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: err.Error()})
		return
	}

	go func() {
		// Start 内部会在需要时自动登录；错误只记录日志，不拖垮进程
		if err := act.Start(); err != nil {
			log.Printf("刷课启动失败 uid=%s account=%s: %v", uid, userPo.Account, err)
		}
	}()

	c.JSON(http.StatusOK, vo.Response{Code: 200, Message: "启动成功"})
}

// 停止单个账号的刷课任务
func StopBrushService(c *gin.Context) {
	uid := c.Param("uid")
	userPo, err := dao.QueryUser(global.GlobalDB, pojo.UserPO{Uid: uid})
	if err != nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: err.Error()})
		return
	}
	if userPo == nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: "该账号不存在"})
		return
	}
	if !activity.Stop(uid) {
		c.JSON(http.StatusOK, vo.Response{Code: 200, Message: "任务未在运行"})
		return
	}
	c.JSON(http.StatusOK, vo.Response{Code: 200, Message: "停止成功"})
}

// 拉取账号课程列表（学习通）
func AccountCourseListService(c *gin.Context) {
	uid := c.Param("uid")
	userPo, err := dao.QueryUser(global.GlobalDB, pojo.UserPO{Uid: uid})
	if err != nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: err.Error()})
		return
	}
	if userPo == nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: "该账号不存在"})
		return
	}

	xxt, ok := lookupCourseLister(*userPo)
	if !ok {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: "该平台暂不支持课程列表预览"})
		return
	}
	list, err := xxt()
	if err != nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: "拉取课程失败：" + err.Error()})
		return
	}
	courseList := []vo.CourseInformResponse{}
	for i := range list {
		courseList = append(courseList, vo.CourseInformResponse{
			CourseId:   list[i].CourseID,
			CourseName: list[i].CourseName,
			Progress:   float32(list[i].JobRate),
			Instructor: list[i].CourseTeacher,
		})
	}
	c.JSON(http.StatusOK, vo.Response{Code: 200, Message: "拉取信息成功", Data: gin.H{"courseList": courseList}})
}

// lookupCourseLister 返回课程摘要拉取函数（当前仅学习通支持预览）
func lookupCourseLister(userPo pojo.UserPO) (func() ([]courseSummary, error), bool) {
	act := activity.Ensure(userPo)
	if act == nil {
		return nil, false
	}
	xxt, ok := act.(activity.XXTAbility)
	if !ok {
		return nil, false
	}
	return func() ([]courseSummary, error) {
		list, err := xxt.PullCourseList()
		if err != nil {
			return nil, err
		}
		out := make([]courseSummary, 0, len(list))
		for i := range list {
			out = append(out, courseSummary{
				CourseID:      list[i].CourseID,
				CourseName:    list[i].CourseName,
				JobRate:       list[i].JobRate,
				CourseTeacher: list[i].CourseTeacher,
			})
		}
		return out, nil
	}, true
}

// courseSummary 课程摘要，供面板预览
type courseSummary struct {
	CourseID      string  `json:"courseId"`
	CourseName    string  `json:"courseName"`
	JobRate       float64 `json:"jobRate"`
	CourseTeacher string  `json:"courseTeacher"`
}

// accountLogUIDPattern uid 格式校验（复用 streamLog 同款规则）
var accountLogUIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// 获取账号日志（读取 ./assets/log 下最新日志文件，按 uid/account/remarkName 过滤并脱敏）
func AccountLogsService(c *gin.Context) {
	uid := c.Param("uid")
	if !accountLogUIDPattern.MatchString(uid) {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: "uid 格式错误"})
		return
	}

	userPo, err := dao.QueryUser(global.GlobalDB, pojo.UserPO{Uid: uid})
	if err != nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: err.Error()})
		return
	}
	if userPo == nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: "该账号不存在"})
		return
	}

	// 收集用于匹配日志的标识：uid、账号、备注名
	identifiers := []string{uid, userPo.Account}
	userConfig := userPo.UserConfigTurnEntity()
	if remark := strings.TrimSpace(userConfig.RemarkName); remark != "" && remark != userPo.Account {
		identifiers = append(identifiers, remark)
	}

	// limit 控制返回行数（默认 500），前端实时刷新时用小 limit 拿增量
	limit := 500
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			limit = n
		}
	}
	// since 返回"晚于该时间戳"的新增行，用于断线重连后补齐，避免漏日志
	since := int64(0)
	if v := c.Query("since"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			since = n
		}
	}

	logs, err := readAccountLogs("./assets/log", identifiers, limit, since)
	if err != nil {
		log.Printf("读取账号日志失败 uid=%s: %v", uid, err)
	}

	c.JSON(http.StatusOK, vo.Response{Code: 200, Message: "拉取日志成功", Data: map[string]any{
		"success": err == nil,
		"uid":     uid,
		"logs":    logs,
		"since":   time.Now().Unix(),
	}})
}

// readAccountLogs 读取匹配 identifiers 的日志，返回**最新**的 maxLines 行（时间正序）。
// 旧实现从头读、凑满上限就停，面板看到的是几天前的日志而不是当前进展；
// 这里改为对每个文件从尾部倒着收集，再反转为时间正序。
func readAccountLogs(logDir string, identifiers []string, maxLines int, since int64) (string, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return "", err
	}

	var files []os.FileInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "log") || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, info)
	}
	if len(files) == 0 {
		return "", nil
	}

	// 按修改时间倒序：新日志在后，收集到上限即可停
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().After(files[j].ModTime())
	})

	var patterns []string
	for _, id := range identifiers {
		if id == "" {
			continue
		}
		patterns = append(patterns, regexp.QuoteMeta(id))
	}
	if len(patterns) == 0 {
		return "", nil
	}
	matchRe := regexp.MustCompile(strings.Join(patterns, "|"))

	var collected []string
	for _, info := range files {
		if len(collected) >= maxLines {
			break
		}
		path := filepath.Join(logDir, info.Name())
		matched, err := tailMatchedLines(path, matchRe, maxLines-len(collected), since)
		if err != nil {
			continue
		}
		// 同一文件内部已按时间正序；跨文件要按新→旧拼接，最后整体反转
		collected = append(collected, matched...)
	}

	if len(collected) == 0 {
		return "", nil
	}
	// collected 是"新文件在前"，反转为时间正序输出
	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}
	return strings.Join(collected, "\n"), nil
}

// logTimePrefix 匹配日志行开头的时间戳 [2006-01-02 15:04:05]
var logTimePrefix = regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2})\]`)

// parseLogTime 解析日志行时间，解析不出来返回 0（该行在 since 模式下会被跳过）
func parseLogTime(line string) int64 {
	m := logTimePrefix.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return 0
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", m[1]+" "+m[2], time.Local)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// tailMatchedLines 从文件尾部倒着读取，最多收集 want 条匹配行，返回时间正序的结果
func tailMatchedLines(path string, matchRe *regexp.Regexp, want int, since int64) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	var rev []string
	for scanner.Scan() {
		line := scanner.Text()
		if !matchRe.MatchString(line) {
			continue
		}
		// since 模式：只要比上次更新的时间戳新；为 0 时返回尾部最新
		if since > 0 && parseLogTime(line) <= since {
			continue
		}
		for _, re := range desensitizeRes {
			line = re.ReplaceAllString(line, "$1: ***")
		}
		rev = append(rev, line)
		if len(rev) >= want {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// 反转成时间正序
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, nil
}

// desensitizeRes 日志脱敏规则
var desensitizeRes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key)\s*[:=]\s*[^\s]+`),
	regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*[^\s]+`),
	regexp.MustCompile(`(?i)(token|secret)\s*[:=]\s*[^\s]+`),
	regexp.MustCompile(`sk-[a-zA-Z0-9]{10,}`),
}
