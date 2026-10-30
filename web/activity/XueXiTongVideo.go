package activity

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"yatori-go-console/config"

	"github.com/thedevsaddam/gojsonq"
	"github.com/yatori-dev/yatori-go-core/aggregation/xuexitong"
	xuexitongApi "github.com/yatori-dev/yatori-go-core/api/xuexitong"
	"github.com/yatori-dev/yatori-go-core/utils"
	lg "github.com/yatori-dev/yatori-go-core/utils/log"
)

// executeVideo 视频任务点（含 403 人脸绕过、过超提交）
func (activity *XXTActivity) executeVideo(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, p *xuexitongApi.PointVideoDto, key, courseId int) {
	for {
		if !activity.IsRunning {
			return
		}
		card, enc, err := xuexitong.PageMobileChapterCardAction(cache, key, courseId, p.KnowledgeID, p.CardIndex, courseItem.Cpi)
		if err != nil {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", lg.Red, "拉取任务点卡片失败，已跳过：", err.Error())
			return
		}
		p.AttachmentsDetection(card)
		p.Enc = enc
		if !p.IsJob {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "[", courseItem.CourseName, "] ", "[", knowledgeItem.Name, "] ", lg.Blue, "该视频非任务点或已完成，已自动跳过")
			return
		}
		state, _ := xuexitong.VideoDtoFetchAction(cache, p)
		if !state {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", p.Title, "】", lg.Red, "该视频任务点解析失败，已自动跳过")
			return
		}
		if err := activity.submitVideoTime(cache, courseItem, knowledgeItem, p); err != nil {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", p.Title, "】", lg.BoldRed, err.Error())
			return
		}
		if p.PlayTime >= p.Duration || p.IsPassed {
			return
		}
	}
}

// submitVideoTime 按学习通提交学时协议循环提交，直到看完
func (activity *XXTActivity) submitVideoTime(cache *xuexitongApi.XueXiTUserCache, courseItem *xuexitong.XueXiTCourse, knowledgeItem xuexitong.KnowledgeItem, p *xuexitongApi.PointVideoDto) error {
	playingTime := p.PlayTime
	overTime := 0
	selectSec := 58
	extendSec := 5
	limitTime := max(500, p.Duration/2)
	mode := 1 //0为Web模式，1为手机模式

	for {
		if !activity.IsRunning {
			return nil
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
			if strings.Contains(err.Error(), "failed to fetch video, status code: 500") {
				return fmt.Errorf("提交学时接口访问异常，触发风控500，已跳到下一任务点：%v", err)
			}
			if strings.Contains(err.Error(), "failed to fetch video, status code: 403") {
				if mode == 1 {
					mode = 0
					lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", lg.Yellow, "检测到手机端触发403正在切换为Web端...")
					continue
				}
				if err2 := bypassFace(cache, p); err2 != nil {
					return fmt.Errorf("触发403且绕过人脸失败，已跳过该视频：%v", err2)
				}
				time.Sleep(5 * time.Second)
				continue
			}
			if strings.Contains(err.Error(), "failed to fetch video, status code: 404") {
				time.Sleep(10 * time.Second)
				continue
			}
			return fmt.Errorf("提交学时接口访问异常：%v", err)
		}
		isPassed := gojsonq.New().JSONString(playReport).Find("isPassed")
		if isPassed == nil {
			return fmt.Errorf("提交学时接口访问异常，返回信息：%s", playReport)
		}
		if outTimeMsg := gojsonq.New().JSONString(playReport).Find("OutTimeMsg"); outTimeMsg != nil {
			if msg, ok := outTimeMsg.(string); ok && msg == "观看时长超过阈值" {
				lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.Green, "观看时长超过阈值，已直接提交")
				return nil
			}
		}
		if isPassed.(bool) && playingTime >= p.Duration {
			lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", "【", knowledgeItem.Label, " ", knowledgeItem.Name, "】", "【", p.Title, "】", lg.Green, "观看完成，进度100%")
			return nil
		}
		if overTime >= limitTime {
			return fmt.Errorf("过超提交失败，自动进行下一任务")
		}
		if p.Duration-playingTime < selectSec && p.Duration != playingTime {
			playingTime = p.Duration
		} else if p.Duration == playingTime {
			if p.JobID == "" && p.Attachment == nil {
				lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", "【", courseItem.CourseName, "】", lg.Blue, "该视频为非任务点，看完后直接跳入下一视频")
				return nil
			}
			overTime += extendSec
		} else {
			playingTime += selectSec
		}
		time.Sleep(time.Duration(selectSec) * time.Second)
	}
}

// bypassFace 上传历史人脸绕过 403
func bypassFace(cache *xuexitongApi.XueXiTUserCache, p *xuexitongApi.PointVideoDto) error {
	_, img, err := cache.GetHistoryFaceImg("")
	if err != nil {
		return err
	}
	disturbImage := utils.ImageRGBDisturb(img)
	if _, _, _, _, err := xuexitong.PassFacePCAction(cache, p.CourseID, p.ClassID, p.Cpi, strconv.Itoa(p.KnowledgeID), p.Enc, p.JobID, p.ObjectID, p.Mid, p.RandomCaptureTime, disturbImage); err != nil {
		return err
	}
	lg.Print(lg.INFO, "[", lg.Green, config.DisplayAccount(cache.Name), lg.Default, "] ", lg.Green, "绕过人脸成功")
	return nil
}
