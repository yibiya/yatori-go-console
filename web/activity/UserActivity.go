package activity

import "yatori-go-console/config"

// Activity 统一接口：每个账号一个活动实例，负责登录、启动、停止与状态查询。
// Web 面板的"启动/停止"按钮以及命令行模式都通过本接口驱动单账号刷课。
type Activity interface {
	Login() error   //登录
	Start() error   //启动刷课
	Stop() error    //停止刷课
	IsActive() bool //当前是否正在运行
	GetUser() config.User
	SetUser(user config.User)
}
