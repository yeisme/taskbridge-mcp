package model

// Quadrant 四象限 - 艾森豪威尔矩阵
type Quadrant int

const (
	// QuadrantUrgentImportant 第一象限 - 紧急且重要 - 立即做
	QuadrantUrgentImportant Quadrant = 1
	// QuadrantNotUrgentImportant 第二象限 - 不紧急但重要 - 计划做
	QuadrantNotUrgentImportant Quadrant = 2
	// QuadrantUrgentNotImportant 第三象限 - 紧急但不重要 - 授权做
	QuadrantUrgentNotImportant Quadrant = 3
	// QuadrantNotUrgentNotImportant 第四象限 - 不紧急也不重要 - 删除/延后
	QuadrantNotUrgentNotImportant Quadrant = 4
)

// String 返回象限的字符串描述
func (q Quadrant) String() string {
	switch q {
	case QuadrantUrgentImportant:
		return "紧急且重要"
	case QuadrantNotUrgentImportant:
		return "重要不紧急"
	case QuadrantUrgentNotImportant:
		return "紧急不重要"
	case QuadrantNotUrgentNotImportant:
		return "不紧急不重要"
	default:
		return "未分类"
	}
}

// ShortName 返回象限的简短名称
func (q Quadrant) ShortName() string {
	switch q {
	case QuadrantUrgentImportant:
		return "Q1-DO"
	case QuadrantNotUrgentImportant:
		return "Q2-PLAN"
	case QuadrantUrgentNotImportant:
		return "Q3-DELEGATE"
	case QuadrantNotUrgentNotImportant:
		return "Q4-DELETE"
	default:
		return "Q0-NONE"
	}
}

// UrgencyLevel 紧急程度
type UrgencyLevel int

const (
	// UrgencyNone 无紧急程度
	UrgencyNone UrgencyLevel = 0
	// UrgencyLow 低紧急程度
	UrgencyLow UrgencyLevel = 1
	// UrgencyMedium 中紧急程度
	UrgencyMedium UrgencyLevel = 2
	// UrgencyHigh 高紧急程度
	UrgencyHigh UrgencyLevel = 3
	// UrgencyCritical 紧急
	UrgencyCritical UrgencyLevel = 4
)

// String 返回紧急程度的字符串描述
func (u UrgencyLevel) String() string {
	switch u {
	case UrgencyNone:
		return "无"
	case UrgencyLow:
		return "低"
	case UrgencyMedium:
		return "中"
	case UrgencyHigh:
		return "高"
	case UrgencyCritical:
		return "紧急"
	default:
		return "未知"
	}
}

// ImportanceLevel 重要程度
type ImportanceLevel int

const (
	// ImportanceNone 无重要程度
	ImportanceNone ImportanceLevel = 0
	// ImportanceLow 低重要程度
	ImportanceLow ImportanceLevel = 1
	// ImportanceMedium 中重要程度
	ImportanceMedium ImportanceLevel = 2
	// ImportanceHigh 高重要程度
	ImportanceHigh ImportanceLevel = 3
	// ImportanceCritical 关键重要程度
	ImportanceCritical ImportanceLevel = 4
)

// String 返回重要程度的字符串描述
func (i ImportanceLevel) String() string {
	switch i {
	case ImportanceNone:
		return "无"
	case ImportanceLow:
		return "低"
	case ImportanceMedium:
		return "中"
	case ImportanceHigh:
		return "高"
	case ImportanceCritical:
		return "关键"
	default:
		return "未知"
	}
}

// CalculateQuadrant 根据紧急和重要程度计算象限
func CalculateQuadrant(urgency UrgencyLevel, importance ImportanceLevel) Quadrant {
	isUrgent := urgency >= UrgencyMedium
	isImportant := importance >= ImportanceMedium

	switch {
	case isUrgent && isImportant:
		return QuadrantUrgentImportant
	case !isUrgent && isImportant:
		return QuadrantNotUrgentImportant
	case isUrgent && !isImportant:
		return QuadrantUrgentNotImportant
	default:
		return QuadrantNotUrgentNotImportant
	}
}

// CalculateQuadrantFromTask 根据任务属性自动计算象限
func CalculateQuadrantFromTask(task *Task) Quadrant {
	urgency := task.Urgency
	importance := task.Importance

	// 如果没有设置紧急/重要程度，尝试从其他属性推断
	if urgency == UrgencyNone && task.DueDate != nil {
		days := task.DaysUntilDue()
		switch {
		case days < 0:
			urgency = UrgencyCritical
		case days == 0:
			urgency = UrgencyHigh
		case days <= 3:
			urgency = UrgencyMedium
		case days <= 7:
			urgency = UrgencyLow
		}
	}

	if importance == ImportanceNone && task.Priority != PriorityNone {
		switch task.Priority {
		case PriorityUrgent:
			importance = ImportanceCritical
		case PriorityHigh:
			importance = ImportanceHigh
		case PriorityMedium:
			importance = ImportanceMedium
		case PriorityLow:
			importance = ImportanceLow
		}
	}

	return CalculateQuadrant(urgency, importance)
}
