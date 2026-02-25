package projectplanner

import (
	"strings"

	"github.com/yeisme/taskbridge/internal/project"
)

var learningKeywords = []string{"学习", "了解", "熟悉", "精通", "掌握", "learn", "study", "master"}
var travelKeywords = []string{"去", "旅游", "出行", "行程", "攻略", "travel", "trip", "itinerary"}

// DetectGoalType 识别目标类型。
func DetectGoalType(goal string) project.GoalType {
	input := strings.TrimSpace(strings.ToLower(goal))
	for _, keyword := range learningKeywords {
		if strings.Contains(input, keyword) {
			return project.GoalTypeLearning
		}
	}
	for _, keyword := range travelKeywords {
		if strings.Contains(input, keyword) {
			return project.GoalTypeTravel
		}
	}
	return project.GoalTypeGeneric
}
