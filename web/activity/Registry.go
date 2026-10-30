package activity

import (
	"sync"
	"yatori-go-console/config"
	"yatori-go-console/entity/pojo"
)

// 账号活动注册表：uid -> Activity。
// 放在 activity 包内而不是 global，避免 global -> web/activity 的反向依赖
// （logic/* 平台实现依赖 global，若 global 再依赖 web/activity 会形成环）。
var (
	registryMu sync.RWMutex
	registry   = map[string]Activity{}
)

// Get 取账号活动，不存在返回 nil
func Get(uid string) Activity {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[uid]
}

// Put 注册账号活动（同一 uid 重复注册会覆盖旧实例）
func Put(uid string, act Activity) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[uid] = act
}

// Delete 移除账号活动（账号删除或基础信息变更后调用，防止残留旧缓存）
func Delete(uid string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	delete(registry, uid)
}

// Ensure 取账号活动，不存在则按账号类型构建并注册；构建失败返回 nil。
func Ensure(userPo pojo.UserPO) Activity {
	if act := Get(userPo.Uid); act != nil {
		return act
	}
	created := BuildUserActivity(userPo)
	if created == nil {
		return nil
	}
	Put(userPo.Uid, created)
	return created
}

// Stop 停止指定 uid 的账号活动，返回是否存在并已停止
func Stop(uid string) bool {
	act := Get(uid)
	if act == nil {
		return false
	}
	return act.Stop() == nil
}

// SetUser 同步账号最新配置到已注册的活动实例（面板保存配置后调用）
func SetUser(uid string, user config.User) {
	if act := Get(uid); act != nil {
		act.SetUser(user)
	}
}
