package global

import (
	"testing"

	"yatori-go-console/config"
	"yatori-go-console/entity/pojo"
)

// TestPosToUser 数据库列优先于可能过期的 JSON 配置。
// 面板改账号/密码只写列，不重写整份 JSON 时会读到旧值，这里守住这个契约。
func TestPosToUser(t *testing.T) {
	po := newTestUserPO("321560cb", "XUEXITONG", "13320169850", "newpass")
	po.UserConfigJson = `{"account":"OLD","password":"OLDPASS","remarkName":"备注甲","isProxy":1,"enabled":1}`

	user := PosToUser(po)

	if user.Account != "13320169850" {
		t.Errorf("Account = %q, 期望取数据库列 13320169850", user.Account)
	}
	if user.Password != "newpass" {
		t.Errorf("Password = %q, 期望取数据库列 newpass", user.Password)
	}
	if user.RemarkName != "备注甲" {
		t.Errorf("RemarkName = %q, 期望保留 JSON 中的备注名 备注甲", user.RemarkName)
	}
	if user.IsProxy != 1 {
		t.Errorf("IsProxy = %d, 期望保留 JSON 中的 1", user.IsProxy)
	}
}

// TestPosToUserRemarkFallback 备注名为空时回退为账号，保证面板与日志都有可读名称
func TestPosToUserRemarkFallback(t *testing.T) {
	po := newTestUserPO("009eef97", "XUEXITONG", "15960600263", "pw")
	po.UserConfigJson = `{"account":"15960600263","password":"pw"}`

	if got := PosToUser(po).RemarkName; got != "15960600263" {
		t.Errorf("RemarkName = %q, 期望回退为账号", got)
	}
}

// TestPosToUserBrokenJson 配置 JSON 损坏不能 panic，必须按零值降级
func TestPosToUserBrokenJson(t *testing.T) {
	po := newTestUserPO("badjson", "XUEXITONG", "18020890183", "pw")
	po.UserConfigJson = "{not-json"

	user := PosToUser(po)
	if user.Account != "18020890183" {
		t.Errorf("Account = %q, 期望仍取数据库列", user.Account)
	}
}

// TestUserToPosRoundTrip 账号配置写库再读回应保持一致（面板保存后刷新不能丢字段）
func TestUserToPosRoundTrip(t *testing.T) {
	cxNode, cxWork := 3, 1
	user := config.User{
		AccountType: "XUEXITONG",
		Account:     "13320169850",
		Password:    "pw",
		RemarkName:  "一号账号",
		IsProxy:     1,
		Enabled:     config.NewEnabled(true),
		CoursesCustom: config.CoursesCustom{
			VideoModel:       2,
			AutoExam:         3,
			CxNode:           &cxNode,
			CxWorkSw:         &cxWork,
			IncludeCourses:   []string{"高等数学"},
			AutoRunStartTime: "08:30",
			AutoRunEndTime:   "22:00",
		},
	}

	po := UserToPos(user)
	back := PosToUser(&po)

	if back.CoursesCustom.VideoModel != 2 || back.CoursesCustom.AutoExam != 3 {
		t.Errorf("刷课模式丢失: %+v", back.CoursesCustom)
	}
	if back.CoursesCustom.CxNode == nil || *back.CoursesCustom.CxNode != 3 {
		t.Errorf("CxNode 丢失: %+v", back.CoursesCustom.CxNode)
	}
	if len(back.CoursesCustom.IncludeCourses) != 1 || back.CoursesCustom.IncludeCourses[0] != "高等数学" {
		t.Errorf("IncludeCourses 丢失: %v", back.CoursesCustom.IncludeCourses)
	}
	if back.CoursesCustom.AutoRunStartTime != "08:30" || back.CoursesCustom.AutoRunEndTime != "22:00" {
		t.Errorf("运行时间窗口丢失: %s ~ %s", back.CoursesCustom.AutoRunStartTime, back.CoursesCustom.AutoRunEndTime)
	}
	if !back.IsEnabled() || back.RemarkName != "一号账号" {
		t.Errorf("启用状态或备注名丢失: enabled=%d remark=%s", back.Enabled, back.RemarkName)
	}
}

// TestIsKnownAccountType 面板下拉与后端校验必须用同一份平台表
func TestIsKnownAccountType(t *testing.T) {
	for _, known := range []string{"XUEXITONG", "YINGHUA", "CANGHUI", "ENAEA", "CQIE", "KETANGX", "ICVE", "QSXT", "WELEARN", "HQKJ"} {
		if !isKnownAccountType(known) {
			t.Errorf("%s 应识别为已知平台", known)
		}
	}
	for _, unknown := range []string{"", "xuexitong", "NOPE", "  "} {
		if isKnownAccountType(unknown) {
			t.Errorf("%q 不应识别为已知平台", unknown)
		}
	}
}

func newTestUserPO(uid, accountType, account, password string) *pojo.UserPO {
	return &pojo.UserPO{
		Uid:            uid,
		AccountType:    accountType,
		Account:        account,
		Password:       password,
		UserConfigJson: "{}",
	}
}

// TestPosToUserLegacyDefaultsEnabled 老账号的 user_config_json 里没有 enabled 字段，
// 必须按"启用"处理，而不是反序列化成 0 显示成已停用。
func TestPosToUserLegacyDefaultsEnabled(t *testing.T) {
	po := newTestUserPO("legacy-1", "XUEXITONG", "13320169850", "pw")
	// 与 v2.6.2 生产库中实际存在的历史数据形状一致（无 enabled）
	po.UserConfigJson = `{"accountType":"XUEXITONG","account":"13320169850","password":"pw","isProxy":0,` +
		`"coursesCustom":{"cxNode":0,"videoModel":1,"autoExam":1}}`

	user := PosToUser(po)
	if !user.IsEnabled() {
		t.Error("老账号没有 enabled 字段时必须视为启用")
	}
}

// TestPosToUserExplicitDisabledKept 显式写 0 的账号必须保持停用
func TestPosToUserExplicitDisabledKept(t *testing.T) {
	po := newTestUserPO("off-1", "XUEXITONG", "13320169850", "pw")
	po.UserConfigJson = `{"account":"13320169850","password":"pw","enabled":0}`

	user := PosToUser(po)
	if user.IsEnabled() {
		t.Error("显式 enabled:0 的账号必须保持停用")
	}
}

// TestEnabledRoundTrip 启停状态写入再读出必须保持一致
func TestEnabledRoundTrip(t *testing.T) {
	for _, on := range []bool{true, false} {
		user := config.User{Account: "a", AccountType: "XUEXITONG"}
		user.SetEnabled(on)
		pos := UserToPos(user)
		back := PosToUser(&pos)
		if back.IsEnabled() != on {
			t.Errorf("SetEnabled(%v) 往返后变成 %v", on, back.IsEnabled())
		}
	}
}
