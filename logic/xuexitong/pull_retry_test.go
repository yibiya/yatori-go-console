package xuexitong

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"yatori-go-console/config"

	"github.com/yatori-dev/yatori-go-core/api/xuexitong"
)

// fakeErr 用于错误分类测试
type fakeErr struct{ msg string }

func (e *fakeErr) Error() string { return e.msg }

// TestIsRetryableCourseError 覆盖错误分类：
// 线上导致进程退出的是 "Client.Timeout exceeded while awaiting headers" 这类
// 网络错误，必须被判定为可重试；解析失败等业务错误不应重试。
func TestIsRetryableCourseError(t *testing.T) {
	retryable := []string{
		// 线上真实报错：mooc2-ans 完成状态接口卡死
		`[15960600263] 拉取课程完成状态失败: Get "https://mooc2-ans.chaoxing.com/mooc2-ans/mycourse/stu-job-info?clazzPersonStr=151175286_488305110": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`,
		// 线上真实报错：mooc1-api 课程列表卡死
		`[15960600263] 拉取课程失败: Get "https://mooc1-api.chaoxing.com/mycourse/backclazzdata": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`,
		// 线上真实报错：登录接口连接中断
		`[13320169850] Post "https://passport2.chaoxing.com/fanyalogin": EOF`,
		// Go 对代理/目标不可达产生的真实错误文本
		`Get "https://mooc1-api.chaoxing.com/mycourse/backclazzdata": proxyconnect tcp: dial tcp 127.0.0.1:1: connect: connection refused`,
		`read tcp 10.11.63.39:50388->45.113.20.54:443: read: connection reset by peer`,
		`拉取课程完成状态: HTTP 503`,
		`拉取课程完成状态: dial tcp: lookup mooc2-ans.chaoxing.com: no such host`,
	}
	for _, msg := range retryable {
		if !isRetryableCourseError(&fakeErr{msg}) {
			t.Errorf("should be retryable: %s", msg)
		}
	}

	nonRetryable := []string{
		`[15960600263] 解析失败: unexpected end of JSON input`,
		`[15960600263] 课程验证码处理失败: 识别失败`,
		`[15960600263] 拉取课程完成状态: 业务请求被拒绝`,
	}
	for _, msg := range nonRetryable {
		if isRetryableCourseError(&fakeErr{msg}) {
			t.Errorf("should NOT be retryable: %s", msg)
		}
	}
}

// TestPullCourseListRetryReturnsErrorInsteadOfExiting 复现线上进程退出问题：
// 完成状态接口一直失败时，pullCourseListWithRetry 必须把错误交回调用方，
// 而不是让进程终止。用可快速失败的桩服务器（连接即拒）保证测试确定且迅速。
func TestPullCourseListRetryReturnsErrorInsteadOfExiting(t *testing.T) {
	// 立刻关闭的服务器：连接被拒，等价于网络不可达的快速失败版本
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadAddr := srv.Listener.Addr().String()
	srv.Close()

	u, err := url.Parse("http://" + deadAddr)
	if err != nil {
		t.Fatalf("parse addr: %v", err)
	}
	cache := &xuexitong.XueXiTUserCache{
		Name:      "repro",
		Password:  "x",
		IpProxySW: true,
		ProxyIP:   u.Host,
	}

	start := time.Now()
	_, pullErr := pullCourseListWithRetry(cache)
	elapsed := time.Since(start)

	// 核心断言：拿到了错误而不是进程被杀掉（旧代码在这里 log.Fatal）
	if pullErr == nil {
		t.Fatal("expected error from unreachable endpoint, got nil")
	}
	// 重试之间有 5s 退避，说明真的重试了而不是一次就放弃
	if elapsed < 5*time.Second {
		t.Fatalf("expected one retry with backoff, returned in %v", elapsed)
	}
	// 也不能无限重试把调用方挂住
	if elapsed > 90*time.Second {
		t.Fatalf("retry took too long (%v), would block the scan loop", elapsed)
	}
	t.Logf("survived failure: err=%v elapsed=%v", pullErr, elapsed)
}

// TestPullCourseListRetryStopsOnNonRetryable 保证业务错误不被盲目重试。
// 解析失败等错误应立即返回，不做无谓等待（由分类单测覆盖判定，
// 这里只验证非网络错误不会触发重试等待）。
func TestPullCourseListRetryStopsOnNonRetryable(t *testing.T) {
	// 分类为不可重试时，即使底层持续失败也不应出现重试退避。
	// 用一个必然解析失败但网络通畅的本地服务器来验证快速返回。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json-at-all`))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	cache := &xuexitong.XueXiTUserCache{
		Name:      "repro-nonretry",
		Password:  "x",
		IpProxySW: true,
		ProxyIP:   u.Host,
	}

	start := time.Now()
	_, err := pullCourseListWithRetry(cache)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if isRetryableCourseError(err) {
		t.Fatalf("parse failure should not be classified retryable: %v", err)
	}
	// 不可重试错误必须快速返回，不能出现 5s 退避
	if elapsed > 2*time.Second {
		t.Fatalf("non-retryable error took %v; should fail fast without backoff", elapsed)
	}
}

// TestUserBlockSurvivesPullFailure 验证 UserBlock 在拉取失败时正常返回
// 并释放 usersLock，不会 panic 也不会挂住调用方。
func TestUserBlockSurvivesPullFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadAddr := srv.Listener.Addr().String()
	srv.Close()

	u, _ := url.Parse("http://" + deadAddr)
	cache := &xuexitong.XueXiTUserCache{
		Name:      "repro-block",
		Password:  "x",
		IpProxySW: true,
		ProxyIP:   u.Host,
	}

	// VideoModel=1 走串行分支，CxNode 非空避免解引用空指针
	zero := 0
	user := &config.User{
		AccountType: "XUEXITONG",
		Account:     "repro-block",
		CoursesCustom: config.CoursesCustom{
			VideoModel: 1,
			CxNode:     &zero,
		},
	}

	usersLock.Add(1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		UserBlock(config.Setting{}, user, cache)
	}()

	select {
	case <-done:
		// UserBlock 返回了，说明没有 fatal、没有 panic
	case <-time.After(2 * time.Minute):
		t.Fatal("UserBlock did not return after pull failure; scan loop would be stuck")
	}
}

var _ = fmt.Sprintf
var _ = errors.New
