package provider

import "strings"

// ProviderDefinition 定义 Provider 的完整信息
type ProviderDefinition struct {
	Name        string   // 标准名称（小写）
	ShortName   string   // 简写
	DisplayName string   // 显示名称
	Description string   // 描述
	Aliases     []string // 额外别名（包括大小写变体）
}

// providerDefinitions 所有支持的 Provider 定义
var providerDefinitions = map[string]ProviderDefinition{
	"google": {
		Name:        "google",
		ShortName:   "google",
		DisplayName: "Google Tasks",
		Description: "Google 任务管理服务",
		Aliases:     []string{"google", "g"},
	},
	"microsoft": {
		Name:        "microsoft",
		ShortName:   "ms",
		DisplayName: "Microsoft To Do",
		Description: "微软任务管理服务",
		Aliases:     []string{"microsoft", "ms"},
	},
	"feishu": {
		Name:        "feishu",
		ShortName:   "feishu",
		DisplayName: "飞书任务",
		Description: "飞书任务管理",
		Aliases:     []string{"feishu"},
	},
	"ticktick": {
		Name:        "ticktick",
		ShortName:   "tick",
		DisplayName: "TickTick",
		Description: "TickTick 任务管理",
		Aliases:     []string{"ticktick", "tick"},
	},
	"todoist": {
		Name:        "todoist",
		ShortName:   "todo",
		DisplayName: "Todoist",
		Description: "Todoist 任务管理",
		Aliases:     []string{"todoist", "todo"},
	},
}

// aliasToName 别名到标准名称的映射（自动生成）
var aliasToName map[string]string

func init() {
	aliasToName = make(map[string]string)
	for _, def := range providerDefinitions {
		// 添加标准名称
		aliasToName[strings.ToLower(def.Name)] = def.Name
		// 添加所有别名
		for _, alias := range def.Aliases {
			aliasToName[strings.ToLower(alias)] = def.Name
		}
	}
}

// ResolveProviderName 将任意形式的 Provider 名称解析为标准名称
// 支持简写、全称、大小写不敏感
func ResolveProviderName(name string) string {
	resolved, ok := aliasToName[strings.ToLower(name)]
	if ok {
		return resolved
	}
	return name // 返回原始名称，让调用方处理未知 Provider
}

// GetProviderDefinition 获取 Provider 定义
func GetProviderDefinition(name string) (ProviderDefinition, bool) {
	standardName := ResolveProviderName(name)
	def, ok := providerDefinitions[standardName]
	return def, ok
}

// GetAllProviders 获取所有 Provider 定义（按固定顺序）
func GetAllProviders() []ProviderDefinition {
	result := make([]ProviderDefinition, 0, len(providerDefinitions))
	order := []string{"google", "microsoft", "feishu", "ticktick", "todoist"}
	for _, name := range order {
		if def, ok := providerDefinitions[name]; ok {
			result = append(result, def)
		}
	}
	return result
}

// IsValidProvider 检查是否是有效的 Provider 名称
func IsValidProvider(name string) bool {
	standardName := ResolveProviderName(name)
	_, ok := providerDefinitions[standardName]
	return ok
}
