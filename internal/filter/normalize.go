package filter

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yeisme/taskbridge/internal/model"
)

// NormalizeListName 规范化清单名称：
// - 去首尾空白
// - 移除 emoji/符号
// - 保留中英文、数字、空格
// - 压缩连续空白
// - ASCII 小写化
func NormalizeListName(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	spacePending := false

	for _, r := range strings.TrimSpace(s) {
		switch {
		case unicode.IsSpace(r):
			spacePending = true
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if spacePending && b.Len() > 0 {
				b.WriteByte(' ')
			}
			spacePending = false
			b.WriteRune(unicode.ToLower(r))
		default:
			// 忽略符号/emoji/标点
		}
	}

	return strings.TrimSpace(b.String())
}

// MatchListNameExactNormalized 使用规范化后的精确匹配。
func MatchListNameExactNormalized(input, stored string) bool {
	n1 := NormalizeListName(input)
	n2 := NormalizeListName(stored)
	return n1 != "" && n1 == n2
}

// MatchQueryText 关键字文本匹配（AND 语义）：
// query 按空白切词，每个词都必须在任务聚合文本中出现。
func MatchQueryText(task *model.Task, query string) bool {
	if task == nil {
		return false
	}
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return true
	}

	var fields []string
	fields = append(fields, task.ID, task.Title, task.Description, task.ListID, task.ListName)
	fields = append(fields, string(task.Source), string(task.Status))
	fields = append(fields, task.Tags...)
	fields = append(fields, task.Categories...)

	haystack := strings.ToLower(strings.Join(fields, " "))
	for _, token := range strings.Fields(query) {
		if token == "" {
			continue
		}
		if !strings.Contains(haystack, token) {
			return false
		}
	}
	return true
}

// VisibleRuneCount 仅用于测试/调试，返回字符串中的可见 rune 数。
func VisibleRuneCount(s string) int {
	count := 0
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size == 1 {
			s = s[size:]
			continue
		}
		if !unicode.IsControl(r) {
			count++
		}
		s = s[size:]
	}
	return count
}
