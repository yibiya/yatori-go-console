package activity

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"yatori-go-console/config"
	utils2 "yatori-go-console/utils"

	"github.com/yatori-dev/yatori-go-core/aggregation/xuexitong"
	xuexitongApi "github.com/yatori-dev/yatori-go-core/api/xuexitong"
	lg "github.com/yatori-dev/yatori-go-core/utils/log"
)

// XXTActivity 学习通账号活动
type XXTActivity struct {
	UserActivityBase
}

// XXTAbility 学习通扩展能力
type XXTAbility interface {
	PullCourseList() ([]xuexitong.XueXiTCourse, error) //拉取课程列表
}

// 用户登录模块
func (activity *XXTActivity) Login() error {
	cache := &xuexitongApi.XueXiTUserCache{Name: activity.User.Account, Password: activity.User.Password}
	//设置代理IP
	if activity.User.IsProxy == 1 {
		cache.IpProxySW = true
		cache.ProxyIP = "http://" + utils2.RandProxyStr()
	}
	if err := xuexitong.XueXiTLoginAction(cache); err != nil {
		lg.Print(lg.INFO, "[", lg.Green, cache.Name, lg.White, "] ", lg.Red, err.Error())
		return err
	}
	activity.UserCache = cache
	return nil
}

// Start 启动刷课
func (activity *XXTActivity) Start() error {
	// 账号配置了运行时间窗口时，仅在窗口内才真正开刷
	if err := CheckRunWindow(activity.User); err != nil {
		return err
	}
	if activity.UserCache == nil {
		if err := activity.Login(); err != nil {
			return err
		}
	}
	activity.IsRunning = true
	defer activity.Stop()
	return activity.userBlock()
}

// Stop 停止刷课
func (activity *XXTActivity) Stop() error {
	activity.IsRunning = false
	return nil
}

// IsActive 当前是否正在运行
func (activity *XXTActivity) IsActive() bool { return activity.IsRunning }

// GetUser 获取账号配置
func (activity *XXTActivity) GetUser() config.User { return activity.User }

// SetUser 更新账号配置（面板保存配置后调用，下次启动生效）
func (activity *XXTActivity) SetUser(user config.User) { activity.User = user }

// PullCourseList 拉取课程列表（供面板预览课程）
func (activity *XXTActivity) PullCourseList() ([]xuexitong.XueXiTCourse, error) {
	if activity.UserCache == nil {
		if err := activity.Login(); err != nil {
			return nil, err
		}
	}
	cache, ok := activity.UserCache.(*xuexitongApi.XueXiTUserCache)
	if !ok || cache == nil {
		return nil, fmt.Errorf("学习通用户缓存未初始化")
	}
	courseList, err := xuexitong.XueXiTPullCourseAction(cache)
	if err != nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.Red, "拉取课程失败：", err.Error())
		return nil, err
	}
	return courseList, nil
}

// userBlock 以账号为单位的刷课基本块
func (activity *XXTActivity) userBlock() error {
	cache, ok := activity.UserCache.(*xuexitongApi.XueXiTUserCache)
	if !ok || cache == nil {
		return fmt.Errorf("学习通用户缓存未初始化")
	}
	courseList, err := xuexitong.XueXiTPullCourseAction(cache)
	if !activity.IsRunning {
		return nil
	}
	if err != nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.Red, "拉取课程失败：", err.Error())
		return err
	}

	for _, course := range courseList {
		if !activity.IsRunning {
			return nil
		}
		activity.nodeListStudy(cache, &course)
		if !activity.IsRunning {
			return nil
		}
		activity.writeCourseWorkAndExam(cache, &course)
	}
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.Purple, "所有待学习课程学习完毕")
	//如果开启了邮箱通知
	if setting := readSetting(); setting.EmailInform.Sw == 1 && len(activity.User.InformEmails) > 0 {
		utils2.SendMail(setting.EmailInform.SMTPHost, setting.EmailInform.SMTPPort, setting.EmailInform.UserName, setting.EmailInform.Password, activity.User.InformEmails, fmt.Sprintf("账号：[%s]</br>平台：[%s]</br>通知：所有课程已执行完毕", activity.User.Account, utils2.AccountTypeName(activity.User.AccountType)))
	}
	return nil
}

// nodeListStudy 课程任务点学习（复用 logic/xuexitong 的完整实现，保证与命令行模式行为一致）
func (activity *XXTActivity) nodeListStudy(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse) {
	// 排除指定课程
	if len(activity.User.CoursesCustom.ExcludeCourses) != 0 && config.CmpCourse(courseItem.CourseName, activity.User.CoursesCustom.ExcludeCourses) {
		return
	}
	//包含指定课程
	if len(activity.User.CoursesCustom.IncludeCourses) != 0 && !config.CmpCourse(courseItem.CourseName, activity.User.CoursesCustom.IncludeCourses) {
		return
	}
	if !activity.IsRunning {
		return
	}
	//如果课程还未开课则直接退出
	if !courseItem.IsStart {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Blue, "该课程还未开课，已自动跳过该课程")
		return
	}
	//如果该课程已经结束
	if courseItem.State == 1 {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Blue, "该课程已经结束，已自动跳过该课程")
		return
	}

	key, _ := strconv.Atoi(courseItem.Key)
	action, _, err := xuexitong.PullCourseChapterAction(cache, courseItem.Cpi, key)
	if !activity.IsRunning {
		return
	}
	if err != nil {
		// 章节为空是正常状态，不是拉取失败：老师建了课但没往章节里放内容。
		// 课程列表接口的 jobcount 同时为 0，两个独立接口互相印证
		// （实测 2026-11-01，53 门课里 24 门如此）。用信息级日志，
		// 避免每轮刷课刷出十几条红字把真故障淹没。
		var emptyChapters xuexitong.ChaptersEmpty
		if errors.As(err, &emptyChapters) {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Blue, "该课程暂无章节内容，已自动跳过")
			return
		}
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.BoldRed, "拉取章节信息失败，已跳过该课程：", err.Error())
		return
	}
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", "获取课程章节成功 (共 ", lg.Yellow, strconv.Itoa(len(action.Knowledge)), lg.Default, " 个) ")

	var nodes []int
	for _, item := range action.Knowledge {
		nodes = append(nodes, item.ID)
	}

	courseId, _ := strconv.Atoi(courseItem.CourseID)
	userId, _ := strconv.Atoi(cache.UserID)
	// 检测节点完成情况
	pointAction, err := xuexitong.ChapterFetchPointAction(cache, nodes, &action, key, userId, courseItem.Cpi, courseId)
	if err != nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.BoldRed, "探测节点完成情况失败，已跳过该课程：", err.Error())
		return
	}
	isFinished := func(index int) bool {
		if index < 0 || index >= len(pointAction.Knowledge) {
			return false
		}
		i := pointAction.Knowledge[index]
		if i.PointTotal == 0 && i.PointFinished == 0 {
			//0 任务点：直接遍历主页面完成任务
			if err2 := xuexitong.EnterChapterForwardCallAction(cache, courseItem.CourseID, strconv.Itoa(key), strconv.Itoa(pointAction.Knowledge[index].ID), strconv.Itoa(courseItem.Cpi)); err2 != nil {
				lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.BoldRed, "零任务点遍历失败：", err2.Error())
			}
		}
		return i.PointTotal >= 0 && i.PointTotal == i.PointFinished
	}

	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Purple, "正在学习该课程")

	for index := range nodes {
		if isFinished(index) {
			continue
		}
		if !activity.IsRunning {
			return
		}
		activity.nodeRun(cache, courseItem, pointAction, action, nodes, index, key, courseId)
	}
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Purple, "课程学习完毕")
}

// nodeRun 任务点分流运行
func (activity *XXTActivity) nodeRun(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, pointAction xuexitong.ChaptersList, action xuexitong.ChaptersList, nodes []int, index, key, courseId int) {
	_, fetchCards, resourceDTOs, err1 := xuexitong.ChapterFetchCardsAction(cache, &action, nodes, index, courseId, key, courseItem.Cpi)
	if err1 != nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.BoldRed, "无法拉取卡片信息，已跳过该任务点：", err1.Error())
		return
	}
	videoDTOs, workDTOs, documentDTOs, hyperlinkDTOs, liveDTOs, bbsDTOs := xuexitongApi.ParsePointDto(fetchCards)
	if !activity.IsRunning {
		return
	}
	// 资源型条目（资料下载 / 知识点球图）单独处理：没有 jobid，走访问记录接口
	activity.executeResources(cache, courseItem, pointAction.Knowledge[index], resourceDTOs)
	if videoDTOs == nil && workDTOs == nil && documentDTOs == nil && hyperlinkDTOs == nil && liveDTOs == nil && bbsDTOs == nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", "[", pointAction.Knowledge[index].Name, "] ", lg.Blue, "课程对应章节无任何任务节点，已自动跳过")
		return
	}

	var wg sync.WaitGroup
	// 视频
	for i := range videoDTOs {
		videoDTO := &videoDTOs[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			activity.executeVideo(cache, courseItem, pointAction.Knowledge[index], videoDTO, key, courseId)
		}()
	}
	// 文档
	for i := range documentDTOs {
		documentDTO := &documentDTOs[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			activity.executeDocument(cache, courseItem, pointAction.Knowledge[index], documentDTO, key, courseId)
		}()
	}
	// 外链
	for i := range hyperlinkDTOs {
		hyperlinkDTO := &hyperlinkDTOs[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			activity.executeHyperlink(cache, courseItem, pointAction.Knowledge[index], hyperlinkDTO, key, courseId)
		}()
	}
	// 直播
	for i := range liveDTOs {
		liveDTO := &liveDTOs[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			activity.executeLive(cache, courseItem, pointAction.Knowledge[index], liveDTO, key, courseId)
		}()
	}
	// 章测
	setting := readSetting()
	if workDTOs != nil && activity.User.CoursesCustom.AutoExam != 0 && activity.User.CoursesCustom.CxChapterTestSw != nil && *activity.User.CoursesCustom.CxChapterTestSw == 1 {
		if err := ensureAnswerBackend(setting, activity.User.CoursesCustom.AutoExam); err != nil {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.BoldRed, err.Error())
		} else {
			for i := range workDTOs {
				if !activity.IsRunning {
					break
				}
				workDTO := &workDTOs[i]
				card, _, err2 := xuexitong.PageMobileChapterCardAction(cache, key, courseId, workDTO.KnowledgeID, workDTO.CardIndex, courseItem.Cpi)
				if err2 != nil {
					continue
				}
				workDTO.AttachmentsDetection(card)
				if !workDTO.IsJob {
					continue
				}
				questionAction, err := xuexitong.ParseWorkQuestionAction(cache, workDTO)
				if err != nil {
					lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", "[", pointAction.Knowledge[index].Name, "] ", lg.Red, "拉取章节作业失败：", err.Error())
					continue
				}
				activity.chapterTestAction(cache, courseItem, pointAction.Knowledge[index], questionAction, setting)
			}
		}
	}
	// 讨论
	if bbsDTOs != nil && activity.User.CoursesCustom.AutoExam != 0 {
		for i := range bbsDTOs {
			if !activity.IsRunning {
				break
			}
			bbsDTO := &bbsDTOs[i]
			wg.Add(1)
			go func() {
				defer wg.Done()
				activity.executeBBS(cache, courseItem, pointAction.Knowledge[index], bbsDTO, setting, key, courseId)
			}()
		}
	}
	wg.Wait()
}

// CheckRunWindow 校验账号的自动运行时间窗口。
// 两端都留空表示不限制；只填一端或格式非法视为配置错误；
// 配置了窗口但当前不在窗口内时返回错误，由调用方拒绝启动。
func CheckRunWindow(user config.User) error {
	start := strings.TrimSpace(user.CoursesCustom.AutoRunStartTime)
	end := strings.TrimSpace(user.CoursesCustom.AutoRunEndTime)
	if start == "" && end == "" {
		return nil // 不限制
	}
	if start == "" || end == "" {
		return errors.New("开始时间与结束时间需要同时填写，否则都留空表示不限制")
	}
	startMinutes, ok1 := parseHHmmMinutes(start)
	endMinutes, ok2 := parseHHmmMinutes(end)
	if !ok1 || !ok2 {
		return errors.New("运行时间窗口配置有误，应为 HH:mm 格式")
	}

	loc := time.Local
	now := time.Now().In(loc)
	nowMinutes := now.Hour()*60 + now.Minute()

	// start <= end 为普通窗口；跨天窗口（start > end，如 22:00~06:00）按取反判断
	inWindow := startMinutes <= endMinutes && nowMinutes >= startMinutes && nowMinutes <= endMinutes
	if !inWindow && startMinutes > endMinutes {
		inWindow = nowMinutes >= startMinutes || nowMinutes <= endMinutes
	}
	if !inWindow {
		return fmt.Errorf("当前时间 %s 不在该账号的运行时间窗口 [%s ~ %s] 内", now.Format("15:04"), start, end)
	}
	return nil
}

// parseHHmmMinutes 解析 HH:mm 为当日分钟数，非法返回 false
func parseHHmmMinutes(v string) (int, bool) {
	t, err := time.Parse("15:04", v)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}
