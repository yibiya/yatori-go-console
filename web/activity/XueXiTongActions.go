package activity

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"yatori-go-console/config"

	"github.com/thedevsaddam/gojsonq"
	"github.com/yatori-dev/yatori-go-core/aggregation/xuexitong"
	"github.com/yatori-dev/yatori-go-core/aggregation/xuexitong/point"
	xuexitongApi "github.com/yatori-dev/yatori-go-core/api/xuexitong"
	"github.com/yatori-dev/yatori-go-core/que-core/aiq"
	"github.com/yatori-dev/yatori-go-core/que-core/external"
	lg "github.com/yatori-dev/yatori-go-core/utils/log"
)

// prefetchCard 拉取任务点卡片并回填附件信息（Jtoken / IsJob 等）。
// 与 logic/xuexitong 的 CLI 实现保持一致：不先做这一步，提交接口会因为缺少 jtoken 返回"无效的请求参数"，
// IsJob 也永远是 false，任务点会被误当成已完成而整节点跳过。
func (activity *XXTActivity) prefetchCard(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeID, cardIndex, key, courseId int) (interface{}, error) {
	card, _, err := xuexitong.PageMobileChapterCardAction(cache, key, courseId, knowledgeID, cardIndex, courseItem.Cpi)
	return card, err
}

// executeDocument 文档任务点
func (activity *XXTActivity) executeDocument(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, p *xuexitongApi.PointDocumentDto, key, courseId int) {
	card, err := activity.prefetchCard(cache, courseItem, p.KnowledgeID, p.CardIndex, key, courseId)
	if err != nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Red, "拉取任务点卡片失败，已跳过：", err.Error())
		return
	}
	p.AttachmentsDetection(card)
	if !p.IsJob {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Blue, "该文档非任务点或已完成，已自动跳过")
		return
	}
	report, err := point.ExecuteDocument(cache, p)
	if err != nil || gojsonq.New().JSONString(report).Find("status") != true {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.BoldRed, "文档学习提交接口访问异常，返回信息：", report, errString(err))
		return
	}
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Green, "文档学习完毕")
}

// executeHyperlink 外链任务点
func (activity *XXTActivity) executeHyperlink(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, p *xuexitongApi.PointHyperlinkDto, key, courseId int) {
	card, err := activity.prefetchCard(cache, courseItem, p.KnowledgeID, p.CardIndex, key, courseId)
	if err != nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Red, "拉取任务点卡片失败，已跳过：", err.Error())
		return
	}
	p.AttachmentsDetection(card)
	report, err := point.ExecuteHyperlink(cache, p)
	if err != nil || gojsonq.New().JSONString(report).Find("status") != true {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.BoldRed, "外链任务点学习提交接口访问异常，返回信息：", report, errString(err))
		return
	}
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Green, "外链任务点学习完毕")
}

// executeLive 直播任务点
func (activity *XXTActivity) executeLive(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, p *xuexitongApi.PointLiveDto, key, courseId int) {
	card, err := activity.prefetchCard(cache, courseItem, p.KnowledgeID, p.CardIndex, key, courseId)
	if err != nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Red, "拉取任务点卡片失败，已跳过：", err.Error())
		return
	}
	p.AttachmentsDetection(card)
	if !p.IsJob {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Blue, "该直播非任务点或已完成，已自动跳过")
		return
	}
	point.PullLiveInfoAction(cache, p)
	if p.LiveStatusCode == 0 {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", lg.Yellow, "该直播任务点还未开播，已自动跳过")
		return
	}
	report, err := point.LiveCreateRelationAction(cache, p)
	if err != nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.BoldRed, "直播任务点建立联系接口访问异常，返回信息：", report, errString(err))
		return
	}
	for {
		if !activity.IsRunning {
			return
		}
		report, err := point.ExecuteLive(cache, p)
		point.PullLiveInfoAction(cache, p)
		if err != nil {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.BoldRed, "直播任务点学习提交接口访问异常，返回信息：", report, errString(err))
		}
		if p.VideoCompletePercent >= 90 {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Green, "直播任务点已完成")
			return
		}
		time.Sleep(30 * time.Second)
	}
}

// executeBBS 讨论任务点
func (activity *XXTActivity) executeBBS(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, p *xuexitongApi.PointBBsDto, setting config.Setting, key, courseId int) {
	bbsCard, cardErr := activity.prefetchCard(cache, courseItem, p.KnowledgeID, p.CardIndex, key, courseId)
	if cardErr != nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Red, "拉取任务点卡片失败，已跳过：", cardErr.Error())
		return
	}
	p.AttachmentsDetection(bbsCard)
	if !p.IsJob {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Blue, "该讨论非任务点或已完成，已自动跳过")
		return
	}
	bbsTopic, err1 := point.PullPhoneBbsInfoAction(cache, p)
	if bbsTopic == nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", lg.BoldRed, "无法拉取讨论任务点主题，已自动跳过")
		return
	}
	if err1 != nil {
		lg.Print(lg.INFO, err1.Error())
	}
	var report string
	var err error
	switch activity.User.CoursesCustom.AutoExam {
	case 1:
		report, err = bbsTopic.AIAnswer(cache, p, setting.AiSetting.AiUrl, setting.AiSetting.Model, setting.AiSetting.AiType, setting.AiSetting.APIKEY)
	case 2:
		report, err = bbsTopic.ExternalAnswer(cache, p, setting.ApiQueSetting.Url)
	case 3:
		report, err = bbsTopic.XXTAIAnswer(cache, p)
	}
	if err != nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", lg.BoldRed, "讨论任务点学习提交接口访问异常，返回信息：", report, errString(err))
		return
	}
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Green, "讨论任务点学习完毕")
}

// writeCourseWorkAndExam 课程的作业与考试
func (activity *XXTActivity) writeCourseWorkAndExam(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse) {
	user := &activity.User
	if user.CoursesCustom.AutoExam != 1 && user.CoursesCustom.AutoExam != 2 && user.CoursesCustom.AutoExam != 3 {
		return
	}
	setting := readSetting()
	if err := ensureAnswerBackend(setting, user.CoursesCustom.AutoExam); err != nil {
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.BoldRed, err.Error())
		return
	}
	if user.CoursesCustom.CxWorkSw != nil && *user.CoursesCustom.CxWorkSw == 1 {
		workList, err1 := xuexitong.PullWorkListAction(cache, *courseItem)
		if err1 != nil {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Red, "拉取作业列表失败,已自动跳过")
		} else {
			for i := range workList {
				if !activity.IsRunning {
					return
				}
				work := workList[i]
				if !(work.Status == "待做" || work.Status == "未交" || work.Status == "待重做") {
					continue
				}
				if err2 := xuexitong.EnterWorkAction(cache, &work); err2 != nil {
					continue
				}
				activity.workAction(cache, courseItem, work, setting)
			}
		}
	}
	if user.CoursesCustom.CxExamSw != nil && *user.CoursesCustom.CxExamSw == 1 {
		examList, err1 := xuexitong.PullExamListAction(cache, *courseItem)
		if err1 != nil {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Red, "拉取考试列表失败,已自动跳过")
		} else {
			for i := range examList {
				if !activity.IsRunning {
					return
				}
				exam := examList[i]
				if exam.Status != "待做" && exam.Status != "待重考" {
					continue
				}
				if err2 := xuexitong.EnterExamAction(cache, &exam); err2 != nil {
					continue
				}
				activity.examAction(cache, courseItem, exam, setting)
			}
		}
	}
}

// workAction 作业处理逻辑
func (activity *XXTActivity) workAction(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, work xuexitong.XXTWork, setting config.Setting) {
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Yellow, "正在写作业中...")
	for i := range work.QuestionTotal {
		if !activity.IsRunning {
			return
		}
		question, err2 := work.PullWorkQuestionAction(cache, i)
		if err2 != nil {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", lg.Red, "拉取作业题目失败：", err2.Error())
			continue
		}
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Yellow, "写作业状态中,正在回答第"+strconv.Itoa(i+1)+"题")
		switch activity.User.CoursesCustom.AutoExam {
		case 1:
			if err3 := question.WriteQuestionForAIAction(cache, setting.AiSetting.AiUrl, setting.AiSetting.Model, setting.AiSetting.AiType, setting.AiSetting.APIKEY); err3 != nil {
				lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.Red, "AI回答错误:", err3.Error())
			}
		case 2:
			question.WriteQuestionForExternalAction(setting.ApiQueSetting.Url)
		case 3:
			if err3 := question.WriteQuestionForXXTAIAction(cache, question.ClassId, question.CourseId, question.Cpi); err3 != nil {
				lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.Red, "内置AI回答错误:", err3.Error())
			}
		}
		isSubmit := (activity.User.CoursesCustom.ExamAutoSubmit == 1 || activity.User.CoursesCustom.ExamAutoSubmit == 2) && work.QuestionTotal == i+1
		submitResult, err3 := question.SubmitWorkAnswerAction(cache, isSubmit)
		if err3 != nil {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.Red, "作业提交失败:", err3.Error())
			continue
		}
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Green, "第"+strconv.Itoa(i+1)+"题回答成功,服务器返回:", submitResult)
	}
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", work.Name, "】", lg.Green, "作业已完成")
}

// examAction 考试处理逻辑
func (activity *XXTActivity) examAction(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, exam xuexitong.XXTExam, setting config.Setting) {
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Yellow, "正在考试中...")
	for i := range exam.QuestionTotal {
		if !activity.IsRunning {
			return
		}
		question, err2 := exam.PullExamQuestionAction(cache, i)
		if err2 != nil {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.Red, "拉取考试题目失败：", err2.Error())
			continue
		}
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Yellow, "考试状态中,正在回答第"+strconv.Itoa(i+1)+"题,总共"+strconv.Itoa(exam.QuestionTotal)+"题")
		switch activity.User.CoursesCustom.AutoExam {
		case 1:
			if err3 := question.WriteQuestionForAIAction(cache, setting.AiSetting.AiUrl, setting.AiSetting.Model, setting.AiSetting.AiType, setting.AiSetting.APIKEY); err3 != nil {
				lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.Red, "AI回答错误:", err3.Error())
			}
		case 2:
			question.WriteQuestionForExternalAction(setting.ApiQueSetting.Url)
		case 3:
			if err3 := question.WriteQuestionForXXTAIAction(cache, question.ClassId, question.CourseId, question.Cpi); err3 != nil {
				lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.Red, "内置AI回答错误:", err3.Error())
			}
		}
		isSubmit := (activity.User.CoursesCustom.ExamAutoSubmit == 1 || activity.User.CoursesCustom.ExamAutoSubmit == 2) && exam.QuestionTotal == i+1
		submitResult, err3 := question.SubmitExamAnswerAction(cache, isSubmit)
		if err3 != nil {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.Red, "试卷提交失败:", err3.Error())
			continue
		}
		if strings.Contains(submitResult, "考试时间已用完,不允许提交答案!") {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.Red, "试卷提交失败，考试时间已用完，已自动跳过。服务器返回信息:", submitResult)
			break
		}
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Green, "第"+strconv.Itoa(i+1)+"题回答成功,服务器返回:", submitResult)
	}
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", exam.Name, "】", lg.Green, "考试已完成")
}

// chapterTestAction 章测处理逻辑
func (activity *XXTActivity) chapterTestAction(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, questionAction xuexitongApi.Question, setting config.Setting) {
	answerTitle := map[int]string{1: "AI", 2: "外挂题库", 3: "内置AI"}[activity.User.CoursesCustom.AutoExam]
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", questionAction.Title, "】", lg.Yellow, "正在"+answerTitle+"自动写章节作业...")
	for i := range questionAction.Choice {
		q := &questionAction.Choice[i]
		switch activity.User.CoursesCustom.AutoExam {
		case 1:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXChoiceQue: *q})
			q.AnswerAIGet(cache.UserID, setting.AiSetting.AiUrl, setting.AiSetting.Model, setting.AiSetting.AiType, message, setting.AiSetting.APIKEY)
		case 2:
			q.AnswerExternalGet(setting.ApiQueSetting.Url)
		case 3:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXChoiceQue: *q})
			q.AnswerXXTAIGet(cache, questionAction.ClassId, questionAction.CourseId, questionAction.Cpi, message)
		}
	}
	for i := range questionAction.Judge {
		q := &questionAction.Judge[i]
		switch activity.User.CoursesCustom.AutoExam {
		case 1:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXJudgeQue: *q})
			q.AnswerAIGet(cache.UserID, setting.AiSetting.AiUrl, setting.AiSetting.Model, setting.AiSetting.AiType, message, setting.AiSetting.APIKEY)
		case 2:
			q.AnswerExternalGet(setting.ApiQueSetting.Url)
		case 3:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXJudgeQue: *q})
			q.AnswerXXTAIGet(cache, questionAction.ClassId, questionAction.CourseId, questionAction.Cpi, message)
		}
	}
	for i := range questionAction.Fill {
		q := &questionAction.Fill[i]
		switch activity.User.CoursesCustom.AutoExam {
		case 1:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXFillQue: *q})
			q.AnswerAIGet(cache.UserID, setting.AiSetting.AiUrl, setting.AiSetting.Model, setting.AiSetting.AiType, message, setting.AiSetting.APIKEY)
		case 2:
			q.AnswerExternalGet(setting.ApiQueSetting.Url)
		case 3:
			message := xuexitong.AIProblemMessage(questionAction.Title, q.Type.String(), xuexitongApi.ExamTurn{XueXFillQue: *q})
			q.AnswerXXTAIGet(cache, questionAction.ClassId, questionAction.CourseId, questionAction.Cpi, message)
		}
	}
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", lg.Green, "章节作业已完成")
}

// errString 统一错误转字符串（nil 返回空串）
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ensureAnswerBackend 答题后端可用性预检，失败时跳过该课程的题类任务
func ensureAnswerBackend(setting config.Setting, autoExam int) error {
	switch autoExam {
	case 1:
		if err := aiq.AICheck(setting.AiSetting.AiUrl, setting.AiSetting.Model, setting.AiSetting.APIKEY, setting.AiSetting.AiType); err != nil {
			return fmt.Errorf("<%s> AI不可用，已跳过答题任务：%v", setting.AiSetting.AiType, err)
		}
	case 2:
		if err := external.CheckApiQueRequest(setting.ApiQueSetting.Url, 5, nil); err != nil {
			return fmt.Errorf("外挂题库不可用，已跳过答题任务：%v", err)
		}
	}
	return nil
}

// executeResources 处理资源型条目（资料下载 / 知识点球图）。
//
// 这类条目在章节卡片里没有 jobid，学习通没有 /ananas/job/xxx 提交接口，
// 只能通过 /knowledge/save-attachment-downloader 记录一次访问
// （前端点下载时调的就是它，见 ananas/videojs-ext/recordDownloadInfo.js）。
// 失败不中断整章，只记日志。
func (activity *XXTActivity) executeResources(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, resources []xuexitongApi.PointResourceDto) {
	for i := range resources {
		if !activity.IsRunning {
			return
		}
		r := &resources[i]
		if !point.HasReportableContent(r) {
			continue
		}
		report, err := point.ExecuteResource(cache, r)
		if err != nil {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Red, "资料下载记录异常：", err.Error())
			continue
		}
		lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Name, "】", lg.Green, "资料下载已记录：", r.Title, " ", report)
		time.Sleep(2 * time.Second)
	}
}
