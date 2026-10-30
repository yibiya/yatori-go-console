package global

import (
	"sync"

	"yatori-go-console/config"
	"yatori-go-console/utils"

	"gorm.io/gorm"
)

var GlobalDB *gorm.DB //数据库挂载

// AccountTypeStr 平台类型 -> 中文名（实际定义在 utils，这里保留别名以兼容既有调用方）
var AccountTypeStr = utils.AccountTypeStr

// accountSnapshot 当前账号快照（数据库为唯一数据源）。
// 刷课执行块与面板接口都从这里读账号，避免 DB 与内存不一致。
var (
	accountSnapshotMu sync.RWMutex
	accountSnapshot   []config.User
)

// SetUsers 覆盖账号快照
func SetUsers(users []config.User) {
	accountSnapshotMu.Lock()
	defer accountSnapshotMu.Unlock()
	accountSnapshot = users
}

// GetUsers 读取账号快照副本
func GetUsers() []config.User {
	accountSnapshotMu.RLock()
	defer accountSnapshotMu.RUnlock()
	if len(accountSnapshot) == 0 {
		return nil
	}
	out := make([]config.User, len(accountSnapshot))
	copy(out, accountSnapshot)
	return out
}
