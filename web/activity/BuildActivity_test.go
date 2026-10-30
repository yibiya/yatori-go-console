package activity

import (
	"testing"
	"yatori-go-console/config"
	"yatori-go-console/entity/pojo"
)

// TestBuildUserActivitySupported 面板已接入的平台必须能构建出可用实例。
func TestBuildUserActivitySupported(t *testing.T) {
	po := pojo.UserPO{Uid: "u1", AccountType: "XUEXITONG", Account: "13320169850", Password: "pw", UserConfigJson: "{}"}
	act := BuildUserActivity(po)
	if act == nil {
		t.Fatal("XUEXITONG 应能构建活动实例")
	}
	if act.IsActive() {
		t.Error("新构建的实例不应处于运行中")
	}
	// GetUser 必须反映账号信息，否则面板标题/日志前缀会错
	if got := act.GetUser().Account; got != "13320169850" {
		t.Errorf("GetUser().Account = %q", got)
	}
}

// TestBuildUserActivityUnsupported 未接入平台必须返回 nil，
// 让接口层回"暂不支持启动"，而不是给出假成功。
func TestBuildUserActivityUnsupported(t *testing.T) {
	for _, accountType := range []string{"YINGHUA", "CANGHUI", "ENAEA", "CQIE", "KETANGX", "WELEARN", "ICVE", "QSXT", "HQKJ", "NOPE", ""} {
		po := pojo.UserPO{Uid: "u-" + accountType, AccountType: accountType, Account: "a", Password: "pw", UserConfigJson: "{}"}
		if act := BuildUserActivity(po); act != nil {
			t.Errorf("%q 不应构建出活动实例（当前未接入）", accountType)
		}
	}
}

// TestEnsureReusesInstance 同一 uid 重复 Ensure 必须复用同一实例，
// 否则"启动/停止"会作用到不同对象上，面板状态与真实任务脱节。
func TestEnsureReusesInstance(t *testing.T) {
	const uid = "ensure-uid"
	defer Delete(uid)

	po := pojo.UserPO{Uid: uid, AccountType: "XUEXITONG", Account: "13320169850", Password: "pw", UserConfigJson: "{}"}
	first := Ensure(po)
	if first == nil {
		t.Fatal("Ensure 应构建实例")
	}
	second := Ensure(po)
	if first != second {
		t.Fatal("同一 uid 应复用同一实例")
	}
	if Get(uid) != first {
		t.Error("Get 应返回已注册的实例")
	}

	Delete(uid)
	if Get(uid) != nil {
		t.Error("Delete 后应取不到实例")
	}
}

// TestRegistryStopUnknown 停止未运行的账号不能让接口误报成功
func TestRegistryStopUnknown(t *testing.T) {
	if Stop("never-registered-uid") {
		t.Error("未注册的 uid 不应报告停止成功")
	}
}

// TestSetUserUpdatesInstance 面板保存配置后，已注册实例应立即看到新配置
func TestSetUserUpdatesInstance(t *testing.T) {
	const uid = "setuser-uid"
	defer Delete(uid)

	Ensure(pojo.UserPO{Uid: uid, AccountType: "XUEXITONG", Account: "13320169850", Password: "old", UserConfigJson: "{}"})

	updated := config.User{AccountType: "XUEXITONG", Account: "13320169850", Password: "new"}
	SetUser(uid, updated)

	if got := Get(uid).GetUser().Password; got != "new" {
		t.Errorf("实例内 Password = %q, 期望 new", got)
	}
}
