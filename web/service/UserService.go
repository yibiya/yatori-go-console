package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"yatori-go-console/config"
	"yatori-go-console/dao"
	"yatori-go-console/entity/pojo"
	"yatori-go-console/entity/vo"
	"yatori-go-console/global"
	"yatori-go-console/utils"
	"yatori-go-console/web/activity"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 拉取账号列表
func UserListService(c *gin.Context) {
	users, total, err := dao.QueryUsers(global.GlobalDB, 1, 10)
	if err != nil {
		c.JSON(http.StatusOK, vo.Response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}
	//转换列表--------------
	resUserList := []map[string]any{}
	for _, user := range users {
		toMap := utils.StructToMap(user)
		userActivity := global.GetUserActivity(user)
		if userActivity != nil {
			if xxt, ok := (*userActivity).(*activity.XXTActivity); ok {
				toMap["isRunning"] = xxt.IsRunning
			}
		} else {
			toMap["isRunning"] = false
		}

		resUserList = append(resUserList, toMap)
	}
	c.JSON(http.StatusOK, vo.Response{
		Code:    200,
		Message: "拉取账号成功",
		Data: gin.H{
			"users": resUserList,
			"total": total,
		},
	})
}

// 添加账号
func AddUserService(c *gin.Context) {
	// 1. 定义结构体用于接收 JSON
	var req vo.AddAccountRequest
	// 2. 解析 JSON 到结构体
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"code": 400,
			"msg":  "请求参数错误: " + err.Error(),
		})
		return
	}
	//检测账号是否已存在
	userPo := pojo.UserPO{
		AccountType: req.AccountType,
		Url:         req.Url,
		Account:     req.Account,
	}
	user, _ := dao.QueryUser(global.GlobalDB, userPo)
	if user != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 400,
			"msg":  "该账号已存在",
		})
		return
	}

	uuidV7, _ := uuid.NewV7()
	userPo.Uid = uuidV7.String()   //设置uuid值
	userPo.Password = req.Password //设置密码
	userConfig := config.User{
		AccountType: userPo.AccountType,
		URL:         userPo.Url,
		Account:     userPo.Account,
		Password:    userPo.Password,
	}

	userConfigJson, err2 := json.Marshal(userConfig)
	if err2 != nil {
		c.JSON(http.StatusOK, vo.Response{
			Code:    400,
			Message: err2.Error(),
		})
		return
	}
	userPo.UserConfigJson = string(userConfigJson) //赋值Config配置

	err := dao.InsertUser(global.GlobalDB, &userPo)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}
	//登录成功
	c.JSON(200,
		vo.Response{
			Code:    200,
			Message: "添加账号成功",
			Data:    &userPo,
		})
}

// 删除账号
func DeleteUserService(c *gin.Context) {
	var req vo.DeleteAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, vo.Response{
			Code:    400,
			Message: "数据转换失败",
		})
		return
	}
	//如果uid不为空则采用uid方式删除
	if req.Uid != "" {
		err := dao.DeleteUser(global.GlobalDB, &pojo.UserPO{Uid: req.Uid})
		if err != nil {
			c.JSON(http.StatusOK, vo.Response{
				Code:    400,
				Message: "删除失败",
			})
			return
		}
	} else if req.AccountType != "" && req.Account != "" { //如果uid方式没有，则直接使用账号和账号类型方式联合查询删除
		err := dao.DeleteUser(global.GlobalDB, &pojo.UserPO{
			AccountType: req.AccountType,
			Url:         req.Url,
			Account:     req.Account,
		})
		if err != nil {
			c.JSON(http.StatusOK, vo.Response{
				Code:    400,
				Message: "删除失败",
			})
			return
		}
	}

	c.JSON(200,
		vo.Response{
			Code:    200,
			Message: "删除成功",
		})
}

// 检查账号密码是否正确
func AccountLoginCheckService(c *gin.Context) {
	// 1. 定义结构体用于接收 JSON
	var req vo.AccountLoginCheckRequest
	// 2. 解析 JSON 到结构体
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK,
			vo.Response{
				Code:    400,
				Message: "请求参数错误: " + err.Error(),
			})
		return
	}
	//如果是uid检测登录则先查询
	if req.Uid != "" {
		user, _ := dao.QueryUser(global.GlobalDB, pojo.UserPO{Uid: req.Uid})
		if user == nil {
			c.JSON(http.StatusOK,
				vo.Response{
					Code:    400,
					Message: "该账号不存在",
				})
			return
		}
		//登录逻辑......
	}

	c.JSON(http.StatusOK, vo.Response{
		Code:    200,
		Message: "账号登录正常",
	})
}

// 获取账号配置信息
func GetAccountInformService(c *gin.Context) {
	uid := c.Param("uid")
	//检测账号是否已存在
	user, _ := dao.QueryUser(global.GlobalDB, pojo.UserPO{
		Uid: uid,
	})
	if user == nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 400,
			"msg":  "该账号不存在",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "拉取信息成功",
		"data": gin.H{
			"user": user,
		},
	})
}

// 登录账号
func LoginUserService(c *gin.Context) {
	// 1. 定义结构体用于接收 JSON
	var req pojo.UserPO
	// 2. 解析 JSON 到结构体
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"code": 400,
			"msg":  "请求参数错误: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "登录成功",
	})
}

// 更新账号信息
func UpdateUserService(c *gin.Context) {
	var req pojo.UserPO

	// 绑定 JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		data, _ := c.GetRawData()
		fmt.Println(string(data))
		c.JSON(http.StatusOK, vo.Response{
			Code:    400,
			Message: "JSON 解析失败",
		})
		return
	}

	// Uid 必须存在
	if req.Uid == "" {
		c.JSON(http.StatusOK, vo.Response{
			Code:    400,
			Message: "UID 不能为空",
		})
		return
	}

	// 将结构体转为 map 并过滤空字段
	updateMap := make(map[string]interface{})

	// 手动挑选可修改字段（最安全方式）
	if req.AccountType != "" {
		updateMap["account_type"] = req.AccountType
	}
	if req.Url != "" {
		updateMap["url"] = req.Url
	}
	if req.Account != "" {
		updateMap["account"] = req.Account
	}
	if req.Password != "" {
		updateMap["password"] = req.Password
	}

	// 空字段检查
	if len(updateMap) == 0 {
		c.JSON(200, vo.Response{
			Code:    400,
			Message: "没有可更新的字段",
		})
		return
	}

	// 调用 DAO 更新
	if err := dao.UpdateUser(global.GlobalDB, req.Uid, updateMap); err != nil {
		c.JSON(http.StatusOK, vo.Response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "更新成功",
	})
}

// 拉取课程列表
func AccountCourseListService(c *gin.Context) {
	// 2. 解析 JSON 到结构体
	uid := c.Param("uid")
	//检测账号是否已存在
	user, _ := dao.QueryUser(global.GlobalDB, pojo.UserPO{
		Uid: uid,
	})
	if user == nil {
		c.JSON(http.StatusOK, vo.Response{
			Code:    400,
			Message: "该账号不存在",
		})
		return
	}

	userActivity := global.GetUserActivity(*user)
	//如果没有活动中的账号则添加活动账号
	if userActivity == nil {
		//构建用户活动
		createActivity := activity.BuildUserActivity(*user)
		userActivity = &createActivity
		global.PutUserActivity(*user, &createActivity)
	}
	//如果是学习通
	if xxt, ok1 := (*userActivity).(activity.XXTAbility); ok1 {
		list, err := xxt.PullCourseList()
		if err != nil {
			//拉取失败时返回错误码与原因，避免前端把空列表当成成功
			c.JSON(http.StatusOK, vo.Response{
				Code:    500,
				Message: "拉取课程失败: " + err.Error(),
			})
			return
		}
		//转换为标准类型的列表数据
		courseList := []vo.CourseInformResponse{}
		for _, course := range list {
			courseList = append(courseList, vo.CourseInformResponse{
				CourseId:   course.CourseID,
				CourseName: course.CourseName,
				Progress:   float32(course.JobRate),
				Instructor: course.CourseTeacher,
			})
		}
		//fmt.Println(list)
		c.JSON(http.StatusOK, vo.Response{
			Code:    200,
			Message: "拉取信息成功",
			Data:    gin.H{"courseList": courseList},
		})
	}

}

// 开始刷课
func StartBrushService(c *gin.Context) {
	uid := c.Param("uid")
	user, err := dao.QueryUser(global.GlobalDB, pojo.UserPO{
		Uid: uid,
	})
	if err != nil {
		c.JSON(http.StatusOK, vo.Response{
			Code:    400,
			Message: err.Error(),
		})
		return
	}
	userActivity := global.GetUserActivity(*user)
	if userActivity != nil {
		//userActivity.IsRunning = false
	}
	go func() {
		// 调用Start方法
		(*userActivity).Start()
	}()

	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "启动成功",
	})
}

// 停止任务
func StopBrushService(c *gin.Context) {
	uid := c.Param("uid")
	user, err := dao.QueryUser(global.GlobalDB, pojo.UserPO{
		Uid: uid,
	})
	if err != nil {
		c.JSON(400, gin.H{})
	}
	if user == nil {
		c.JSON(400, gin.H{})
		return
	}
	userActivity := global.GetUserActivity(*user)
	if userActivity == nil {
		c.JSON(400, gin.H{})
	}
	// 根据账号类型断言为具体活动类型并设置IsRunning
	if xxt, ok := (*userActivity).(*activity.XXTActivity); ok {
		xxt.IsRunning = false
	} else if yinghua, ok := (*userActivity).(*activity.YingHuaActivity); ok {
		yinghua.IsRunning = true
	}
	(*userActivity).Stop()
	//userActivity.Kill()
	c.JSON(http.StatusOK, vo.Response{
		Code:    200,
		Message: "停止成功",
	})
}

// accountLogUIDPattern 账号 uid 格式校验，避免拼路径时被注入
var accountLogUIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// AccountLogsService 读取 ./assets/log 下日志，按 uid/账号/备注名过滤并脱敏，
// 返回按时间倒序（最新在前）排列的最近 maxLines 条，供前端账号详情页展示。
func AccountLogsService(c *gin.Context) {
	uid := c.Param("uid")
	if !accountLogUIDPattern.MatchString(uid) {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: "uid 格式错误"})
		return
	}

	user, err := dao.QueryUser(global.GlobalDB, pojo.UserPO{Uid: uid})
	if err != nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: err.Error()})
		return
	}
	if user == nil {
		c.JSON(http.StatusOK, vo.Response{Code: 400, Message: "该账号不存在"})
		return
	}

	identifiers := []string{uid, user.Account}
	if remark := strings.TrimSpace(user.UserConfigTurnEntity().RemarkName); remark != "" && remark != user.Account {
		identifiers = append(identifiers, remark)
	}

	logs, err := readAccountLogs("./assets/log", identifiers, 500)
	if err != nil {
		// 读不到日志不算接口失败：返回空列表，前端提示"暂无日志"
		fmt.Printf("读取账号日志失败 uid=%s: %v\n", uid, err)
	}

	c.JSON(http.StatusOK, vo.Response{Code: 200, Message: "拉取日志成功", Data: map[string]any{
		"uid":  uid,
		"logs": logs,
	}})
}

// readAccountLogs 扫描 log*.txt，保留含任一标识符的行，脱敏后按时间倒序（最新在前）返回最近 maxLines 条。
func readAccountLogs(logDir string, identifiers []string, maxLines int) (string, error) {
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
	if len(files) > 0 {
		// 旧 -> 新，配合后续按行内时间戳排序，保证输出时序稳定
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime().Before(files[j].ModTime())
		})
	}

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

	// 日志里可能出现 AI Key / 密码等敏感值，统一打码
	desensitizeKV := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(api[_-]?key)\s*[:=]\s*[^\s]+`),
		regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*[^\s]+`),
		regexp.MustCompile(`(?i)(token|secret)\s*[:=]\s*[^\s]+`),
	}
	skRe := regexp.MustCompile(`sk-[a-zA-Z0-9]{10,}`)

	var matched []string
	for _, info := range files {
		file, err := os.Open(filepath.Join(logDir, info.Name()))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !matchRe.MatchString(line) {
				continue
			}
			for _, re := range desensitizeKV {
				line = re.ReplaceAllString(line, "$1: ***")
			}
			line = skRe.ReplaceAllString(line, "sk-***")
			matched = append(matched, line)
		}
		file.Close()
	}

	// 多账号并发写同一文件，行序可能乱；按行内时间戳稳定排序
	lineTS := regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]`)
	tsOf := func(s string) string {
		if m := lineTS.FindStringSubmatch(s); m != nil {
			return m[1]
		}
		return ""
	}
	sort.SliceStable(matched, func(i, j int) bool {
		return tsOf(matched[i]) < tsOf(matched[j])
	})
	if maxLines > 0 && len(matched) > maxLines {
		matched = matched[len(matched)-maxLines:]
	}
	// 前端日志视图按人类阅读习惯展示：最新一条在最上方，向上滚动回溯更早记录
	for i, j := 0, len(matched)-1; i < j; i, j = i+1, j-1 {
		matched[i], matched[j] = matched[j], matched[i]
	}
	return strings.Join(matched, "\n"), nil
}
