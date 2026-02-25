package filter

import (
	"testing"

	"github.com/yeisme/taskbridge/internal/model"
)

func TestNormalizeListName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove emoji and keep chinese",
			input:    "🏗 学习与成长",
			expected: "学习与成长",
		},
		{
			name:     "trim and collapse spaces",
			input:    "  学习   与  成长  ",
			expected: "学习 与 成长",
		},
		{
			name:     "ascii lower-case",
			input:    "  My List  ",
			expected: "my list",
		},
		{
			name:     "drop symbols",
			input:    "📌 项目#1 - 规划",
			expected: "项目1 规划",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeListName(tt.input)
			if got != tt.expected {
				t.Fatalf("NormalizeListName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMatchListNameExactNormalized(t *testing.T) {
	if !MatchListNameExactNormalized("学习与成长", "🏗 学习与成长") {
		t.Fatal("expected normalized list names to match")
	}
	if MatchListNameExactNormalized("学习与成长", "项目与工作") {
		t.Fatal("expected different list names not to match")
	}
}

func TestMatchQueryText(t *testing.T) {
	task := &model.Task{
		ID:          "t1",
		Title:       "学习 Kubernetes",
		Description: "阅读官方文档",
		ListName:    "🏗 学习与成长",
		Source:      model.SourceMicrosoft,
		Status:      model.StatusTodo,
		Tags:        []string{"k8s", "cloud"},
	}

	if !MatchQueryText(task, "kubernetes 学习与成长") {
		t.Fatal("expected query tokens to match task fields")
	}
	if MatchQueryText(task, "kubernetes completed") {
		t.Fatal("expected unmatched token to fail")
	}
}
