package provider

import (
	"fmt"
	"sync"

	"github.com/yeisme/taskbridge/internal/model"
)

// Registry Provider 注册中心
type Registry struct {
	mu        sync.RWMutex
	providers map[model.TaskSource]Provider
	factories map[model.TaskSource]Factory
}

// Factory Provider 工厂函数
type Factory func(config map[string]interface{}) (Provider, error)

// NewRegistry 创建新的注册中心
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[model.TaskSource]Provider),
		factories: make(map[model.TaskSource]Factory),
	}
}

// Register 注册 Provider 工厂
func (r *Registry) Register(source model.TaskSource, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[source] = factory
}

// Create 创建 Provider 实例
func (r *Registry) Create(source model.TaskSource, config map[string]interface{}) (Provider, error) {
	r.mu.RLock()
	factory, ok := r.factories[source]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider not registered: %s", source)
	}

	provider, err := factory(config)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.providers[source] = provider
	r.mu.Unlock()

	return provider, nil
}

// Get 获取已创建的 Provider
func (r *Registry) Get(source model.TaskSource) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[source]
	return provider, ok
}

// GetAll 获取所有已创建的 Provider
func (r *Registry) GetAll() map[model.TaskSource]Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[model.TaskSource]Provider, len(r.providers))
	for k, v := range r.providers {
		result[k] = v
	}
	return result
}

// ListRegistered 列出所有已注册的 Provider
func (r *Registry) ListRegistered() []model.TaskSource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sources := make([]model.TaskSource, 0, len(r.factories))
	for source := range r.factories {
		sources = append(sources, source)
	}
	return sources
}

// GlobalRegistry 全局注册中心
var GlobalRegistry = NewRegistry()

// RegisterProvider 注册 Provider 到全局注册中心
func RegisterProvider(source model.TaskSource, factory Factory) {
	GlobalRegistry.Register(source, factory)
}

// CreateProvider 创建 Provider 实例
func CreateProvider(source model.TaskSource, config map[string]interface{}) (Provider, error) {
	return GlobalRegistry.Create(source, config)
}

// GetProvider 获取已创建的 Provider
func GetProvider(source model.TaskSource) (Provider, bool) {
	return GlobalRegistry.Get(source)
}
