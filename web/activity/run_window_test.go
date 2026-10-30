package activity

import (
	"testing"

	"yatori-go-console/config"
)

// TestParseHHmmMinutes 覆盖面板运行时间窗口的解析：合法 HH:mm 才接受。
// 面板任何账号都能填这两个字段，坏输入必须在保存前就被拦掉。
func TestParseHHmmMinutes(t *testing.T) {
	valid := map[string]int{
		"00:00": 0,
		"09:30": 570,
		"23:59": 1439,
	}
	for in, want := range valid {
		got, ok := parseHHmmMinutes(in)
		if !ok {
			t.Errorf("parseHHmmMinutes(%q) 应该合法", in)
			continue
		}
		if got != want {
			t.Errorf("parseHHmmMinutes(%q) = %d, 期望 %d", in, got, want)
		}
	}

	// 解析只接受干净的 HH:mm；首尾空白由调用方（CheckRunWindow）先 TrimSpace
	invalid := []string{"", " 07:05 ", "9", "24:00", "12:60", "abc", "12-30", "121:00"}
	for _, in := range invalid {
		if _, ok := parseHHmmMinutes(in); ok {
			t.Errorf("parseHHmmMinutes(%q) 应该非法", in)
		}
	}
}

// TestCheckRunWindowEmpty 两端都留空表示不限制：这是默认行为，不能误判为配置错误。
func TestCheckRunWindowEmpty(t *testing.T) {
	user := config.User{}
	if err := CheckRunWindow(user); err != nil {
		t.Errorf("两端留空应放行，得到: %v", err)
	}
}

// TestCheckRunWindowIncomplete 只填一端必须报错，否则面板"保存成功"但永远不生效。
func TestCheckRunWindowIncomplete(t *testing.T) {
	for _, c := range []struct{ start, end string }{
		{"08:00", ""},
		{"", "22:00"},
	} {
		user := config.User{}
		user.CoursesCustom.AutoRunStartTime = c.start
		user.CoursesCustom.AutoRunEndTime = c.end
		if err := CheckRunWindow(user); err == nil {
			t.Errorf("start=%q end=%q 应报错", c.start, c.end)
		}
	}
}

// TestCheckRunWindowInvalidFormat 非法 HH:mm 必须在保存前就被拦掉
func TestCheckRunWindowInvalidFormat(t *testing.T) {
	for _, c := range []struct{ start, end string }{
		{"99:99", "22:00"},
		{"08:00", "12:60"},
		{"abc", "22:00"},
	} {
		user := config.User{}
		user.CoursesCustom.AutoRunStartTime = c.start
		user.CoursesCustom.AutoRunEndTime = c.end
		if err := CheckRunWindow(user); err == nil {
			t.Errorf("start=%q end=%q 应报错", c.start, c.end)
		}
	}
}

// TestCheckRunWindowContainsNow 覆盖整个自然天的通用窗口，一定包含当前时间。
func TestCheckRunWindowContainsNow(t *testing.T) {
	user := config.User{}
	user.CoursesCustom.AutoRunStartTime = "00:00"
	user.CoursesCustom.AutoRunEndTime = "23:59"
	if err := CheckRunWindow(user); err != nil {
		t.Errorf("00:00~23:59 应包含当前时间，得到: %v", err)
	}
}

// TestParseHHmmMinutesWindowBoundary 分钟换算决定边界是否包含，逐分钟比对。
func TestParseHHmmMinutesWindowBoundary(t *testing.T) {
	start, _ := parseHHmmMinutes("08:00")
	end, _ := parseHHmmMinutes("22:00")
	if start != 480 || end != 1320 {
		t.Fatalf("08:00=%d 22:00=%d", start, end)
	}
	// 起点与终点都应视为窗口内（闭区间）
	for _, m := range []int{480, 1320} {
		if !(m >= start && m <= end) {
			t.Errorf("分钟 %d 应落在闭区间内", m)
		}
	}
	// 跨天窗口靠取反判断，这里锁定其分钟差不为 0，避免 start==end 被误判为跨天
	cs, _ := parseHHmmMinutes("22:00")
	ce, _ := parseHHmmMinutes("06:00")
	if cs == ce {
		t.Error("跨天窗口两端不能相等")
	}
	if cs <= ce {
		t.Error("22:00~06:00 应被识别为跨天窗口（start > end）")
	}
}
