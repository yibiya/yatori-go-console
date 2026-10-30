package dao

import (
	"errors"
	"time"
	"yatori-go-console/entity/pojo"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 初始化Sqlite
func SqliteInit() (*gorm.DB, error) {
	// 表名沿用历史版本默认的 user_pos：生产库已有该表与既有账号，
	// 若改成 user_po 会把老账号"藏起来"并触发重复迁移。
	db, err := gorm.Open(sqlite.Open("yatori.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	//自动创建/升级User表
	err = db.AutoMigrate(&pojo.UserPO{})
	if err != nil {
		return nil, err
	}
	// 兼容历史库：早期版本没有 remark_name 列，补一个可空列
	if !db.Migrator().HasColumn(&pojo.UserPO{}, "remark_name") {
		if err := db.Migrator().AddColumn(&pojo.UserPO{}, "RemarkName"); err != nil {
			return nil, err
		}
	}

	sqlDB, err := db.DB() //数据库连接池
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
}

// IsUserTableEmpty 用于判断数据库中是否还没有任何账号（首次启动需要迁移 config.yaml）
func IsUserTableEmpty(db *gorm.DB) (bool, error) {
	var count int64
	if err := db.Model(&pojo.UserPO{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

// QueryAllUsers 拉取全部账号，按 uid 稳定排序，供启动时加载与面板列表使用。
func QueryAllUsers(db *gorm.DB) ([]pojo.UserPO, error) {
	var users []pojo.UserPO
	if err := db.Order("uid ASC").Find(&users).Error; err != nil {
		return nil, errors.New("查询用户失败: " + err.Error())
	}
	return users, nil
}

// QueryUsersByType 按平台类型过滤账号
func QueryUsersByType(db *gorm.DB, accountType string) ([]pojo.UserPO, error) {
	var users []pojo.UserPO
	if err := db.Where("account_type = ?", accountType).Order("uid ASC").Find(&users).Error; err != nil {
		return nil, errors.New("查询用户失败: " + err.Error())
	}
	return users, nil
}
