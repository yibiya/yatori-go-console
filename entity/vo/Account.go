package vo

import "yatori-go-console/config"

type Response struct {
	Code    int    `json:"code"`    //状态码
	Message string `json:"message"` //返回信息
	Data    any    `json:"data"`    //主要的数据
}

// 添加账号请求
type AddAccountRequest struct {
	AccountType   string                `json:"accountType"`
	Url           string                `json:"url"`
	Account       string                `json:"account"`
	Password      string                `json:"password"`
	RemarkName    string                `json:"remarkName"`
	IsProxy       int                   `json:"isProxy"`
	InformEmails  []string              `json:"informEmails"`
	CoursesCustom *config.CoursesCustom `json:"coursesCustom"`
}

// 删除账号请求
type DeleteAccountRequest struct {
	Uid         string `json:"uid"`
	AccountType string `json:"accountType"`
	Url         string `json:"url"`
	Account     string `json:"account"`
}

// 更新账号请求
type UpdateAccountRequest struct {
	Uid           string               `json:"uid"`
	AccountType   string               `json:"accountType"`
	Url           string               `json:"url"`
	Account       string               `json:"account"`
	Password      string               `json:"password"`
	RemarkName    string               `json:"remarkName"`
	IsProxy       int                  `json:"isProxy"`
	Enabled       *int                 `json:"enabled"` //1启用 0停用；nil 表示不改动（保存刷课配置时不动启停状态）
	InformEmails  []string             `json:"informEmails"`
	CoursesCustom config.CoursesCustom `json:"coursesCustom"`
}

// 账号登录检测请求
type AccountLoginCheckRequest struct {
	Uid         string `json:"uid"`
	AccountType string `json:"accountType"`
	Url         string `json:"url"`
	Account     string `json:"account"`
	Password    string `json:"password"`
}

// 课程信息响应
type CourseInformResponse struct {
	CourseId   string  `json:"courseId"`
	CourseName string  `json:"courseName"`
	Progress   float32 `json:"progress"`
	Instructor string  `json:"instructor"`
}
