package global

import (
	"encoding/json"
	"log"
	"strings"

	"yatori-go-console/config"
	"yatori-go-console/dao"
	"yatori-go-console/entity/pojo"
	"yatori-go-console/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 账号存储：Web 模式下 config.yaml 中的 users 段不再生效，账号全部由 SQLite (yatori.db) 管理。
// 本文件负责 DB <-> config.User 的互相转换、config.yaml 首次迁移，以及运行期账号快照。

// ReloadUsers 从数据库重新加载账号，作为后续刷课/面板的唯一数据源。
// Web 模式启动时调用一次；每次账号增删改后也调用，保证内存快照与 DB 一致。
func ReloadUsers(db *gorm.DB) []config.User {
	users, err := LoadUsers(db)
	if err != nil {
		log.Printf("从数据库加载账号失败: %v", err)
		return nil
	}
	SetUsers(users)
	return users
}

// LoadUsers 拉取数据库中所有账号并转换为 config.User
func LoadUsers(db *gorm.DB) ([]config.User, error) {
	pos, err := dao.QueryAllUsers(db)
	if err != nil {
		return nil, err
	}
	users := make([]config.User, 0, len(pos))
	for i := range pos {
		users = append(users, PosToUser(&pos[i]))
	}
	return users, nil
}

// PosToUser 将数据库实体转换为 config.User
func PosToUser(po *pojo.UserPO) config.User {
	user := po.UserConfigTurnEntity()
	// 基础字段以数据库列为准，避免旧 JSON 里残留过期账号信息
	user.AccountType = po.AccountType
	user.URL = po.Url
	user.Account = po.Account
	user.Password = po.Password
	if user.RemarkName == "" {
		user.RemarkName = po.Account
	}
	return user
}

// UserToPos 将 config.User 转换为数据库实体
func UserToPos(user config.User) pojo.UserPO {
	configJson, err := json.Marshal(user)
	if err != nil {
		log.Printf("序列化用户配置失败 account=%s: %v", user.Account, err)
		configJson = []byte("{}")
	}
	return pojo.UserPO{
		AccountType:    user.AccountType,
		Url:            user.URL,
		Account:        user.Account,
		Password:       user.Password,
		UserConfigJson: string(configJson),
	}
}

// NewUid 生成账号唯一 ID（UUID v7，按序递增，列表排序稳定）
func NewUid() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString()
	}
	return id.String()
}

// MigrateConfigUsers 首次启动（数据库无账号）时把 config.yaml 中的 users 迁入数据库。
// 之后 config.yaml 的 users 段被忽略，账号统一由数据库管理。
// 返回迁入的账号数量。
func MigrateConfigUsers(db *gorm.DB, configUsers []config.User) int {
	pos := make([]pojo.UserPO, 0, len(configUsers))
	seen := map[string]struct{}{}
	for i := range configUsers {
		user := configUsers[i]
		if strings.TrimSpace(user.Account) == "" {
			continue
		}
		if !isKnownAccountType(user.AccountType) {
			log.Printf("跳过未知平台类型账号 account=%s type=%s", user.Account, user.AccountType)
			continue
		}
		// config.yaml 迁移进来的账号一律视为启用（enabled 缺省）
		user.SetEnabled(true)
		key := user.AccountType + "|" + user.Account
		if _, ok := seen[key]; ok {
			log.Printf("跳过重复账号 account=%s type=%s", user.Account, user.AccountType)
			continue
		}
		seen[key] = struct{}{}
		po := UserToPos(user)
		po.Uid = NewUid()
		pos = append(pos, po)
	}
	if len(pos) == 0 {
		return 0
	}
	if err := dao.InsertUsers(db, pos); err != nil {
		log.Printf("迁移 config.yaml 账号到数据库失败: %v", err)
		return 0
	}
	log.Printf("已将 config.yaml 中的 %d 个账号迁移到数据库管理", len(pos))
	return len(pos)
}

func isKnownAccountType(accountType string) bool {
	return utils.IsKnownAccountType(accountType)
}

// NormalizeEnabled 为历史账号补全 enabled 字段，返回补全数量。
// 老版本写入的 user_config_json 里没有 enabled，反序列化后为 nil；
// 一次性补成显式的"启用"，之后启停状态就有明确来源，不再依赖缺省推断。
func NormalizeEnabled(db *gorm.DB) int {
	users, err := dao.QueryAllUsers(db)
	if err != nil {
		log.Printf("归一化启用状态时读取账号失败: %v", err)
		return 0
	}
	fixed := 0
	for i := range users {
		user := users[i].UserConfigTurnEntity()
		if user.Enabled != nil {
			continue // 已显式设置过，保持原值
		}
		user.SetEnabled(true)
		configJson, err := json.Marshal(user)
		if err != nil {
			continue
		}
		if err := dao.UpdateUser(db, users[i].Uid, map[string]interface{}{"user_config_json": string(configJson)}); err != nil {
			log.Printf("补全账号 %s 的启用状态失败: %v", users[i].Account, err)
			continue
		}
		fixed++
	}
	return fixed
}
