package projectplanner

import (
	"testing"

	"github.com/yeisme/taskbridge/internal/project"
)

func TestDetectGoalType(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want project.GoalType
	}{
		{name: "learning chinese", in: "我希望学习 openclaw", want: project.GoalTypeLearning},
		{name: "travel chinese", in: "我希望去上海旅游", want: project.GoalTypeTravel},
		{name: "generic", in: "整理一下家务", want: project.GoalTypeGeneric},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectGoalType(tt.in); got != tt.want {
				t.Fatalf("DetectGoalType(%q)=%q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
