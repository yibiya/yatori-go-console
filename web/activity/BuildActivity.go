package activity

import (
	"encoding/json"

	"yatori-go-console/config"
	"yatori-go-console/entity/pojo"
)

// UserActivityBase 账号活动基础信息（运行状态 + 用户配置缓存）
type UserActivityBase struct {
	User      config.User //账号配置
	IsRunning bool        //是否正在运行
	UserCache any         //平台登录缓存
}

// GetUser 获取账号配置
func (u *UserActivityBase) GetUser() config.User { return u.User }

// SetUser 更新账号配置
func (u *UserActivityBase) SetUser(user config.User) { u.User = user }

// BuildUserActivity 按账号类型构建活动实例。
// 目前 Web 面板端已完整接入：学习通(XUEXITONG)。
// 其余平台（英华/仓辉/学习公社/CQIE/码上研训/WeLearn/智慧职教/青书学堂/海旗科技）
// 仍走命令行模式刷课，面板会对这些账号明确返回"暂不支持启动"而不是假装成功。
// 后续接入新平台时在这里补一个 case 即可。
func BuildUserActivity(po pojo.UserPO) Activity {
	user := po.UserConfigTurnEntity()
	user.AccountType = po.AccountType
	user.URL = po.Url
	user.Account = po.Account
	user.Password = po.Password
	if user.RemarkName == "" {
		user.RemarkName = po.Account
	}

	switch po.AccountType {
	case "XUEXITONG":
		return &XXTActivity{
			UserActivityBase: UserActivityBase{
				User:      user,
				IsRunning: false,
				UserCache: nil,
			},
		}
	default:
		return nil
	}
}

// BuildUserActivityFromConfig 与 BuildUserActivity 等价，但直接接受 config.User
func BuildUserActivityFromConfig(user config.User) Activity {
	po := pojo.UserPO{
		AccountType:    user.AccountType,
		Url:            user.URL,
		Account:        user.Account,
		Password:       user.Password,
		UserConfigJson: mustMarshalUser(user),
	}
	return BuildUserActivity(po)
}

// mustMarshalUser 序列化账号配置，失败时返回空对象（由调用方按零值处理）
func mustMarshalUser(user config.User) string {
	data, err := json.Marshal(user)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// jsonMarshalUser 序列化账号配置
func jsonMarshalUser(user config.User) ([]byte, error) {
	return json.Marshal(user)
}
