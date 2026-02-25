package model

// Priority 优先级
type Priority int

const (
	// PriorityNone 无优先级
	PriorityNone Priority = 0
	// PriorityLow 低优先级
	PriorityLow Priority = 1
	// PriorityMedium 中优先级
	PriorityMedium Priority = 2
	// PriorityHigh 高优先级
	PriorityHigh Priority = 3
	// PriorityUrgent 紧急优先级
	PriorityUrgent Priority = 4
)

// String 返回优先级的字符串描述
func (p Priority) String() string {
	switch p {
	case PriorityNone:
		return "无"
	case PriorityLow:
		return "低"
	case PriorityMedium:
		return "中"
	case PriorityHigh:
		return "高"
	case PriorityUrgent:
		return "紧急"
	default:
		return "未知"
	}
}

// Emoji 返回优先级的 Emoji 表示
func (p Priority) Emoji() string {
	switch p {
	case PriorityNone:
		return "⚪"
	case PriorityLow:
		return "🔵"
	case PriorityMedium:
		return "🟡"
	case PriorityHigh:
		return "🟠"
	case PriorityUrgent:
		return "🔴"
	default:
		return "⚪"
	}
}

// PriorityFromInt 从整数创建优先级
func PriorityFromInt(i int) Priority {
	switch {
	case i <= 0:
		return PriorityNone
	case i == 1:
		return PriorityLow
	case i == 2:
		return PriorityMedium
	case i == 3:
		return PriorityHigh
	case i >= 4:
		return PriorityUrgent
	default:
		return PriorityNone
	}
}

// PriorityCalculator 优先级计算器 - AI 可调用
type PriorityCalculator struct {
	// WeightDueDate 截止日期权重
	WeightDueDate float64 `json:"weight_due_date"`
	// WeightImportance 重要程度权重
	WeightImportance float64 `json:"weight_importance"`
	// WeightUrgency 紧急程度权重
	WeightUrgency float64 `json:"weight_urgency"`
	// WeightProgress 进度权重
	WeightProgress float64 `json:"weight_progress"`
}

// NewDefaultPriorityCalculator 创建默认优先级计算器
func NewDefaultPriorityCalculator() *PriorityCalculator {
	return &PriorityCalculator{
		WeightDueDate:    0.4,
		WeightImportance: 0.3,
		WeightUrgency:    0.2,
		WeightProgress:   0.1,
	}
}

// Calculate 计算综合优先级分数
func (pc *PriorityCalculator) Calculate(task *Task) int {
	score := 0.0

	// 截止日期权重
	if task.DueDate != nil {
		days := task.DaysUntilDue()
		var dueScore float64
		switch {
		case days < 0:
			dueScore = 100 // 已过期
		case days == 0:
			dueScore = 90 // 今天截止
		case days <= 1:
			dueScore = 80 // 明天截止
		case days <= 3:
			dueScore = 60 // 3天内
		case days <= 7:
			dueScore = 40 // 一周内
		case days <= 14:
			dueScore = 20 // 两周内
		default:
			dueScore = 10
		}
		score += dueScore * pc.WeightDueDate
	}

	// 重要程度权重
	importanceScore := float64(task.Importance) * 25
	score += importanceScore * pc.WeightImportance

	// 紧急程度权重
	urgencyScore := float64(task.Urgency) * 25
	score += urgencyScore * pc.WeightUrgency

	// 进度权重（已完成越多，优先级相对降低）
	if task.Progress > 0 {
		progressScore := float64(100 - task.Progress)
		score += progressScore * pc.WeightProgress
	}

	return int(score)
}
