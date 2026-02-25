package projectplanner

import (
	"fmt"
	"strings"

	"github.com/yeisme/taskbridge/internal/project"
)

type phaseTemplate struct {
	Name  string
	Tasks []project.PlanTask
}

func templatesFor(goalType project.GoalType, subject string) []phaseTemplate {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = "该目标"
	}

	switch goalType {
	case project.GoalTypeLearning:
		return learningTemplates(subject)
	case project.GoalTypeTravel:
		return travelTemplates(subject)
	default:
		return genericTemplates(subject)
	}
}

func learningTemplates(subject string) []phaseTemplate {
	return []phaseTemplate{
		{
			Name: "目标澄清",
			Tasks: []project.PlanTask{
				{Title: fmt.Sprintf("明确%s学习目标与完成标准", subject), Description: "定义范围、结果形式、验收标准", EstimateMinutes: 45, DueOffsetDays: 1, Priority: 3, Quadrant: 2, Tags: []string{"learning", "plan"}},
				{Title: fmt.Sprintf("拆解%s核心知识点", subject), Description: "列出必须掌握的 5-8 个主题", EstimateMinutes: 60, DueOffsetDays: 2, Priority: 3, Quadrant: 2, Tags: []string{"learning", "scope"}},
			},
		},
		{
			Name: "资源准备",
			Tasks: []project.PlanTask{
				{Title: fmt.Sprintf("收集%s学习资料并排序", subject), Description: "至少 3 类资料：文档/视频/实践项目", EstimateMinutes: 90, DueOffsetDays: 3, Priority: 2, Quadrant: 2, Tags: []string{"learning", "resource"}},
				{Title: fmt.Sprintf("搭建%s练习环境", subject), Description: "确保可以立即动手验证", EstimateMinutes: 120, DueOffsetDays: 4, Priority: 3, Quadrant: 2, Tags: []string{"learning", "setup"}},
			},
		},
		{
			Name: "基础实践",
			Tasks: []project.PlanTask{
				{Title: fmt.Sprintf("完成%s基础练习 1", subject), Description: "从最小可运行示例开始", EstimateMinutes: 120, DueOffsetDays: 6, Priority: 3, Quadrant: 2, Tags: []string{"learning", "practice"}},
				{Title: fmt.Sprintf("完成%s基础练习 2", subject), Description: "在练习 1 基础上增加一个变化", EstimateMinutes: 150, DueOffsetDays: 8, Priority: 3, Quadrant: 2, Tags: []string{"learning", "practice"}},
				{Title: fmt.Sprintf("整理%s常见问题清单", subject), Description: "记录障碍、原因和解决方案", EstimateMinutes: 45, DueOffsetDays: 9, Priority: 2, Quadrant: 2, Tags: []string{"learning", "notes"}},
			},
		},
		{
			Name: "阶段复盘",
			Tasks: []project.PlanTask{
				{Title: fmt.Sprintf("输出%s学习总结", subject), Description: "总结方法、关键知识、下一步", EstimateMinutes: 60, DueOffsetDays: 12, Priority: 2, Quadrant: 2, Tags: []string{"learning", "review"}},
				{Title: fmt.Sprintf("制定%s下一阶段计划", subject), Description: "定义进阶主题与时间安排", EstimateMinutes: 45, DueOffsetDays: 14, Priority: 2, Quadrant: 2, Tags: []string{"learning", "next"}},
			},
		},
	}
}

func travelTemplates(subject string) []phaseTemplate {
	return []phaseTemplate{
		{
			Name: "约束确认",
			Tasks: []project.PlanTask{
				{Title: fmt.Sprintf("确认%s出行时间和预算范围", subject), Description: "明确总预算、可用天数、同行人约束", EstimateMinutes: 45, DueOffsetDays: 1, Priority: 3, Quadrant: 2, Tags: []string{"travel", "plan"}},
				{Title: fmt.Sprintf("确定%s行程优先事项", subject), Description: "景点/美食/休息节奏排序", EstimateMinutes: 45, DueOffsetDays: 2, Priority: 3, Quadrant: 2, Tags: []string{"travel", "priority"}},
			},
		},
		{
			Name: "行程草案",
			Tasks: []project.PlanTask{
				{Title: fmt.Sprintf("整理%s候选地点清单", subject), Description: "按地理位置与兴趣分组", EstimateMinutes: 90, DueOffsetDays: 4, Priority: 2, Quadrant: 2, Tags: []string{"travel", "itinerary"}},
				{Title: fmt.Sprintf("制定%s每日路线草案", subject), Description: "控制通勤时长与停留时间", EstimateMinutes: 120, DueOffsetDays: 6, Priority: 3, Quadrant: 2, Tags: []string{"travel", "route"}},
			},
		},
		{
			Name: "预算与预订",
			Tasks: []project.PlanTask{
				{Title: fmt.Sprintf("比价并预订%s往返交通", subject), Description: "确认退改政策", EstimateMinutes: 120, DueOffsetDays: 8, Priority: 4, Quadrant: 1, Tags: []string{"travel", "booking"}},
				{Title: fmt.Sprintf("预订%s住宿", subject), Description: "优先交通便利区域", EstimateMinutes: 120, DueOffsetDays: 9, Priority: 4, Quadrant: 1, Tags: []string{"travel", "booking"}},
				{Title: fmt.Sprintf("更新%s预算明细", subject), Description: "核对住宿、交通、门票、餐饮", EstimateMinutes: 60, DueOffsetDays: 10, Priority: 3, Quadrant: 2, Tags: []string{"travel", "budget"}},
			},
		},
		{
			Name: "出发清单",
			Tasks: []project.PlanTask{
				{Title: fmt.Sprintf("准备%s证件与支付工具", subject), Description: "核对身份证件、支付方式与应急联系人", EstimateMinutes: 45, DueOffsetDays: 12, Priority: 4, Quadrant: 1, Tags: []string{"travel", "checklist"}},
				{Title: fmt.Sprintf("整理%s出发前打包清单", subject), Description: "衣物、药品、电子设备", EstimateMinutes: 60, DueOffsetDays: 13, Priority: 3, Quadrant: 2, Tags: []string{"travel", "checklist"}},
			},
		},
	}
}

func genericTemplates(subject string) []phaseTemplate {
	return []phaseTemplate{
		{
			Name: "目标定义",
			Tasks: []project.PlanTask{
				{Title: fmt.Sprintf("定义%s完成标准", subject), Description: "明确可交付结果与截止时间", EstimateMinutes: 45, DueOffsetDays: 1, Priority: 3, Quadrant: 2, Tags: []string{"generic", "plan"}},
				{Title: fmt.Sprintf("列出%s关键约束", subject), Description: "时间、资源、依赖与风险", EstimateMinutes: 45, DueOffsetDays: 2, Priority: 3, Quadrant: 2, Tags: []string{"generic", "scope"}},
			},
		},
		{
			Name: "任务拆分",
			Tasks: []project.PlanTask{
				{Title: fmt.Sprintf("拆分%s执行步骤", subject), Description: "输出 5-8 个可执行子任务", EstimateMinutes: 90, DueOffsetDays: 4, Priority: 3, Quadrant: 2, Tags: []string{"generic", "breakdown"}},
				{Title: fmt.Sprintf("确定%s优先级与顺序", subject), Description: "识别关键路径任务", EstimateMinutes: 60, DueOffsetDays: 6, Priority: 3, Quadrant: 2, Tags: []string{"generic", "priority"}},
			},
		},
		{
			Name: "执行检查点",
			Tasks: []project.PlanTask{
				{Title: fmt.Sprintf("完成%s第一轮执行", subject), Description: "优先推进高优先任务", EstimateMinutes: 180, DueOffsetDays: 9, Priority: 4, Quadrant: 1, Tags: []string{"generic", "execute"}},
				{Title: fmt.Sprintf("复盘%s当前进展", subject), Description: "更新阻塞项与下一步", EstimateMinutes: 45, DueOffsetDays: 12, Priority: 2, Quadrant: 2, Tags: []string{"generic", "review"}},
			},
		},
	}
}
