package service

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"yatori-go-console/config"
	"yatori-go-console/dao"
	"yatori-go-console/entity/pojo"
	"yatori-go-console/entity/vo"
	"yatori-go-console/global"
	"yatori-go-console/utils"
	"yatori-go-console/web/activity"

	"github.com/gin-gonic/gin"
)

// 拉取账号列表（含每个账号的运行状态）
func UserListService(c *gin.Context) {
	users, err := dao.QueryAllUsers(global.GlobalDB)
	if err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: err.Error()})
		return
	}
	resUserList := []map[string]any{}
	for i := range users {
		user := users[i]
		userConfig := user.UserConfigTurnEntity()
		remark := userConfig.RemarkName
		if remark == "" {
			remark = user.Account
		}
		isRunning := false
		if act := activity.Get(user.Uid); act != nil && act.IsActive() {
			isRunning = true
		}
		// 前端直接消费这些字段，不让它去猜 userConfigJson，
		// 否则"已启用/已停用"会因为解析差异显示成相反的状态。
		runWindow := ""
		cc := userConfig.CoursesCustom
		if cc.AutoRunStartTime != "" || cc.AutoRunEndTime != "" {
			runWindow = "⏱ " + orDash(cc.AutoRunStartTime) + " ~ " + orDash(cc.AutoRunEndTime)
		}
		resUserList = append(resUserList, map[string]any{
			"runWindow":    runWindow,
			"uid":          user.Uid,
			"account":      user.Account,
			"accountType":  user.AccountType,
			"url":          user.Url,
			"remarkName":   remark,
			"enabled":      userConfig.IsEnabled(),
			"isRunning":    isRunning,
			"isProxy":      userConfig.IsProxy,
			"platformName": utils.AccountTypeName(user.AccountType),
		})
	}
	c.JSON(200, vo.Response{
		Code:    200,
		Message: "拉取账号成功",
		Data: gin.H{
			"users": resUserList,
			"total": len(resUserList),
		},
	})
}

// 添加账号
func AddUserService(c *gin.Context) {
	var req vo.AddAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "msg": "请求参数错误: " + err.Error()})
		return
	}
	req.Account = strings.TrimSpace(req.Account)
	if req.Account == "" {
		c.JSON(200, gin.H{"code": 400, "msg": "账号不能为空"})
		return
	}
	if _, ok := global.AccountTypeStr[req.AccountType]; !ok {
		c.JSON(200, gin.H{"code": 400, "msg": "未知的平台类型: " + req.AccountType})
		return
	}
	// 同平台同账号判重
	exist, _ := dao.QueryUser(global.GlobalDB, pojo.UserPO{
		AccountType: req.AccountType,
		Url:         req.Url,
		Account:     req.Account,
	})
	if exist != nil {
		c.JSON(200, gin.H{"code": 400, "msg": "该账号已存在"})
		return
	}

	userConfig := configUserFromRequest(req)
	userPo := global.UserToPos(userConfig)
	userPo.Uid = global.NewUid()
	if err := dao.InsertUser(global.GlobalDB, &userPo); err != nil {
		c.JSON(200, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	global.ReloadUsers(global.GlobalDB)
	c.JSON(200, vo.Response{Code: 200, Message: "添加账号成功", Data: &userPo})
}

// 删除账号
func DeleteUserService(c *gin.Context) {
	var req vo.DeleteAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: "数据转换失败"})
		return
	}
	if req.Uid == "" && (req.AccountType == "" || req.Account == "") {
		c.JSON(200, vo.Response{Code: 400, Message: "uid 或 (accountType+account) 至少提供一组"})
		return
	}
	if err := dao.DeleteUser(global.GlobalDB, &pojo.UserPO{
		Uid:         req.Uid,
		AccountType: req.AccountType,
		Url:         req.Url,
		Account:     req.Account,
	}); err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: "删除失败"})
		return
	}
	// 清理账号活动缓存，避免残留旧实例
	activity.Delete(req.Uid)
	global.ReloadUsers(global.GlobalDB)
	c.JSON(200, vo.Response{Code: 200, Message: "删除成功"})
}

// 检查账号密码是否正确
func AccountLoginCheckService(c *gin.Context) {
	var req vo.AccountLoginCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: "请求参数错误: " + err.Error()})
		return
	}

	userConfig := config.User{
		AccountType: req.AccountType,
		URL:         req.Url,
		Account:     req.Account,
		Password:    req.Password,
	}
	act := activity.BuildUserActivityFromConfig(userConfig)
	if act == nil {
		c.JSON(200, vo.Response{Code: 400, Message: "暂不支持该平台的登录检测: " + req.AccountType})
		return
	}
	if err := act.Login(); err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: "账号登录失败: " + err.Error()})
		return
	}
	c.JSON(200, vo.Response{Code: 200, Message: "账号登录正常"})
}

// 获取账号基础信息（含启停状态）
func GetAccountInformService(c *gin.Context) {
	uid := c.Param("uid")
	user, err := dao.QueryUser(global.GlobalDB, pojo.UserPO{Uid: uid})
	if err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: err.Error()})
		return
	}
	if user == nil {
		c.JSON(200, gin.H{"code": 400, "msg": "该账号不存在"})
		return
	}
	resp := gin.H{"code": 200, "msg": "拉取信息成功", "data": gin.H{"user": user}}
	if act := activity.Get(user.Uid); act != nil {
		resp["isRunning"] = act.IsActive()
	} else {
		resp["isRunning"] = false
	}
	c.JSON(200, resp)
}

// 保存账号刷课配置（coursesCustom + 运行时间窗口），单账号独立配置
func SaveAccountConfigService(c *gin.Context) {
	var req vo.UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: "JSON 解析失败: " + err.Error()})
		return
	}
	if req.Uid == "" {
		c.JSON(200, vo.Response{Code: 400, Message: "UID 不能为空"})
		return
	}
	if err := validateTimeWindow(req.CoursesCustom.AutoRunStartTime, req.CoursesCustom.AutoRunEndTime); err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: err.Error()})
		return
	}

	userPo, err := dao.QueryUser(global.GlobalDB, pojo.UserPO{Uid: req.Uid})
	if err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: err.Error()})
		return
	}
	if userPo == nil {
		c.JSON(200, vo.Response{Code: 400, Message: "该账号不存在"})
		return
	}

	userConfig := userPo.UserConfigTurnEntity()
	userConfig.AccountType = userPo.AccountType
	userConfig.URL = userPo.Url
	userConfig.Account = userPo.Account
	userConfig.Password = userPo.Password
	userConfig.RemarkName = req.RemarkName
	userConfig.IsProxy = req.IsProxy
	if userConfig.RemarkName == "" {
		userConfig.RemarkName = userPo.Account
	}
	if req.InformEmails != nil {
		userConfig.InformEmails = req.InformEmails
	}
	if req.CoursesCustom.CxNode == nil {
		// 未显式指定同时任务点数时保持默认值 3，兼容旧数据
		req.CoursesCustom.CxNode = &[]int{0, 0, 0, 3}[3]
	}
	userConfig.CoursesCustom = req.CoursesCustom
	// 启停状态由专门的 toggle 接口控制：保存刷课配置不应顺带改变账号是否启用，
	// 否则前端少传一个字段就会把账号悄悄停用。
	if req.Enabled != nil {
		userConfig.SetEnabled(*req.Enabled == 1)
	}

	userConfigJson, err := jsonMarshal(userConfig)
	if err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: err.Error()})
		return
	}

	updateMap := map[string]interface{}{
		"account_type":     userPo.AccountType,
		"url":              userPo.Url,
		"account":          userPo.Account,
		"password":         userPo.Password,
		"remark_name":      userConfig.RemarkName,
		"user_config_json": userConfigJson,
	}
	if err := dao.UpdateUser(global.GlobalDB, req.Uid, updateMap); err != nil {
		c.JSON(200, vo.Response{Code: 500, Message: err.Error()})
		return
	}
	// 同步内存中的活动实例配置，下次启动生效
	activity.SetUser(req.Uid, userConfig)
	global.ReloadUsers(global.GlobalDB)
	c.JSON(200, gin.H{"code": 200, "msg": "保存成功"})
}

// 获取账号刷课配置
func GetAccountConfigService(c *gin.Context) {
	uid := c.Param("uid")
	userPo, err := dao.QueryUser(global.GlobalDB, pojo.UserPO{Uid: uid})
	if err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: err.Error()})
		return
	}
	if userPo == nil {
		c.JSON(200, vo.Response{Code: 400, Message: "该账号不存在"})
		return
	}
	userConfig := userPo.UserConfigTurnEntity()
	c.JSON(200, vo.Response{
		Code:    200,
		Message: "拉取配置成功",
		Data: gin.H{
			"uid":           userPo.Uid,
			"accountType":   userPo.AccountType,
			"platformName":  utils.AccountTypeName(userPo.AccountType),
			"url":           userPo.Url,
			"account":       userPo.Account,
			"remarkName":    userConfig.RemarkName,
			"isProxy":       userConfig.IsProxy,
			"enabled":       userConfig.IsEnabled(),
			"informEmails":  userConfig.InformEmails,
			"coursesCustom": userConfig.CoursesCustom,
		},
	})
}

// 启用/停用账号：停用后命令行批量模式与面板启动都会跳过该账号
func ToggleAccountEnabledService(c *gin.Context) {
	var req struct {
		Uid     string `json:"uid"`
		Enabled int    `json:"enabled"` //1启用 0停用
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: "请求参数错误: " + err.Error()})
		return
	}
	if req.Uid == "" {
		c.JSON(200, vo.Response{Code: 400, Message: "UID 不能为空"})
		return
	}
	if req.Enabled != 0 && req.Enabled != 1 {
		c.JSON(200, vo.Response{Code: 400, Message: "enabled 只能为 0 或 1"})
		return
	}
	userPo, err := dao.QueryUser(global.GlobalDB, pojo.UserPO{Uid: req.Uid})
	if err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: err.Error()})
		return
	}
	if userPo == nil {
		c.JSON(200, vo.Response{Code: 400, Message: "该账号不存在"})
		return
	}
	userConfig := userPo.UserConfigTurnEntity()
	userConfig.SetEnabled(req.Enabled == 1)
	userConfigJson, err := jsonMarshal(userConfig)
	if err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: err.Error()})
		return
	}
	if err := dao.UpdateUser(global.GlobalDB, req.Uid, map[string]interface{}{"user_config_json": userConfigJson}); err != nil {
		c.JSON(200, vo.Response{Code: 500, Message: err.Error()})
		return
	}
	// 停用同时停止该账号正在运行的任务，避免"停了还在跑"的歧义
	if req.Enabled == 0 {
		activity.Stop(req.Uid)
	}
	global.ReloadUsers(global.GlobalDB)
	state := "启用"
	if req.Enabled == 0 {
		state = "停用"
	}
	c.JSON(200, vo.Response{Code: 200, Message: "账号已" + state})
}

// orDash 空时间显示为占位符
func orDash(v string) string {
	if v == "" {
		return "--:--"
	}
	return v
}

// jsonMarshal 序列化账号配置
func jsonMarshal(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ErrorStr 构造简单错误
func ErrorStr(msg string) error {
	return errors.New(msg)
}

var hhmmPattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// isHHmm 判断是否为合法的 HH:mm
func isHHmm(v string) bool {
	if !hhmmPattern.MatchString(v) {
		return false
	}
	// 额外用 time.Parse 拦住 25:99 之类的组合
	_, err := time.Parse("15:04", v)
	return err == nil
}

// configUserFromRequest 从添加账号请求构造 config.User
func configUserFromRequest(req vo.AddAccountRequest) config.User {
	user := config.User{
		AccountType: req.AccountType,
		URL:         req.Url,
		RemarkName:  req.RemarkName,
		Account:     req.Account,
		Password:    req.Password,
		IsProxy:     req.IsProxy,
		Enabled:     config.NewEnabled(true),
	}
	if user.RemarkName == "" {
		user.RemarkName = req.Account
	}
	if req.InformEmails != nil {
		user.InformEmails = req.InformEmails
	}
	if req.CoursesCustom != nil {
		user.CoursesCustom = *req.CoursesCustom
	} else {
		user.CoursesCustom = defaultCoursesCustom()
	}
	return user
}

// defaultCoursesCustom 新账号默认刷课配置：普通刷视频 + 默认任务点行为
func defaultCoursesCustom() config.CoursesCustom {
	cxNode, cxChapterTestSw, cxWorkSw, cxExamSw := 3, 1, 1, 1
	return config.CoursesCustom{
		CxNode:          &cxNode,
		CxChapterTestSw: &cxChapterTestSw,
		CxWorkSw:        &cxWorkSw,
		CxExamSw:        &cxExamSw,
		VideoModel:      1,
		AutoExam:        0,
		ExamAutoSubmit:  0,
	}
}

// validateTimeWindow 校验 HH:mm 形式的运行时间窗口
func validateTimeWindow(start, end string) error {
	if start == "" && end == "" {
		return nil
	}
	if start == "" || end == "" {
		return ErrorStr("开始时间与结束时间需要同时填写，否则都留空表示不限制")
	}
	if !isHHmm(start) || !isHHmm(end) {
		return ErrorStr("时间格式应为 HH:mm，例如 00:30")
	}
	return nil
}

// 更新账号基础信息（账号/密码/平台/URL）
func UpdateUserService(c *gin.Context) {
	var req vo.UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: "JSON 解析失败: " + err.Error()})
		return
	}
	if req.Uid == "" {
		c.JSON(200, vo.Response{Code: 400, Message: "UID 不能为空"})
		return
	}
	userPo, err := dao.QueryUser(global.GlobalDB, pojo.UserPO{Uid: req.Uid})
	if err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: err.Error()})
		return
	}
	if userPo == nil {
		c.JSON(200, vo.Response{Code: 400, Message: "该账号不存在"})
		return
	}

	userConfig := userPo.UserConfigTurnEntity()
	userConfig.AccountType = req.AccountType
	userConfig.URL = req.Url
	userConfig.Account = req.Account
	userConfig.Password = req.Password
	if req.RemarkName != "" {
		userConfig.RemarkName = req.RemarkName
	} else if userConfig.RemarkName == "" {
		userConfig.RemarkName = req.Account
	}
	userConfig.IsProxy = req.IsProxy
	if req.InformEmails != nil {
		userConfig.InformEmails = req.InformEmails
	}
	if req.CoursesCustom.CxNode == nil && userConfig.CoursesCustom.CxNode == nil {
		cxNode := 3
		userConfig.CoursesCustom.CxNode = &cxNode
	}
	if req.CoursesCustom.VideoModel != 0 || req.CoursesCustom.AutoExam != 0 {
		userConfig.CoursesCustom.VideoModel = req.CoursesCustom.VideoModel
		userConfig.CoursesCustom.AutoExam = req.CoursesCustom.AutoExam
		userConfig.CoursesCustom.ExamAutoSubmit = req.CoursesCustom.ExamAutoSubmit
	}
	userConfigJson, err := jsonMarshal(userConfig)
	if err != nil {
		c.JSON(200, vo.Response{Code: 400, Message: err.Error()})
		return
	}

	updateMap := map[string]interface{}{
		"account_type":     req.AccountType,
		"url":              req.Url,
		"account":          req.Account,
		"password":         req.Password,
		"remark_name":      userConfig.RemarkName,
		"user_config_json": userConfigJson,
	}
	if err := dao.UpdateUser(global.GlobalDB, req.Uid, updateMap); err != nil {
		c.JSON(200, vo.Response{Code: 500, Message: err.Error()})
		return
	}
	// 基础信息变化后旧缓存失效，清理让下次启动用新配置
	activity.Delete(req.Uid)
	global.ReloadUsers(global.GlobalDB)
	c.JSON(200, vo.Response{Code: 200, Message: "更新成功"})
}
