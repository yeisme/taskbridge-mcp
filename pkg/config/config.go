// Package config 提供配置管理功能
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Sync      SyncConfig      `mapstructure:"sync"`
	MCP       MCPConfig       `mapstructure:"mcp"`
	Providers ProvidersConfig `mapstructure:"providers"`
	Templates TemplatesConfig `mapstructure:"templates"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name     string `mapstructure:"name"`
	Version  string `mapstructure:"version"`
	LogLevel string `mapstructure:"log_level"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type string `mapstructure:"type"` // file, mongodb
	Path string `mapstructure:"path"`

	File  FileStorageConfig  `mapstructure:"file"`
	NoSQL NoSQLStorageConfig `mapstructure:"nosql"`
}

// FileStorageConfig 文件存储配置
type FileStorageConfig struct {
	Format   string `mapstructure:"format"`   // json, markdown
	Template string `mapstructure:"template"` // 自定义模板路径
}

// NoSQLStorageConfig NoSQL 存储配置
type NoSQLStorageConfig struct {
	// URL 数据库连接 URL
	// 格式示例：
	// - MongoDB: mongodb://localhost:27017/taskbridge
	URL string `mapstructure:"url"`

	// Database 数据库名称（用于 MongoDB 等）
	Database string `mapstructure:"database"`

	// Collection 集合名称（用于 MongoDB 等）
	Collection string `mapstructure:"collection"`
}

// SyncConfig 同步配置
type SyncConfig struct {
	Mode               string        `mapstructure:"mode"`                // once, interval, realtime
	Interval           time.Duration `mapstructure:"interval"`            // 同步间隔
	ConflictResolution string        `mapstructure:"conflict_resolution"` // local_wins, remote_wins, newer_wins, manual
	RetryCount         int           `mapstructure:"retry_count"`
	RetryDelay         time.Duration `mapstructure:"retry_delay"`
}

// MCPConfig MCP 服务配置
type MCPConfig struct {
	Enabled       bool                 `mapstructure:"enabled"`
	Transport     string               `mapstructure:"transport"` // stdio, sse, streamable; tcp 兼容映射到 sse
	Port          int                  `mapstructure:"port"`      // HTTP 模式端口
	Security      SecurityConfig       `mapstructure:"security"`
	Tools         ToolGovernanceConfig `mapstructure:"tools"`
	Observability ObservabilityConfig  `mapstructure:"observability"`
	Reliability   ReliabilityConfig    `mapstructure:"reliability"`
	Cache         CacheConfig          `mapstructure:"cache"`
	Tenant        TenantConfig         `mapstructure:"tenant"`
	Intelligence  IntelligenceConfig   `mapstructure:"intelligence"`
}

// SecurityConfig MCP 安全配置
type SecurityConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	AuthMode        string   `mapstructure:"auth_mode"`
	Tokens          []string `mapstructure:"tokens"`
	AllowedOrigins  []string `mapstructure:"allowed_origins"`
	IPAllowlist     []string `mapstructure:"ip_allowlist"`
	AuditMaskFields []string `mapstructure:"audit_mask_fields"`
}

// ToolGovernanceConfig MCP 工具治理配置
type ToolGovernanceConfig struct {
	Enabled             bool                `mapstructure:"enabled"`
	DefaultEnabled      bool                `mapstructure:"default_enabled"`
	AllowList           []string            `mapstructure:"allow_list"`
	DenyList            []string            `mapstructure:"deny_list"`
	ExperimentalEnabled bool                `mapstructure:"experimental_enabled"`
	Groups              map[string][]string `mapstructure:"groups"`
}

// ObservabilityConfig MCP 可观测性配置
type ObservabilityConfig struct {
	Metrics MetricsConfig `mapstructure:"metrics"`
	Audit   AuditConfig   `mapstructure:"audit"`
	Trace   TraceConfig   `mapstructure:"trace"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

// AuditConfig 审计配置
type AuditConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Output   string `mapstructure:"output"`
	FilePath string `mapstructure:"file_path"`
}

// TraceConfig Trace 配置
type TraceConfig struct {
	Enabled    bool    `mapstructure:"enabled"`
	SampleRate float64 `mapstructure:"sample_rate"`
}

// ReliabilityConfig 可靠性配置
type ReliabilityConfig struct {
	DefaultTimeout time.Duration        `mapstructure:"default_timeout"`
	MaxTimeout     time.Duration        `mapstructure:"max_timeout"`
	Retry          RetryConfig          `mapstructure:"retry"`
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
}

// RetryConfig 重试配置
type RetryConfig struct {
	Enabled     bool          `mapstructure:"enabled"`
	MaxAttempts int           `mapstructure:"max_attempts"`
	Backoff     time.Duration `mapstructure:"backoff"`
}

// CircuitBreakerConfig 熔断配置
type CircuitBreakerConfig struct {
	Enabled          bool          `mapstructure:"enabled"`
	FailureThreshold int           `mapstructure:"failure_threshold"`
	HalfOpenAfter    time.Duration `mapstructure:"half_open_after"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	Backend        string        `mapstructure:"backend"`
	DefaultTTL     time.Duration `mapstructure:"default_ttl"`
	MaxEntries     int           `mapstructure:"max_entries"`
	CacheableTools []string      `mapstructure:"cacheable_tools"`
}

// TenantConfig 租户配置
type TenantConfig struct {
	Enabled       bool                         `mapstructure:"enabled"`
	DefaultTenant string                       `mapstructure:"default_tenant"`
	HeaderKey     string                       `mapstructure:"header_key"`
	Quotas        map[string]TenantQuotaConfig `mapstructure:"quotas"`
}

// TenantQuotaConfig 租户配额
type TenantQuotaConfig struct {
	QPS         int `mapstructure:"qps"`
	DailyCalls  int `mapstructure:"daily_calls"`
	MaxParallel int `mapstructure:"max_parallel"`
}

// IntelligenceConfig MCP 智能治理配置
type IntelligenceConfig struct {
	Enabled     bool              `mapstructure:"enabled"`
	Timezone    string            `mapstructure:"timezone"`
	Overdue     OverdueConfig     `mapstructure:"overdue"`
	LongTerm    LongTermConfig    `mapstructure:"long_term"`
	Decompose   DecomposeConfig   `mapstructure:"decomposition"`
	Achievement AchievementConfig `mapstructure:"achievement"`
}

// OverdueConfig 逾期治理配置
type OverdueConfig struct {
	WarningThreshold  int  `mapstructure:"warning_threshold"`
	OverloadThreshold int  `mapstructure:"overload_threshold"`
	SevereDays        int  `mapstructure:"severe_days"`
	AskBeforeDelete   bool `mapstructure:"ask_before_delete"`
	MaxCandidates     int  `mapstructure:"max_candidates"`
}

// LongTermConfig 长期任务调配配置
type LongTermConfig struct {
	MinAgeDays               int    `mapstructure:"min_age_days"`
	ShortTermWindowDays      int    `mapstructure:"short_term_window_days"`
	ShortTermMin             int    `mapstructure:"short_term_min"`
	ShortTermMax             int    `mapstructure:"short_term_max"`
	PromoteCountWhenShortage int    `mapstructure:"promote_count_when_shortage"`
	RetainCountWhenOverflow  int    `mapstructure:"retain_count_when_overflow"`
	OverflowStrategy         string `mapstructure:"overflow_strategy"`
}

// DecomposeConfig 复杂任务拆分配置
type DecomposeConfig struct {
	ComplexityThreshold    int      `mapstructure:"complexity_threshold"`
	DetectAbstractKeywords bool     `mapstructure:"detect_abstract_keywords"`
	AskBeforeSplit         bool     `mapstructure:"ask_before_split"`
	PreferredStrategy      string   `mapstructure:"preferred_strategy"`
	AbstractKeywords       []string `mapstructure:"abstract_keywords"`
}

// AchievementConfig 成就分析配置
type AchievementConfig struct {
	SnapshotGranularity   string `mapstructure:"snapshot_granularity"`
	StreakGoalPerDay      int    `mapstructure:"streak_goal_per_day"`
	BadgeEnabled          bool   `mapstructure:"badge_enabled"`
	NarrativeEnabled      bool   `mapstructure:"narrative_enabled"`
	ComparePreviousPeriod bool   `mapstructure:"compare_previous_period"`
}

// ProvidersConfig Provider 配置
type ProvidersConfig struct {
	Microsoft ProviderConfig `mapstructure:"microsoft"`
	Google    ProviderConfig `mapstructure:"google"`
	Feishu    ProviderConfig `mapstructure:"feishu"`
	TickTick  ProviderConfig `mapstructure:"ticktick"`
	Dida      ProviderConfig `mapstructure:"dida"`
	Todoist   ProviderConfig `mapstructure:"todoist"`
	OmniFocus ProviderConfig `mapstructure:"omnifocus"`
	Apple     ProviderConfig `mapstructure:"apple"`
}

// ProviderConfig 单个 Provider 的配置
type ProviderConfig struct {
	Enabled         bool                   `mapstructure:"enabled"`
	ClientID        string                 `mapstructure:"clientid"`
	ClientSecret    string                 `mapstructure:"clientsecret"`
	TenantID        string                 `mapstructure:"tenantid"`
	AppID           string                 `mapstructure:"appid"`
	AppSecret       string                 `mapstructure:"appsecret"`
	APIKey          string                 `mapstructure:"apikey"`
	APIToken        string                 `mapstructure:"apitoken"`
	DatabaseID      string                 `mapstructure:"databaseid"`
	Username        string                 `mapstructure:"username"`
	Password        string                 `mapstructure:"password"`
	CredentialsFile string                 `mapstructure:"credentialsfile"`
	Transport       string                 `mapstructure:"transport"`
	ListNames       []string               `mapstructure:"listnames"`
	Extra           map[string]interface{} `mapstructure:",remain"`
}

// TemplatesConfig 模板配置
type TemplatesConfig struct {
	JSON     TemplateConfig `mapstructure:"json"`
	Markdown TemplateConfig `mapstructure:"markdown"`
}

// TemplateConfig 单个模板的配置
type TemplateConfig struct {
	Path string `mapstructure:"path"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		App: AppConfig{
			Name:     "taskbridge",
			Version:  "1.0.1",
			LogLevel: "info",
		},
		Storage: StorageConfig{
			Type: "file",
			Path: "./data",
			File: FileStorageConfig{
				Format:   "json",
				Template: "",
			},
			NoSQL: NoSQLStorageConfig{
				URL:        "mongodb://localhost:27017/taskbridge",
				Database:   "taskbridge",
				Collection: "tasks",
			},
		},
		Sync: SyncConfig{
			Mode:               "interval",
			Interval:           5 * time.Minute,
			ConflictResolution: "newer_wins",
			RetryCount:         3,
			RetryDelay:         1 * time.Second,
		},
		MCP: MCPConfig{
			Enabled:   true,
			Transport: "stdio",
			Port:      14940,
			Security: SecurityConfig{
				Enabled:  false,
				AuthMode: "none",
				AuditMaskFields: []string{
					"apikey", "api_token", "apitoken", "clientsecret", "appsecret", "password", "token", "access_token", "refresh_token",
				},
			},
			Tools: ToolGovernanceConfig{
				Enabled:        false,
				DefaultEnabled: true,
				Groups:         map[string][]string{},
			},
			Observability: ObservabilityConfig{
				Metrics: MetricsConfig{
					Enabled: false,
					Path:    "/metrics",
				},
				Audit: AuditConfig{
					Enabled: false,
					Output:  "stdout",
				},
				Trace: TraceConfig{
					Enabled:    false,
					SampleRate: 0.1,
				},
			},
			Reliability: ReliabilityConfig{
				DefaultTimeout: 30 * time.Second,
				MaxTimeout:     2 * time.Minute,
				Retry: RetryConfig{
					Enabled:     false,
					MaxAttempts: 3,
					Backoff:     500 * time.Millisecond,
				},
				CircuitBreaker: CircuitBreakerConfig{
					Enabled:          false,
					FailureThreshold: 5,
					HalfOpenAfter:    30 * time.Second,
				},
			},
			Cache: CacheConfig{
				Enabled:    false,
				Backend:    "memory",
				DefaultTTL: 30 * time.Second,
				MaxEntries: 1000,
			},
			Tenant: TenantConfig{
				Enabled:       false,
				DefaultTenant: "default",
				HeaderKey:     "X-TaskBridge-Tenant",
				Quotas:        map[string]TenantQuotaConfig{},
			},
			Intelligence: IntelligenceConfig{
				Enabled:  true,
				Timezone: "Asia/Shanghai",
				Overdue: OverdueConfig{
					WarningThreshold:  3,
					OverloadThreshold: 10,
					SevereDays:        7,
					AskBeforeDelete:   true,
					MaxCandidates:     30,
				},
				LongTerm: LongTermConfig{
					MinAgeDays:               7,
					ShortTermWindowDays:      7,
					ShortTermMin:             5,
					ShortTermMax:             10,
					PromoteCountWhenShortage: 3,
					RetainCountWhenOverflow:  1,
					OverflowStrategy:         "defer",
				},
				Decompose: DecomposeConfig{
					ComplexityThreshold:    60,
					DetectAbstractKeywords: true,
					AskBeforeSplit:         true,
					PreferredStrategy:      "project_split",
					AbstractKeywords: []string{
						"优化", "推进", "完善", "研究", "整理",
					},
				},
				Achievement: AchievementConfig{
					SnapshotGranularity:   "daily",
					StreakGoalPerDay:      1,
					BadgeEnabled:          true,
					NarrativeEnabled:      true,
					ComparePreviousPeriod: true,
				},
			},
		},
		Providers: ProvidersConfig{
			Microsoft: ProviderConfig{Enabled: false},
			Google:    ProviderConfig{Enabled: false},
			Feishu:    ProviderConfig{Enabled: false},
			TickTick:  ProviderConfig{Enabled: false},
			Dida:      ProviderConfig{Enabled: false},
			Todoist:   ProviderConfig{Enabled: false},
			OmniFocus: ProviderConfig{Enabled: false},
			Apple:     ProviderConfig{Enabled: false},
		},
		Templates: TemplatesConfig{
			JSON:     TemplateConfig{Path: "./templates/json/default.json"},
			Markdown: TemplateConfig{Path: "./templates/markdown/default.md"},
		},
	}
}

// Load 加载配置
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 设置默认值
	defaultCfg := DefaultConfig()
	setDefaults(v, defaultCfg)

	// 设置配置文件
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// 查找配置文件
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		// 添加 HOME 目录支持
		if homeDir, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(homeDir, ".taskbridge"))
		}
		v.AddConfigPath("/etc/taskbridge")
	}

	// 读取环境变量
	v.SetEnvPrefix("TASKBRIDGE")
	v.AutomaticEnv()

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// 配置文件不存在，使用默认值
	}

	// 解析配置
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults(v *viper.Viper, cfg *Config) {
	v.SetDefault("app.name", cfg.App.Name)
	v.SetDefault("app.version", cfg.App.Version)
	v.SetDefault("app.log_level", cfg.App.LogLevel)

	v.SetDefault("storage.type", cfg.Storage.Type)
	v.SetDefault("storage.path", cfg.Storage.Path)
	v.SetDefault("storage.file.format", cfg.Storage.File.Format)
	v.SetDefault("storage.nosql.url", cfg.Storage.NoSQL.URL)
	v.SetDefault("storage.nosql.database", cfg.Storage.NoSQL.Database)
	v.SetDefault("storage.nosql.collection", cfg.Storage.NoSQL.Collection)

	v.SetDefault("sync.mode", cfg.Sync.Mode)
	v.SetDefault("sync.interval", cfg.Sync.Interval)
	v.SetDefault("sync.conflict_resolution", cfg.Sync.ConflictResolution)
	v.SetDefault("sync.retry_count", cfg.Sync.RetryCount)
	v.SetDefault("sync.retry_delay", cfg.Sync.RetryDelay)

	v.SetDefault("mcp.enabled", cfg.MCP.Enabled)
	v.SetDefault("mcp.transport", cfg.MCP.Transport)
	v.SetDefault("mcp.port", cfg.MCP.Port)
	v.SetDefault("mcp.security.enabled", cfg.MCP.Security.Enabled)
	v.SetDefault("mcp.security.auth_mode", cfg.MCP.Security.AuthMode)
	v.SetDefault("mcp.security.tokens", cfg.MCP.Security.Tokens)
	v.SetDefault("mcp.security.allowed_origins", cfg.MCP.Security.AllowedOrigins)
	v.SetDefault("mcp.security.ip_allowlist", cfg.MCP.Security.IPAllowlist)
	v.SetDefault("mcp.security.audit_mask_fields", cfg.MCP.Security.AuditMaskFields)
	v.SetDefault("mcp.tools.enabled", cfg.MCP.Tools.Enabled)
	v.SetDefault("mcp.tools.default_enabled", cfg.MCP.Tools.DefaultEnabled)
	v.SetDefault("mcp.tools.allow_list", cfg.MCP.Tools.AllowList)
	v.SetDefault("mcp.tools.deny_list", cfg.MCP.Tools.DenyList)
	v.SetDefault("mcp.tools.experimental_enabled", cfg.MCP.Tools.ExperimentalEnabled)
	v.SetDefault("mcp.tools.groups", cfg.MCP.Tools.Groups)
	v.SetDefault("mcp.observability.metrics.enabled", cfg.MCP.Observability.Metrics.Enabled)
	v.SetDefault("mcp.observability.metrics.path", cfg.MCP.Observability.Metrics.Path)
	v.SetDefault("mcp.observability.audit.enabled", cfg.MCP.Observability.Audit.Enabled)
	v.SetDefault("mcp.observability.audit.output", cfg.MCP.Observability.Audit.Output)
	v.SetDefault("mcp.observability.audit.file_path", cfg.MCP.Observability.Audit.FilePath)
	v.SetDefault("mcp.observability.trace.enabled", cfg.MCP.Observability.Trace.Enabled)
	v.SetDefault("mcp.observability.trace.sample_rate", cfg.MCP.Observability.Trace.SampleRate)
	v.SetDefault("mcp.reliability.default_timeout", cfg.MCP.Reliability.DefaultTimeout)
	v.SetDefault("mcp.reliability.max_timeout", cfg.MCP.Reliability.MaxTimeout)
	v.SetDefault("mcp.reliability.retry.enabled", cfg.MCP.Reliability.Retry.Enabled)
	v.SetDefault("mcp.reliability.retry.max_attempts", cfg.MCP.Reliability.Retry.MaxAttempts)
	v.SetDefault("mcp.reliability.retry.backoff", cfg.MCP.Reliability.Retry.Backoff)
	v.SetDefault("mcp.reliability.circuit_breaker.enabled", cfg.MCP.Reliability.CircuitBreaker.Enabled)
	v.SetDefault("mcp.reliability.circuit_breaker.failure_threshold", cfg.MCP.Reliability.CircuitBreaker.FailureThreshold)
	v.SetDefault("mcp.reliability.circuit_breaker.half_open_after", cfg.MCP.Reliability.CircuitBreaker.HalfOpenAfter)
	v.SetDefault("mcp.cache.enabled", cfg.MCP.Cache.Enabled)
	v.SetDefault("mcp.cache.backend", cfg.MCP.Cache.Backend)
	v.SetDefault("mcp.cache.default_ttl", cfg.MCP.Cache.DefaultTTL)
	v.SetDefault("mcp.cache.max_entries", cfg.MCP.Cache.MaxEntries)
	v.SetDefault("mcp.cache.cacheable_tools", cfg.MCP.Cache.CacheableTools)
	v.SetDefault("mcp.tenant.enabled", cfg.MCP.Tenant.Enabled)
	v.SetDefault("mcp.tenant.default_tenant", cfg.MCP.Tenant.DefaultTenant)
	v.SetDefault("mcp.tenant.header_key", cfg.MCP.Tenant.HeaderKey)
	v.SetDefault("mcp.tenant.quotas", cfg.MCP.Tenant.Quotas)
	v.SetDefault("mcp.intelligence.enabled", cfg.MCP.Intelligence.Enabled)
	v.SetDefault("mcp.intelligence.timezone", cfg.MCP.Intelligence.Timezone)
	v.SetDefault("mcp.intelligence.overdue.warning_threshold", cfg.MCP.Intelligence.Overdue.WarningThreshold)
	v.SetDefault("mcp.intelligence.overdue.overload_threshold", cfg.MCP.Intelligence.Overdue.OverloadThreshold)
	v.SetDefault("mcp.intelligence.overdue.severe_days", cfg.MCP.Intelligence.Overdue.SevereDays)
	v.SetDefault("mcp.intelligence.overdue.ask_before_delete", cfg.MCP.Intelligence.Overdue.AskBeforeDelete)
	v.SetDefault("mcp.intelligence.overdue.max_candidates", cfg.MCP.Intelligence.Overdue.MaxCandidates)
	v.SetDefault("mcp.intelligence.long_term.min_age_days", cfg.MCP.Intelligence.LongTerm.MinAgeDays)
	v.SetDefault("mcp.intelligence.long_term.short_term_window_days", cfg.MCP.Intelligence.LongTerm.ShortTermWindowDays)
	v.SetDefault("mcp.intelligence.long_term.short_term_min", cfg.MCP.Intelligence.LongTerm.ShortTermMin)
	v.SetDefault("mcp.intelligence.long_term.short_term_max", cfg.MCP.Intelligence.LongTerm.ShortTermMax)
	v.SetDefault("mcp.intelligence.long_term.promote_count_when_shortage", cfg.MCP.Intelligence.LongTerm.PromoteCountWhenShortage)
	v.SetDefault("mcp.intelligence.long_term.retain_count_when_overflow", cfg.MCP.Intelligence.LongTerm.RetainCountWhenOverflow)
	v.SetDefault("mcp.intelligence.long_term.overflow_strategy", cfg.MCP.Intelligence.LongTerm.OverflowStrategy)
	v.SetDefault("mcp.intelligence.decomposition.complexity_threshold", cfg.MCP.Intelligence.Decompose.ComplexityThreshold)
	v.SetDefault("mcp.intelligence.decomposition.detect_abstract_keywords", cfg.MCP.Intelligence.Decompose.DetectAbstractKeywords)
	v.SetDefault("mcp.intelligence.decomposition.ask_before_split", cfg.MCP.Intelligence.Decompose.AskBeforeSplit)
	v.SetDefault("mcp.intelligence.decomposition.preferred_strategy", cfg.MCP.Intelligence.Decompose.PreferredStrategy)
	v.SetDefault("mcp.intelligence.decomposition.abstract_keywords", cfg.MCP.Intelligence.Decompose.AbstractKeywords)
	v.SetDefault("mcp.intelligence.achievement.snapshot_granularity", cfg.MCP.Intelligence.Achievement.SnapshotGranularity)
	v.SetDefault("mcp.intelligence.achievement.streak_goal_per_day", cfg.MCP.Intelligence.Achievement.StreakGoalPerDay)
	v.SetDefault("mcp.intelligence.achievement.badge_enabled", cfg.MCP.Intelligence.Achievement.BadgeEnabled)
	v.SetDefault("mcp.intelligence.achievement.narrative_enabled", cfg.MCP.Intelligence.Achievement.NarrativeEnabled)
	v.SetDefault("mcp.intelligence.achievement.compare_previous_period", cfg.MCP.Intelligence.Achievement.ComparePreviousPeriod)
}

// Save 保存配置到文件
func Save(cfg *Config, path string) error {
	v := viper.New()
	v.Set("app", cfg.App)
	v.Set("storage", cfg.Storage)
	v.Set("sync", cfg.Sync)
	v.Set("mcp", cfg.MCP)
	v.Set("providers", cfg.Providers)
	v.Set("templates", cfg.Templates)

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	return v.WriteConfigAs(path)
}

// GetConfigPath 获取配置文件路径
// 优先级:
// 1. 环境变量 TASKBRIDGE_CONFIG
// 2. HOME 目录 ~/.taskbridge/config.yaml
// 3. 当前目录 ./config.yaml
// 4. ./configs/config.yaml
// 5. /etc/taskbridge/config.yaml
func GetConfigPath() string {
	// 优先使用环境变量
	if path := os.Getenv("TASKBRIDGE_CONFIG"); path != "" {
		return path
	}

	// 获取 HOME 目录
	homeDir, err := os.UserHomeDir()
	if err == nil {
		homeConfig := filepath.Join(homeDir, ".taskbridge", "config.yaml")
		if _, err := os.Stat(homeConfig); err == nil {
			return homeConfig
		}
	}

	// 检查其他默认位置
	paths := []string{
		"./config.yaml",
		"./configs/config.yaml",
		"/etc/taskbridge/config.yaml",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 返回 HOME 目录作为默认路径（即使不存在）
	if homeDir != "" {
		return filepath.Join(homeDir, ".taskbridge", "config.yaml")
	}

	return "./configs/config.yaml"
}

// GetDefaultConfigPath 获取默认配置文件路径（用于 init 命令）
// 始终返回 HOME 目录下的路径
func GetDefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./configs/config.yaml"
	}
	return filepath.Join(homeDir, ".taskbridge", "config.yaml")
}
