package activity

import (
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"yatori-go-console/config"
	utils2 "yatori-go-console/utils"

	"github.com/thedevsaddam/gojsonq"
	"github.com/yatori-dev/yatori-go-core/aggregation/xuexitong"
	"github.com/yatori-dev/yatori-go-core/aggregation/xuexitong/point"
	xuexitongApi "github.com/yatori-dev/yatori-go-core/api/xuexitong"
	"github.com/yatori-dev/yatori-go-core/que-core/aiq"
	"github.com/yatori-dev/yatori-go-core/que-core/external"

	"github.com/yatori-dev/yatori-go-core/utils"
	lg "github.com/yatori-dev/yatori-go-core/utils/log"
	"github.com/yatori-dev/yatori-go-core/utils/qutils"
)

type XXTActivity struct {
	UserActivityBase
}

// 学习通扩展能力
type XXTAbility interface {
	PullCourseList() ([]xuexitong.XueXiTCourse, error) //拉取课程
}

// 用户登录模块
func (activity *XXTActivity) Login() error {
	cache := &xuexitongApi.XueXiTUserCache{Name: activity.User.Account, Password: activity.User.Password}
	//设置代理IP
	if activity.User.IsProxy == 1 {
		cache.IpProxySW = true
		cache.ProxyIP = "http://" + utils2.RandProxyStr()
	}
	loginError := xuexitong.XueXiTLoginAction(cache) // 登录
	if loginError != nil {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.White, "] ", lg.Red, loginError.Error())
		return loginError
	}
	activity.UserCache = cache //赋值寄存
	return nil
}

// 启动
func (activity *XXTActivity) Start() error {
	if activity.UserCache == nil {
		if err := activity.Login(); err != nil {
			return err
		}
	}
	activity.IsRunning = true
	defer func() {
		activity.IsRunning = false
	}()
	return activity.userBlock() //开刷
}

// 暂停
func (activity *XXTActivity) Stop() error {
	activity.IsRunning = false
	return nil
}

// 拉取课程列表
func (user *XXTActivity) PullCourseList() ([]xuexitong.XueXiTCourse, error) {
	if user.UserCache == nil { //如果为空则登录
		if err := user.Login(); err != nil {
			return nil, err
		}
	}
	cache, ok := user.UserCache.(*xuexitongApi.XueXiTUserCache)
	if !ok || cache == nil {
		return nil, fmt.Errorf("学习通用户缓存未初始化")
	}
	courseList, err := xuexitong.XueXiTPullCourseAction(cache)
	if err != nil {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", lg.Red, "拉取课程失败")
		return nil, err
	}
	return courseList, nil
}

// 直接刷课
func (activity *XXTActivity) userBlock() error {
	cache, ok := activity.UserCache.(*xuexitongApi.XueXiTUserCache)
	if !ok || cache == nil {
		return fmt.Errorf("学习通用户缓存未初始化")
	}
	courseList, err := xuexitong.XueXiTPullCourseAction(cache)
	if !activity.IsRunning { //打断
		return nil
	}
	if err != nil {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", lg.Red, "拉取课程失败")
		return err
	}

	// 加载全局配置以获取 AI 等设置
	globalSetting := config.ReadConfig("./config.yaml").Setting

	var nodesLock sync.WaitGroup //视频锁
	for _, course := range courseList {
		nodesLock.Add(1)

		activity.nodeListStudy(globalSetting, &activity.User, cache, &course)
		// 写课程的作业和考试
		activity.WriteCourseWorkAndExam(globalSetting, &activity.User, cache, &course)

		nodesLock.Done()
		if !activity.IsRunning { //打断
			return nil
		}
	}
	if !activity.IsRunning { //打断
		return nil
	}
	nodesLock.Wait()
	lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", lg.Purple, "所有待学习课程学习完毕")

	return nil
}

func (activity *XXTActivity) nodeListStudy(setting config.Setting, user *config.User, userCache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse) {
	//过滤课程---------------------------------
	//排除指定课程
	if len(user.CoursesCustom.ExcludeCourses) != 0 && config.CmpCourse(courseItem.CourseName, user.CoursesCustom.ExcludeCourses) {
		return
	}
	//包含指定课程
	if len(user.CoursesCustom.IncludeCourses) != 0 && !config.CmpCourse(courseItem.CourseName, user.CoursesCustom.IncludeCourses) {
		return
	}

	if !activity.IsRunning { //打断
		return
	}
	//如果课程还未开课则直接退出
	if !courseItem.IsStart {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Blue, "该课程还未开课，已自动跳过该课程")
		return
	}
	//如果该课程已经结束
	if courseItem.State == 1 {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Blue, "该课程已经结束，已自动跳过该课程")
		return
	}

	key, _ := strconv.Atoi(courseItem.Key)
	action, _, err := xuexitong.PullCourseChapterAction(userCache, courseItem.Cpi, key) //获取对应章节信息
	if !activity.IsRunning {                                                            //打断
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "课程章节为空") {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "该课程章节为空已自动跳过")
			return
		}
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "拉取章节信息接口访问异常，若需要继续可以配置中添加排除此异常课程。返回信息：", err.Error())
		return
	}
	lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, "获取课程章节成功 (共 ", lg.Yellow, strconv.Itoa(len(action.Knowledge)), lg.Default, " 个) ")

	var nodes []int
	for _, item := range action.Knowledge {
		nodes = append(nodes, item.ID)
	}

	courseId, _ := strconv.Atoi(courseItem.CourseID)
	userId, _ := strconv.Atoi(userCache.UserID)
	// 检测节点完成情况
	pointAction, err := xuexitong.ChapterFetchPointAction(userCache, nodes, &action, key, userId, courseItem.Cpi, courseId)
	if err != nil {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "探测节点完成情况接口访问异常，若需要继续可以配置中添加排除此异常课程。返回信息：", err.Error())
		return
	}
	var isFinished = func(index int) bool {
		if index < 0 || index >= len(pointAction.Knowledge) {
			return false
		}
		i := pointAction.Knowledge[index]
		if i.PointTotal == 0 && i.PointFinished == 0 {
			//如果是0任务点，则直接浏览一遍主页面即可完成任务，不必继续下去
			err2 := xuexitong.EnterChapterForwardCallAction(userCache, strconv.Itoa(courseId), strconv.Itoa(key), strconv.Itoa(pointAction.Knowledge[index].ID), strconv.Itoa(courseItem.Cpi))
			if err2 != nil {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "零任务点遍历失败。返回信息：", err2.Error())
			}
		}
		return i.PointTotal >= 0 && i.PointTotal == i.PointFinished
	}

	lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Purple, "正在学习该课程")

	//遍历结点
	for index := range nodes {
		if isFinished(index) { //如果完成了的那么直接跳过
			continue
		}
		activity.nodeRun(setting, userCache, courseItem, pointAction, action, nodes, index, key, courseId)
	}
	lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Purple, "课程学习完毕")
}

// 任务点分流运行
func (activity *XXTActivity) nodeRun(setting config.Setting, userCache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse,
	pointAction xuexitong.ChaptersList, action xuexitong.ChaptersList, nodes []int, index int, key int, courseId int) {
	_, fetchCards, err1 := xuexitong.ChapterFetchCardsAction(userCache, &action, nodes, index, courseId, key, courseItem.Cpi)
	if err1 != nil {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "无法正常拉取卡片信息，请联系作者查明情况,报错信息：", err1.Error())
		return
	}
	videoDTOs, workDTOs, documentDTOs, hyperlinkDTOs, liveDTOs, bbsDTOs := xuexitongApi.ParsePointDto(fetchCards)
	if !activity.IsRunning { //打断
		return
	}
	if videoDTOs == nil && workDTOs == nil && documentDTOs == nil && hyperlinkDTOs == nil && liveDTOs == nil && bbsDTOs == nil {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, `[`, pointAction.Knowledge[index].Name, `] `, lg.Blue, "课程对应章节无任何任务节点，已自动跳过")
		return
	}
	// 视屏类型

	for _, videoDTO := range videoDTOs {
		if !activity.IsRunning { //打断
			return
		}
		card, enc, err2 := xuexitong.PageMobileChapterCardAction(
			userCache, key, courseId, videoDTO.KnowledgeID, videoDTO.CardIndex, courseItem.Cpi)
		if err2 != nil {
			if strings.Contains(err2.Error(), "章节未开放") {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "该章节未开放，可能是因为前面章节有任务点未学完导致后续任务点未开放，已自动跳过该任务点")
				break
			}
			if strings.Contains(err2.Error(), "没有历史人脸") {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "过人脸失败，该账号可能从未进行过人脸识别，请先进行一次人脸识别后再试")
				os.Exit(0)
			}
			if strings.Contains(err2.Error(), "活体检测失败") {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "过人脸失败，该账号所录入的人脸可能并不规范，请自行拍摄人脸放到assets/face/账号名称.jpg路径下再重试")
				os.Exit(0)
			}
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.Red, err2.Error())
			os.Exit(0)
		}
		_, err := videoDTO.AttachmentsDetection(card)
		if err != nil {
		}

		if !videoDTO.IsJob {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, `[`, pointAction.Knowledge[index].Name, `] `, lg.Blue, "该视屏非任务点或已完成，已自动跳过")
			continue
		}
		videoDTO.Enc = enc                                        //赋值enc值
		if videoDTO.IsPassed == true && videoDTO.IsJob == false { //如果已经通过了，那么直接跳过
			continue
		} else if videoDTO.IsPassed == false && videoDTO.Attachment == nil && videoDTO.JobID == "" && videoDTO.Duration <= videoDTO.PlayTime { //非任务点如果完成了
			continue
		}

		activity.ExecuteVideo2(userCache, courseItem, pointAction.Knowledge[index], &videoDTO, key, courseItem.Cpi) //普通模式
		randSleepTime := rand.Intn(51) + 10
		time.Sleep(time.Duration(randSleepTime) * time.Second)
	}
	// 文档类型
	if documentDTOs != nil {
		for _, documentDTO := range documentDTOs {
			if !activity.IsRunning { //打断
				return
			}
			card, _, err2 := xuexitong.PageMobileChapterCardAction(
				userCache, key, courseId, documentDTO.KnowledgeID, documentDTO.CardIndex, courseItem.Cpi)
			if err2 != nil {
				if strings.Contains(err2.Error(), "章节未开放") {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "该章节未开放，可能是因为前面章节有任务点未学完导致后续任务点未开放，已自动跳过该任务点")
					break
				}
				if strings.Contains(err2.Error(), "没有历史人脸") {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "过人脸失败，该账号可能从未进行过人脸识别，请先进行一次人脸识别后再试")
					os.Exit(0)
				}
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.Red, err2.Error())
				os.Exit(0)
			}
			documentDTO.AttachmentsDetection(card)
			//如果不是任务或者说该任务已完成，那么直接跳过
			if !documentDTO.IsJob {
				continue
			}
			ExecuteDocument(userCache, courseItem, pointAction.Knowledge[index], &documentDTO)
			time.Sleep(5 * time.Second)
		}
	}

	//外链任务点刷取
	if hyperlinkDTOs != nil {
		for _, hyperlinkDTO := range hyperlinkDTOs {
			if !activity.IsRunning { //打断
				return
			}
			card, _, err2 := xuexitong.PageMobileChapterCardAction(
				userCache, key, courseId, hyperlinkDTO.KnowledgeID, hyperlinkDTO.CardIndex, courseItem.Cpi)

			if err2 != nil {
				if strings.Contains(err2.Error(), "章节未开放") {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "该章节未开放，可能是因为前面章节有任务点未学完导致后续任务点未开放，已自动跳过该任务点")
					return
				}
				if strings.Contains(err2.Error(), "没有历史人脸") {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "过人脸失败，该账号可能从未进行过人脸识别，请先进行一次人脸识别后再试")
					os.Exit(0)
				}
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.Red, err2.Error())
				os.Exit(0)
			}
			hyperlinkDTO.AttachmentsDetection(card)

			ExecuteHyperlink(userCache, courseItem, pointAction.Knowledge[index], &hyperlinkDTO)
			time.Sleep(5 * time.Second)
		}
	}
	// 直播任务点刷取
	if liveDTOs != nil {
		for _, liveDTO := range liveDTOs {
			if !activity.IsRunning { //打断
				return
			}
			card, _, err2 := xuexitong.PageMobileChapterCardAction(
				userCache, key, courseId, liveDTO.KnowledgeID, liveDTO.CardIndex, courseItem.Cpi)

			if err2 != nil {
				if strings.Contains(err2.Error(), "章节未开放") {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "该章节未开放，可能是因为前面章节有任务点未学完导致后续任务点未开放，已自动跳过该任务点")
					return
				}
				if strings.Contains(err2.Error(), "没有历史人脸") {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "过人脸失败，该账号可能从未进行过人脸识别，请先进行一次人脸识别后再试")
					os.Exit(0)
				}
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.Red, err2.Error())
				os.Exit(0)
			}
			liveDTO.AttachmentsDetection(card)
			if !liveDTO.IsJob { //不是任务点或者已经是完成的任务点直接退出
				continue
			}
			ExecuteLive(userCache, courseItem, pointAction.Knowledge[index], &liveDTO)
			time.Sleep(5 * time.Second)
		}
	}
	//作业刷取
	if workDTOs != nil && (activity.User.CoursesCustom.AutoExam == 1 || activity.User.CoursesCustom.AutoExam == 2 || activity.User.CoursesCustom.AutoExam == 3) && (activity.User.CoursesCustom.CxChapterTestSw != nil && *activity.User.CoursesCustom.CxChapterTestSw == 1) {
		for _, workDTO := range workDTOs {
			if !activity.IsRunning { //打断
				return
			}
			card, _, err2 := xuexitong.PageMobileChapterCardAction(
				userCache, key, courseId, workDTO.KnowledgeID, workDTO.CardIndex, courseItem.Cpi)

			if err2 != nil {
				if strings.Contains(err2.Error(), "章节未开放") {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "该章节未开放，可能是因为前面章节有任务点未学完导致后续任务点未开放，已自动跳过该任务点")
					return
				}
				if strings.Contains(err2.Error(), "没有历史人脸") {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "过人脸失败，该账号可能从未进行过人脸识别，请先进行一次人脸识别后再试")
					os.Exit(0)
				}
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.Red, err2.Error())
				os.Exit(0)
			}
			flag, _ := workDTO.AttachmentsDetection(card)
			questionAction, err2 := xuexitong.ParseWorkQuestionAction(userCache, &workDTO)
			if err2 != nil && strings.Contains(err2.Error(), "已截止，不能作答") {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", questionAction.Title, "】", lg.Yellow, "该试卷已到截止时间，已自动跳过")
				continue
			}
			if !flag {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", questionAction.Title, "】", lg.Green, "该作业已完成，已自动跳过")
				continue
			}
			if len(questionAction.Short) == 0 && len(questionAction.Choice) == 0 &&
				len(questionAction.Judge) == 0 && len(questionAction.Fill) == 0 &&
				len(questionAction.TermExplanation) == 0 && len(questionAction.Essay) == 0 {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", questionAction.Title, "】", lg.Yellow, "该作业任务点无题目，已自动跳过")
				continue
			}

			activity.ChapterTestAction(userCache, &activity.User, setting, courseItem, pointAction.Knowledge[index], questionAction)
			time.Sleep(5 * time.Second)
		}
	}
	// 讨论刷取
	if bbsDTOs != nil {
		for _, bbsDTO := range bbsDTOs {
			if !activity.IsRunning { //打断
				return
			}
			card, _, err2 := xuexitong.PageMobileChapterCardAction(
				userCache, key, courseId, bbsDTO.KnowledgeID, bbsDTO.CardIndex, courseItem.Cpi)

			if err2 != nil {
				if strings.Contains(err2.Error(), "章节未开放") {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "该章节未开放，可能是因为前面章节有任务点未学完导致后续任务点未开放，已自动跳过该任务点")
					return
				}
				if strings.Contains(err2.Error(), "没有历史人脸") {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.BoldRed, "过人脸失败，该账号可能从未进行过人脸识别，请先进行一次人脸识别后再试")
					os.Exit(0)
				}
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", `[`, courseItem.CourseName, `] `, lg.Red, err2.Error())
				os.Exit(0)
			}
			bbsDTO.AttachmentsDetection(card)
			if !bbsDTO.IsJob { //不是任务点或者已经是完成的任务点直接退出
				continue
			}
			activity.ExecuteBBS(userCache, setting, courseItem, pointAction.Knowledge[index], &bbsDTO)
			time.Sleep(5 * time.Second)
		}
	}

}

// 常规刷视频逻辑
func (activity *XXTActivity) ExecuteVideo2(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, p *xuexitongApi.PointVideoDto, key, courseCpi int) {

	if state, _ := xuexitong.VideoDtoFetchAction(cache, p); state {

		var playingTime = p.PlayTime
		if p.IsPassed == false && p.PlayTime == p.Duration {
			playingTime = 0
		}
		var overTime = 0
		selectSec := 58                     //默认60s
		extendSec := 5                      //过超提交停留时间
		limitTime := max(500, p.Duration/2) //过超时间最大限制
		mode := 1                           //0为Web模式，1为手机模式
		for {
			if !activity.IsRunning { //打断
				return
			}
			var playReport string
			var err error
			if playingTime != p.Duration {

				if playingTime == p.PlayTime {
					playReport, err = xuexitong.VideoSubmitStudyTimeAction(cache, p, playingTime, mode, 3)
				} else {
					playReport, err = xuexitong.VideoSubmitStudyTimeAction(cache, p, playingTime, mode, 0)
				}
			} else {
				playReport, err = xuexitong.VideoSubmitStudyTimeAction(cache, p, playingTime, mode, 0)
			}
			if err != nil {
				//若报错500并且已经过超，那么可能是视屏有问题，所以最好直接跳过进行下一个视频
				if strings.Contains(err.Error(), "failed to fetch video, status code: 500") {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.BoldRed, "提交学时接口访问异常，触发风控500，重登次数过多已自动跳到下一任务点。", "，返回信息：", playReport, err.Error())
					break
				}
				//当报错无权限的时候尝试人脸
				if strings.Contains(err.Error(), "failed to fetch video, status code: 403") { //触发403立即使用人脸检测
					if mode == 1 {
						mode = 0
						lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", lg.Yellow, "检测到手机端触发403正在切换为Web端...")
						continue
					}
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", lg.Yellow, "触发403正在尝试绕过人脸识别...")
					//上传人脸
					pullJson, img, err2 := cache.GetHistoryFaceImg("")
					if err2 != nil {
						lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", lg.BoldRed, "上传人脸失败，已自动跳过该视屏", pullJson, err2)
						return
					}
					disturbImage := utils.ImageRGBDisturb(img)
					_, _, _, _, errPass := xuexitong.PassFacePCAction(cache, p.CourseID, p.ClassID, p.Cpi, fmt.Sprintf("%d", p.KnowledgeID), p.Enc, p.JobID, p.ObjectID, p.Mid, p.RandomCaptureTime, disturbImage)
					if errPass != nil {
						lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", lg.Red, "绕过人脸失败", errPass.Error(), "请在学习通客户端上确保最近一次人脸识别是正确的，yatori会自动拉取最近一次识别的人脸数据进行")
					} else {
						lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", lg.Green, "绕过人脸成功")
					}
					time.Sleep(5 * time.Second) //不要删！！！！一定要等待一小段时间才能请求PageMobile
					continue
				}
				if strings.Contains(err.Error(), "failed to fetch video, status code: 404") { //触发404
					time.Sleep(10 * time.Second)
					continue
				}
			}
			if err != nil {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.BoldRed, "提交学时接口访问异常，返回信息：", err.Error())
				break
			}
			if gojsonq.New().JSONString(playReport).Find("isPassed") == nil {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.BoldRed, "提交学时接口访问异常，返回信息：", playReport)
				break
			}
			//阈值超限提交
			outTimeMsg := gojsonq.New().JSONString(playReport).Find("OutTimeMsg")
			if outTimeMsg != nil {
				if outTimeMsg.(string) == "观看时长超过阈值" {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", "提交状态：", lg.Green, strconv.FormatBool(gojsonq.New().JSONString(playReport).Find("isPassed").(bool)), "观看时长超过阈值，已直接提交", lg.Default, " ", "观看时间：", strconv.Itoa(p.Duration)+"/"+strconv.Itoa(p.Duration), " ", "观看进度：", fmt.Sprintf("%.2f", float32(p.Duration)/float32(p.Duration)*100), "%")
					break
				}
			}
			if gojsonq.New().JSONString(playReport).Find("isPassed").(bool) == true && playingTime >= p.Duration { //看完了，则直接退出
				if overTime == 0 {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", "提交状态：", lg.Green, strconv.FormatBool(gojsonq.New().JSONString(playReport).Find("isPassed").(bool)), lg.Default, " ", "观看时间：", strconv.Itoa(p.Duration)+"/"+strconv.Itoa(p.Duration), " ", "观看进度：", fmt.Sprintf("%.2f", float32(p.Duration)/float32(p.Duration)*100), "%")
				} else {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", "提交状态：", lg.Green, strconv.FormatBool(gojsonq.New().JSONString(playReport).Find("isPassed").(bool)), lg.Default, " ", "观看时间：", strconv.Itoa(p.Duration)+"/"+strconv.Itoa(p.Duration), " ", "过超时间：", strconv.Itoa(overTime)+"/"+strconv.Itoa(limitTime), " ", lg.Green, "过超提交成功", lg.Default, " ", "观看进度：", fmt.Sprintf("%.2f", float32(p.Duration)/float32(p.Duration)*100), "%")
				}
				break
			}

			if overTime == 0 { //正常提交
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", "提交状态：", lg.Green, lg.Green, strconv.FormatBool(gojsonq.New().JSONString(playReport).Find("isPassed").(bool)), lg.Default, " ", "观看时间：", strconv.Itoa(playingTime)+"/"+strconv.Itoa(p.Duration), " ", "观看进度：", fmt.Sprintf("%.2f", float32(playingTime)/float32(p.Duration)*100), "%")
			} else { //过超提交
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", "提交状态：", lg.Green, lg.Green, strconv.FormatBool(gojsonq.New().JSONString(playReport).Find("isPassed").(bool)), lg.Default, " ", "观看时间：", strconv.Itoa(playingTime)+"/"+strconv.Itoa(p.Duration), " ", "过超时间：", strconv.Itoa(overTime)+"/"+strconv.Itoa(limitTime), " ", "观看进度：", fmt.Sprintf("%.2f", float32(playingTime)/float32(p.Duration)*100), "%")
			}
			if overTime >= limitTime { //过超提交触发
				lg.Print(lg.INFO, lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", lg.Red, "过超提交失败，自动进行下一任务...")
				break
			}

			if p.Duration-playingTime < selectSec && p.Duration != playingTime { //时间小于58s时
				playingTime = p.Duration
				time.Sleep(time.Duration(p.Duration-playingTime) * time.Second)
			} else if p.Duration == playingTime { //记录过超提交触发条件
				//判断是否为任务点，如果为任务点那么就不累计过超提交
				if p.JobID == "" && p.Attachment == nil {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", "提交状态：", lg.Green, strconv.FormatBool(gojsonq.New().JSONString(playReport).Find("isPassed").(bool)), lg.Default, " ", "该视频为非任务点看完后直接跳入下一视频")
					break
				} else {
					overTime += extendSec
				}
				time.Sleep(time.Duration(extendSec) * time.Second)
			} else { //正常计时逻辑
				playingTime = playingTime + selectSec
				time.Sleep(time.Duration(selectSec) * time.Second)
			}
		}
	} else {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", lg.Red, "该视屏任务点解析失败，可能是任务点视屏本身问题，已自动跳过")
	}
}

// 常规刷文档逻辑
func ExecuteDocument(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, p *xuexitongApi.PointDocumentDto) {
	report, err := point.ExecuteDocument(cache, p)
	if gojsonq.New().JSONString(report).Find("status") == nil || err != nil || gojsonq.New().JSONString(report).Find("status") == false {
		if err == nil {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.BoldRed, "文档学习提交接口访问异常（可能是因为该文档不是任务点导致的），返回信息：", report)
		} else {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.BoldRed, "文档学习提交接口访问异常（可能是因为该文档不是任务点导致的），返回信息：", report, err.Error())
		}
	}

	if gojsonq.New().JSONString(report).Find("status").(bool) {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", "文档阅览状态：", lg.Green, lg.Green, strconv.FormatBool(gojsonq.New().JSONString(report).Find("status").(bool)), lg.Default, " ")
	}
}

// 常规外链任务处理
func ExecuteHyperlink(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, p *xuexitongApi.PointHyperlinkDto) {
	report, err := point.ExecuteHyperlink(cache, p)
	if gojsonq.New().JSONString(report).Find("status") == nil || err != nil || gojsonq.New().JSONString(report).Find("status") == false {
		if err == nil {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.BoldRed, "外链任务点学习提交接口访问异常，返回信息：", report)
		} else {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.BoldRed, "外链任务点学习提交接口访问异常，返回信息：", report, err.Error())
		}
	}

	if gojsonq.New().JSONString(report).Find("status").(bool) {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", "外链任务点状态：", lg.Green, lg.Green, strconv.FormatBool(gojsonq.New().JSONString(report).Find("status").(bool)), lg.Default, " ")
	}
}

// 常规直播任务处理
func ExecuteLive(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, p *xuexitongApi.PointLiveDto) {
	point.PullLiveInfoAction(cache, p)
	var passValue float64 = 90

	//如果该直播还未开播
	if p.LiveStatusCode == 0 {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", lg.Yellow, "该直播任务点还未开播，已自动跳过")
		return
	}
	relationReport, err2 := point.LiveCreateRelationAction(cache, p)
	if err2 != nil {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.BoldRed, "直播任务点建立联系接口访问异常，返回信息：", relationReport, err2.Error())
	} else {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.Green, "直播任务点建立联系成功，返回信息：", relationReport)
	}

	for {
		report, err := point.ExecuteLive(cache, p)

		point.PullLiveInfoAction(cache, p) //更新直播节点结构体进度
		if err != nil {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.BoldRed, "直播任务点学习提交接口访问异常，返回信息：", report, err.Error())
		}

		if strings.Contains(report, "@success") {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", "直播任务点状态：", lg.Green, report, lg.Default, "，直播观看进度：", lg.Green, fmt.Sprintf("%.2f", p.VideoCompletePercent), "%")
		} else {
			if err != nil {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.BoldRed, "直播任务点学习提交接口访问异常，返回信息：", report, err.Error())
			} else {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.BoldRed, "直播任务点学习提交接口访问异常，返回信息：", report)
			}
		}
		if p.VideoCompletePercent >= passValue {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】 >>> ", lg.Green, "直播任务点已完成")
			return
		}
		time.Sleep(30 * time.Second)
	}
}

// 写课程的作业和考试
func (activity *XXTActivity) WriteCourseWorkAndExam(setting config.Setting, user *config.User, userCache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse) {
	if !activity.IsRunning {
		return
	}
	if user.CoursesCustom.AutoExam == 1 || user.CoursesCustom.AutoExam == 2 || user.CoursesCustom.AutoExam == 3 {
		if user.CoursesCustom.AutoExam == 1 { //检测AI可用性
			err2 := aiq.AICheck(setting.AiSetting.AiUrl, setting.AiSetting.Model, setting.AiSetting.APIKEY, setting.AiSetting.AiType)
			if err2 != nil {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", lg.BoldRed, "<"+string(setting.AiSetting.AiType)+">", "AI不可用，错误信息："+err2.Error())
				return
			}
		} else if user.CoursesCustom.AutoExam == 2 { // 检测外挂题库可用性
			err2 := external.CheckApiQueRequest(setting.ApiQueSetting.Url, 5, nil)
			if err2 != nil {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", lg.BoldRed, "外挂题库不可用，错误信息："+err2.Error())
				return
			}
		}
		if user.CoursesCustom.CxWorkSw != nil && *user.CoursesCustom.CxWorkSw == 1 {
			//拉取作业列表
			workList, err1 := xuexitong.PullWorkListAction(userCache, *courseItem)
			if err1 != nil {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Red, "拉取作业列表失败,已自动跳过")
			} else {
				for _, work := range workList {
					if !activity.IsRunning {
						return
					}
					if !(work.Status == "待做" || work.Status == "未交" || work.Status == "待重做") {
						continue
					}
					//进入作业
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Yellow, "正在尝试进入作业...")
					time.Sleep(3 * time.Second)
					err2 := xuexitong.EnterWorkAction(userCache, &work)
					if err2 != nil {
						if strings.Contains(err2.Error(), "已过时效，不能操作!") {
							lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Red, "该作业已过时，已自动跳过该作业...")
							continue
						}
						continue
					}
					//执行作业
					activity.WorkAction(userCache, user, setting, courseItem, work)
				}
			}
		}
		if user.CoursesCustom.CxExamSw != nil && *user.CoursesCustom.CxExamSw == 1 {
			//拉取考试列表
			examList, err1 := xuexitong.PullExamListAction(userCache, *courseItem)
			if err1 != nil {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Red, "拉取考试列表失败,已自动跳过")
			} else {
				for _, exam := range examList {
					if !activity.IsRunning {
						return
					}
					if exam.Status != "待做" && exam.Status != "待重考" {
						continue
					}
					//进入考试
					err2 := xuexitong.EnterExamAction(userCache, &exam)
					if err2 != nil {
						continue
					}
					//执行考试
					activity.ExamAction(userCache, user, setting, courseItem, exam)
				}
			}
		}
	}
}

func (activity *XXTActivity) WorkAction(userCache *xuexitongApi.XueXiTUserCache, user *config.User, setting config.Setting, courseItem *xuexitong.XueXiTCourse, work xuexitong.XXTWork) {
	if !activity.IsRunning {
		return
	}
	lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Yellow, "正在写作业中...")
	//拉取题目
	for i := range work.QuestionTotal {
		if !activity.IsRunning {
			return
		}
		questionItem, err2 := work.PullWorkQuestionAction(userCache, i)
		if err2 != nil {
			continue
		}
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Yellow, fmt.Sprintf("写作业状态中,正在回答第%d题", i+1))
		//内置AI自动写题
		if user.CoursesCustom.AutoExam == 1 {
			err3 := questionItem.WriteQuestionForAIAction(userCache, setting.AiSetting.AiUrl, setting.AiSetting.Model, setting.AiSetting.AiType, setting.AiSetting.APIKEY)
			if err3 != nil {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Red, "AI回答错误:"+err3.Error())
			}
		} else if user.CoursesCustom.AutoExam == 2 {
			questionItem.WriteQuestionForExternalAction(setting.ApiQueSetting.Url)
		} else if user.CoursesCustom.AutoExam == 3 {
			err3 := questionItem.WriteQuestionForXXTAIAction(userCache, questionItem.ClassId, questionItem.CourseId, questionItem.Cpi)
			if err3 != nil {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Red, "内置AI回答错误:"+err3.Error())
			}
		}
		//提交写的题
		submitResult, err3 := questionItem.SubmitWorkAnswerAction(userCache, (user.CoursesCustom.ExamAutoSubmit == 1 || user.CoursesCustom.ExamAutoSubmit == 2) && work.QuestionTotal == i+1)
		if err3 != nil {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Red, "作业提交失败:"+err3.Error())
		}

		if gojsonq.New().JSONString(submitResult).Find("status") == false {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Red, fmt.Sprintf("第%d题回答失败,服务器返回:%s", i+1, submitResult))
		} else {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Green, fmt.Sprintf("第%d题回答成功,服务器返回:%s", i+1, submitResult))
		}
	}
	lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Green, "作业已完成")
}

func (activity *XXTActivity) ExamAction(userCache *xuexitongApi.XueXiTUserCache, user *config.User, setting config.Setting, courseItem *xuexitong.XueXiTCourse, exam xuexitong.XXTExam) {
	if !activity.IsRunning {
		return
	}
	lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Yellow, "正在考试中...")
	//拉取题目
	for i := range exam.QuestionTotal {
		if !activity.IsRunning {
			return
		}
		questionItem, err2 := exam.PullExamQuestionAction(userCache, i)
		if err2 != nil {
			continue
		}
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Yellow, fmt.Sprintf("考试状态中,正在回答第%d题,总共%d题", i+1, exam.QuestionTotal))
		//内置AI自动写题
		if user.CoursesCustom.AutoExam == 1 {
			err3 := questionItem.WriteQuestionForAIAction(userCache, setting.AiSetting.AiUrl, setting.AiSetting.Model, setting.AiSetting.AiType, setting.AiSetting.APIKEY)
			if err3 != nil {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Red, "AI回答错误:"+err3.Error())
			}
		} else if user.CoursesCustom.AutoExam == 2 {
			questionItem.WriteQuestionForExternalAction(setting.ApiQueSetting.Url)
		} else if user.CoursesCustom.AutoExam == 3 {
			err3 := questionItem.WriteQuestionForXXTAIAction(userCache, questionItem.ClassId, questionItem.CourseId, questionItem.Cpi)
			if err3 != nil {
				lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Red, "内置AI回答错误:"+err3.Error())
			}
		}
		//提交写的题
		isSubmit := false
		if (user.CoursesCustom.ExamAutoSubmit == 1 || user.CoursesCustom.ExamAutoSubmit == 2) && exam.QuestionTotal == i+1 {
			isSubmit = true //满足提交条件则提交试卷
		}
		submitResult, err3 := questionItem.SubmitExamAnswerAction(userCache, isSubmit)
		if err3 != nil {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Red, "试卷提交失败:"+err3.Error())
		}
		//处理限制提交时间的考试-----------------
		re := regexp.MustCompile(`考试(\d+)分钟内不允许提交考试`)
		matches := re.FindStringSubmatch(submitResult)
		if len(matches) > 1 {
			minSubmitTime, _ := strconv.Atoi(matches[1])
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Green, fmt.Sprintf("检测到该考试限制开考%d分钟内不允许提交考试，已自动延时%d分钟...", minSubmitTime, minSubmitTime))
			time.Sleep(time.Duration(minSubmitTime) * time.Minute)
			submitResult, err3 = questionItem.SubmitExamAnswerAction(userCache, isSubmit)
		}

		//如果考试时间已用完则直接退出
		if strings.Contains(submitResult, "考试时间已用完,不允许提交答案!") {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Red, "试卷提交失败，考试时间已用完，已自动跳过。服务器返回信息:"+submitResult)
			break
		}

		if gojsonq.New().JSONString(submitResult).Find("status") == false {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Red, fmt.Sprintf("第%d题回答失败,服务器返回:%s", i+1, submitResult))
		} else {
			lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Green, fmt.Sprintf("第%d题回答成功,服务器返回:%s", i+1, submitResult))
		}
	}
	lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Green, "考试已完成")
}

func (activity *XXTActivity) ExecuteBBS(cache *xuexitongApi.XueXiTUserCache, setting config.Setting, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, bbsDto *xuexitongApi.PointBBsDto) {
	if !activity.IsRunning {
		return
	}
	bbsTopic, err1 := point.PullPhoneBbsInfoAction(cache, bbsDto) //拉取相关数据
	if bbsTopic == nil {
		lg.Print(lg.INFO, "[学习通]", `[`, cache.Name, `] `, "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", bbsDto.Title, "】", lg.BoldRed, "无法正常拉取讨论任务点主题，已自动跳过该讨论任务点...")
		return
	}
	if err1 != nil {
		lg.Print(lg.INFO, err1.Error())
	}
	lg.Print(lg.INFO, "[学习通]", `[`, cache.Name, `] `, "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", bbsDto.Title, "】", lg.Yellow, "正在执行讨论任务点...")
	var report string
	var err error
	if activity.User.CoursesCustom.AutoExam == 1 {
		report, err = bbsTopic.AIAnswer(cache, bbsDto, setting.AiSetting.AiUrl, setting.AiSetting.Model, setting.AiSetting.AiType, setting.AiSetting.APIKEY)
	} else if activity.User.CoursesCustom.AutoExam == 2 {
		report, err = bbsTopic.ExternalAnswer(cache, bbsDto, setting.ApiQueSetting.Url)
	} else if activity.User.CoursesCustom.AutoExam == 3 {
		report, err = bbsTopic.XXTAIAnswer(cache, bbsDto)
	}
	if err != nil {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", bbsDto.Title, "】", lg.Red, "讨论提交失败:"+err.Error())
	} else {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, cache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", bbsTopic.Title, "】 >>> ", "讨论任务点状态：", lg.Green, lg.Green, gojsonq.New().JSONString(report).Find("msg").(string))
	}
}

func (activity *XXTActivity) ChapterTestAction(userCache *xuexitongApi.XueXiTUserCache, user *config.User, setting config.Setting, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, questionAction xuexitongApi.Question) {
	if !activity.IsRunning {
		return
	}
	if user.CoursesCustom.AutoExam == 1 {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", fmt.Sprintf("<%s>", setting.AiSetting.AiType), lg.Default, "【"+courseItem.CourseName+"】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.Yellow, "正在AI自动写章节作业...")
	} else if user.CoursesCustom.AutoExam == 2 {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", lg.Default, "【"+courseItem.CourseName+"】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.Yellow, "正在外挂题库自动写章节作业...")
	} else if user.CoursesCustom.AutoExam == 3 {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", lg.Default, "【"+courseItem.CourseName+"】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.Yellow, "正在内置AI自动写章节作业...")
	}
	stopStart := 1
	stopEnd := 2
	//选择题
	for i := range questionAction.Choice {
		if !activity.IsRunning {
			return
		}
		q := &questionAction.Choice[i] // 获取对应选项
		switch user.CoursesCustom.AutoExam {
		case 1:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXChoiceQue: *q,
			})
			aiSetting := setting.AiSetting //获取AI设置
			q.AnswerAIGet(userCache.UserID, aiSetting.AiUrl, aiSetting.Model, aiSetting.AiType, message, aiSetting.APIKEY)
		case 2:
			q.AnswerExternalGet(setting.ApiQueSetting.Url)
		case 3:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXChoiceQue: *q,
			})
			q.AnswerXXTAIGet(userCache, questionAction.ClassId, questionAction.CourseId, questionAction.Cpi, message)
		}
		time.Sleep(time.Duration(rand.Intn(stopEnd-stopStart)+stopStart) * time.Second) //随机暂停，避免太快
	}
	//判断题
	for i := range questionAction.Judge {
		if !activity.IsRunning {
			return
		}
		q := &questionAction.Judge[i] // 获取对应选项
		switch user.CoursesCustom.AutoExam {
		case 1:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXJudgeQue: *q,
			})
			aiSetting := setting.AiSetting //获取AI设置
			q.AnswerAIGet(userCache.UserID, aiSetting.AiUrl, aiSetting.Model, aiSetting.AiType, message, aiSetting.APIKEY)
		case 2:
			q.AnswerExternalGet(setting.ApiQueSetting.Url)
		case 3:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXJudgeQue: *q,
			})
			q.AnswerXXTAIGet(userCache, questionAction.ClassId, questionAction.CourseId, questionAction.Cpi, message)
		}
		time.Sleep(time.Duration(rand.Intn(stopEnd-stopStart)+stopStart) * time.Second) //随机暂停，避免太快
	}
	//填空题
	for i := range questionAction.Fill {
		if !activity.IsRunning {
			return
		}
		q := &questionAction.Fill[i] // 获取对应选项
		switch user.CoursesCustom.AutoExam {
		case 1:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXFillQue: *q,
			})
			aiSetting := setting.AiSetting //获取AI设置
			q.AnswerAIGet(userCache.UserID, aiSetting.AiUrl, aiSetting.Model, aiSetting.AiType, message, aiSetting.APIKEY)
		case 2:
			q.AnswerExternalGet(setting.ApiQueSetting.Url)
		case 3:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXFillQue: *q,
			})
			q.AnswerXXTAIGet(userCache, questionAction.ClassId, questionAction.CourseId, questionAction.Cpi, message)
		}
		time.Sleep(time.Duration(rand.Intn(stopEnd-stopStart)+stopStart) * time.Second) //随机暂停，避免太快
	}
	//简答题
	for i := range questionAction.Short {
		if !activity.IsRunning {
			return
		}
		q := &questionAction.Short[i] // 获取对应选项
		switch user.CoursesCustom.AutoExam {
		case 1:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXShortQue: *q,
			})
			aiSetting := setting.AiSetting //获取AI设置
			q.AnswerAIGet(userCache.UserID, aiSetting.AiUrl, aiSetting.Model, aiSetting.AiType, message, aiSetting.APIKEY)
		case 2:
			q.AnswerExternalGet(setting.ApiQueSetting.Url)
		case 3:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXShortQue: *q,
			})
			q.AnswerXXTAIGet(userCache, questionAction.ClassId, questionAction.CourseId, questionAction.Cpi, message)
		}
		time.Sleep(time.Duration(rand.Intn(stopEnd-stopStart)+stopStart) * time.Second) //随机暂停，避免太快
	}
	//名词解释
	for i := range questionAction.TermExplanation {
		if !activity.IsRunning {
			return
		}
		q := &questionAction.TermExplanation[i] // 获取对应选项
		switch user.CoursesCustom.AutoExam {
		case 1:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXTermExplanationQue: *q,
			})
			aiSetting := setting.AiSetting //获取AI设置
			q.AnswerAIGet(userCache.UserID, aiSetting.AiUrl, aiSetting.Model, aiSetting.AiType, message, aiSetting.APIKEY)
		case 2:
			q.AnswerExternalGet(setting.ApiQueSetting.Url)
		case 3:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXTermExplanationQue: *q,
			})
			q.AnswerXXTAIGet(userCache, questionAction.ClassId, questionAction.CourseId, questionAction.Cpi, message)
		}
		time.Sleep(time.Duration(rand.Intn(stopEnd-stopStart)+stopStart) * time.Second) //随机暂停，避免太快
	}
	//论述题
	for i := range questionAction.Essay {
		if !activity.IsRunning {
			return
		}
		q := &questionAction.Essay[i] // 获取对应选项
		switch user.CoursesCustom.AutoExam {
		case 1:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXEssayQue: *q,
			})
			aiSetting := setting.AiSetting //获取AI设置
			q.AnswerAIGet(userCache.UserID, aiSetting.AiUrl, aiSetting.Model, aiSetting.AiType, message, aiSetting.APIKEY)
		case 2:
			q.AnswerExternalGet(setting.ApiQueSetting.Url)
		case 3:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXEssayQue: *q,
			})
			q.AnswerXXTAIGet(userCache, questionAction.ClassId, questionAction.CourseId, questionAction.Cpi, message)
		}
		time.Sleep(time.Duration(rand.Intn(stopEnd-stopStart)+stopStart) * time.Second) //随机暂停，避免太快
	}

	//连线题
	for i := range questionAction.Matching {
		if !activity.IsRunning {
			return
		}
		q := &questionAction.Matching[i] // 获取对应选项
		switch user.CoursesCustom.AutoExam {
		case 1:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXMatchingQue: *q,
			})
			aiSetting := setting.AiSetting //获取AI设置
			q.AnswerAIGet(userCache.UserID, aiSetting.AiUrl, aiSetting.Model, aiSetting.AiType, message, aiSetting.APIKEY)
		case 2:
			q.AnswerExternalGet(setting.ApiQueSetting.Url)
		case 3:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{
				XueXMatchingQue: *q,
			})
			q.AnswerXXTAIGet(userCache, questionAction.ClassId, questionAction.CourseId, questionAction.Cpi, message)

		}
		time.Sleep(time.Duration(rand.Intn(stopEnd-stopStart)+stopStart) * time.Second) //随机暂停，避免太快
	}

	var resultStr string
	if user.CoursesCustom.ExamAutoSubmit == 0 {
		xuexitong.AnswerFixedPattern(questionAction.Choice, questionAction.Judge)
		resultStr, _ = xuexitong.WorkNewSubmitAnswerAction(userCache, questionAction, false)
	} else if user.CoursesCustom.ExamAutoSubmit == 1 {
		xuexitong.AnswerFixedPattern(questionAction.Choice, questionAction.Judge)
		resultStr, _ = xuexitong.WorkNewSubmitAnswerAction(userCache, questionAction, true)
	} else if user.CoursesCustom.ExamAutoSubmit == 2 {
		xuexitong.AnswerFixedPattern(questionAction.Choice, questionAction.Judge)
		if activity.CheckAnswerIsAvoid(questionAction.Choice, questionAction.Judge, questionAction.Fill, questionAction.Short) {
			resultStr, _ = xuexitong.WorkNewSubmitAnswerAction(userCache, questionAction, false) //留空了，只保存
			//如果提交失败那么直接输出AI答题的文本
			if gojsonq.New().JSONString(resultStr).Find("status") == false {
				if user.CoursesCustom.AutoExam == 1 {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", fmt.Sprintf("<%s>", setting.AiSetting.AiType), "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.BoldRed, "AI答题保存失败,返回信息："+resultStr, " AI答题信息：", questionAction.String())
				} else if user.CoursesCustom.AutoExam == 2 {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.BoldRed, "外挂题库答题保存失败,返回信息："+resultStr, " 外挂题库答题信息：", questionAction.String())
				} else if user.CoursesCustom.AutoExam == 3 {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", fmt.Sprintf("<%s>", "学习通内置AI"), "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.BoldRed, "AI答题保存失败,返回信息："+resultStr, " AI答题信息：", questionAction.String())
				}
			}
		} else {
			resultStr, _ = xuexitong.WorkNewSubmitAnswerAction(userCache, questionAction, true) //没有留空则提交
			if gojsonq.New().JSONString(resultStr).Find("status") == false {
				if user.CoursesCustom.AutoExam == 1 {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", fmt.Sprintf("<%s>", setting.AiSetting.AiType), "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.BoldRed, "AI答题保存失败,返回信息："+resultStr, " AI答题信息：", questionAction.String())
				} else if user.CoursesCustom.AutoExam == 2 {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.BoldRed, "外挂题库答题保存失败,返回信息："+resultStr, " 外挂题库答题信息：", questionAction.String())
				} else if user.CoursesCustom.AutoExam == 3 {
					lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", fmt.Sprintf("<%s>", "学习通内置AI"), "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.BoldRed, "AI答题保存失败,返回信息："+resultStr, " AI答题信息：", questionAction.String())
				}
			}
		}
	}
	if user.CoursesCustom.AutoExam == 1 {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", fmt.Sprintf("<%s>", setting.AiSetting.AiType), "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.Green, "章节作业AI答题完毕,服务器返回信息：", resultStr)
	} else if user.CoursesCustom.AutoExam == 2 {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.Green, "章节作业外挂题库答题完毕,服务器返回信息：", resultStr)
	} else if user.CoursesCustom.AutoExam == 3 {
		lg.Print(lg.INFO, "[学习通]", "[", lg.Green, userCache.Name, lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.Green, "章节作业内置AI答题完毕,服务器返回信息：", resultStr)
	}
}

func (activity *XXTActivity) CheckAnswerIsAvoid(choices []xuexitongApi.ChoiceQue, judges []xuexitongApi.JudgeQue, fills []xuexitongApi.FillQue, shorts []xuexitongApi.ShortQue) bool {
	for _, choice := range choices {
		resStatus := true
		if choice.Answers != nil {
			candidateSelects := []string{} //待选
			for _, option := range choice.Options {
				candidateSelects = append(candidateSelects, option)
			}
			for _, answer := range choice.Answers {
				var sortArray []qutils.Co = qutils.SimilarityArrayAndSort(answer, candidateSelects)
				if sortArray[0].Score >= 0.9 {
					resStatus = false
				}
			}
			if resStatus {
				return true
			}
		} else {
			return true
		}
	}
	for _, judge := range judges {
		resStatus := true
		if judge.Answers != nil {
			for _, answer := range judge.Answers {
				for _, option := range judge.Options {
					if answer == option || answer == "错误" || answer == "正确" {
						resStatus = false
					}
				}
			}
			if resStatus {
				return true
			}
		} else {
			return true
		}
	}
	for _, fill := range fills {
		if fill.OpFromAnswer == nil || len(fill.OpFromAnswer) <= 0 {
			return true
		}
	}
	for _, short := range shorts {
		if short.OpFromAnswer == nil || len(short.OpFromAnswer) <= 0 {
			return true
		}
	}
	return false
}
