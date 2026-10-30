package web

import (
	"log"
	"yatori-go-console/config"
	"yatori-go-console/dao"
	"yatori-go-console/global"
)

// InitAccountDB 打开账号数据库并按需迁移 config.yaml 的账号。
// 命令行模式与 Web 模式共用，保证两种模式看到同一份账号数据。
func InitAccountDB(configData *config.JSONDataForConfig) bool {
	db, err := dao.SqliteInit()
	if err != nil {
		log.Printf("账号数据库初始化失败: %v", err)
		return false
	}
	global.GlobalDB = db

	empty, err := dao.IsUserTableEmpty(db)
	if err != nil {
		log.Printf("检查账号表失败: %v", err)
	}
	if empty && configData != nil {
		if n := global.MigrateConfigUsers(db, configData.Users); n > 0 {
			log.Printf("已把 config.yaml 中的 %d 个账号迁移到数据库", n)
		}
	}
	// 归一化历史账号：老数据的 user_config_json 里没有 enabled 字段，
	// 不显式补成 1 的话，每次读回来都像"未设置"，
	// 面板/命令行对启停的判断会和实际状态脱节。
	if fixed := global.NormalizeEnabled(db); fixed > 0 {
		log.Printf("已把 %d 个历史账号的启用状态补全为启用", fixed)
	}
	global.ReloadUsers(db)
	return true
}
