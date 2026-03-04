package tokenstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yeisme/taskbridge/pkg/paths"
)

const currentVersion = 1

type fileStore struct {
	Version   int                        `json:"version"`
	Providers map[string]json.RawMessage `json:"providers"`
}

var fileMu sync.Mutex

// Load 从统一 token 文件加载指定 provider 的 token 到 out。
func Load(path, provider string, out interface{}) error {
	raw, err := LoadRaw(path, provider)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("failed to parse %s token: %w", provider, err)
	}
	return nil
}

// LoadRaw 从统一 token 文件加载指定 provider 的原始 token JSON。
func LoadRaw(path, provider string) ([]byte, error) {
	tokenPath, providerName, err := normalizeInput(path, provider)
	if err != nil {
		return nil, err
	}

	fileMu.Lock()
	defer fileMu.Unlock()

	raw, err := loadRawLocked(tokenPath, providerName)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), raw...), nil
}

// Save 将指定 provider 的 token 写入统一 token 文件。
func Save(path, provider string, token interface{}) error {
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal %s token: %w", provider, err)
	}
	return SaveRaw(path, provider, data)
}

// SaveRaw 将指定 provider 的原始 token JSON 写入统一 token 文件。
func SaveRaw(path, provider string, raw []byte) error {
	tokenPath, providerName, err := normalizeInput(path, provider)
	if err != nil {
		return err
	}

	payload := bytes.TrimSpace(raw)
	if len(payload) == 0 {
		return fmt.Errorf("empty token payload for provider %s", providerName)
	}

	fileMu.Lock()
	defer fileMu.Unlock()

	store, err := loadStoreForWriteLocked(tokenPath, providerName)
	if err != nil {
		return err
	}
	store.Providers[providerName] = append([]byte(nil), payload...)

	if err := writeStoreLocked(tokenPath, store); err != nil {
		return err
	}
	return removeLegacyFileLocked(tokenPath, providerName)
}

// Delete 删除指定 provider 的 token。
func Delete(path, provider string) error {
	tokenPath, providerName, err := normalizeInput(path, provider)
	if err != nil {
		return err
	}

	fileMu.Lock()
	defer fileMu.Unlock()

	if err := deleteFromStoreLocked(tokenPath, providerName); err != nil {
		return err
	}
	return removeLegacyFileLocked(tokenPath, providerName)
}

// Has 检查指定 provider 是否存在 token。
func Has(path, provider string) (bool, error) {
	_, err := LoadRaw(path, provider)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func loadRawLocked(path, provider string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		store, isStore, parseErr := decodeStore(data)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse token store file %s: %w", path, parseErr)
		}
		if isStore {
			if raw, ok := store.Providers[provider]; ok && len(bytes.TrimSpace(raw)) > 0 {
				return raw, nil
			}
		} else if canUseDirectPayload(path, provider) && len(bytes.TrimSpace(data)) > 0 {
			return data, nil
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read token file %s: %w", path, err)
	}

	legacyPath := paths.GetLegacyTokenPath(provider)
	if !samePath(path, legacyPath) {
		legacyData, legacyErr := os.ReadFile(legacyPath)
		if legacyErr == nil && len(bytes.TrimSpace(legacyData)) > 0 {
			return legacyData, nil
		}
		if legacyErr != nil && !os.IsNotExist(legacyErr) {
			return nil, fmt.Errorf("failed to read legacy token file %s: %w", legacyPath, legacyErr)
		}
	}

	return nil, fmt.Errorf("%w: token for provider %s", os.ErrNotExist, provider)
}

func loadStoreForWriteLocked(path, provider string) (*fileStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newStore(), nil
		}
		return nil, fmt.Errorf("failed to read token file %s: %w", path, err)
	}

	store, isStore, parseErr := decodeStore(data)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse token store file %s: %w", path, parseErr)
	}
	if isStore {
		return &store, nil
	}

	legacyStore := newStore()
	if len(bytes.TrimSpace(data)) > 0 {
		legacyStore.Providers[provider] = append([]byte(nil), bytes.TrimSpace(data)...)
	}
	return legacyStore, nil
}

func deleteFromStoreLocked(path, provider string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read token file %s: %w", path, err)
	}

	store, isStore, parseErr := decodeStore(data)
	if parseErr != nil {
		return fmt.Errorf("failed to parse token store file %s: %w", path, parseErr)
	}

	if isStore {
		delete(store.Providers, provider)
		if len(store.Providers) == 0 {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove token file %s: %w", path, err)
			}
			return nil
		}
		return writeStoreLocked(path, &store)
	}

	if samePath(path, paths.GetLegacyTokenPath(provider)) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove token file %s: %w", path, err)
		}
	}
	return nil
}

func writeStoreLocked(path string, store *fileStore) error {
	if store == nil {
		return fmt.Errorf("token store is nil")
	}
	if store.Providers == nil {
		store.Providers = map[string]json.RawMessage{}
	}
	if store.Version == 0 {
		store.Version = currentVersion
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token store: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}
	return nil
}

func removeLegacyFileLocked(currentPath, provider string) error {
	legacyPath := paths.GetLegacyTokenPath(provider)
	if samePath(currentPath, legacyPath) {
		return nil
	}
	if err := os.Remove(legacyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove legacy token file %s: %w", legacyPath, err)
	}
	return nil
}

func decodeStore(data []byte) (fileStore, bool, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return fileStore{}, false, nil
	}

	providersRaw, hasProviders := root["providers"]
	if !hasProviders {
		return fileStore{}, false, nil
	}

	store := *newStore()
	if versionRaw, ok := root["version"]; ok {
		_ = json.Unmarshal(versionRaw, &store.Version)
	}

	trimmedProviders := bytes.TrimSpace(providersRaw)
	if len(trimmedProviders) > 0 && !bytes.Equal(trimmedProviders, []byte("null")) {
		if err := json.Unmarshal(providersRaw, &store.Providers); err != nil {
			return fileStore{}, true, err
		}
	}

	if store.Version == 0 {
		store.Version = currentVersion
	}
	if store.Providers == nil {
		store.Providers = map[string]json.RawMessage{}
	}

	return store, true, nil
}

func newStore() *fileStore {
	return &fileStore{
		Version:   currentVersion,
		Providers: map[string]json.RawMessage{},
	}
}

func normalizeInput(path, provider string) (string, string, error) {
	tokenPath := strings.TrimSpace(path)
	if tokenPath == "" {
		return "", "", fmt.Errorf("token file path is empty")
	}
	providerName := strings.ToLower(strings.TrimSpace(provider))
	if providerName == "" {
		return "", "", fmt.Errorf("provider is empty")
	}
	return tokenPath, providerName, nil
}

func samePath(a, b string) bool {
	cleanA := filepath.Clean(strings.TrimSpace(a))
	cleanB := filepath.Clean(strings.TrimSpace(b))
	return strings.EqualFold(cleanA, cleanB)
}

func canUseDirectPayload(path, provider string) bool {
	if samePath(path, paths.GetLegacyTokenPath(provider)) {
		return true
	}
	return !samePath(path, paths.GetTokenPath(provider))
}
