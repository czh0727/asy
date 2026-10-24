package configloader

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const defaultConfigRelPath = "config/config.toml"

// FindRoot 从当前目录逐级向上查找 marker 文件。
func FindRoot(startDir, marker string) (string, error) {
	if startDir == "" || marker == "" {
		return "", fmt.Errorf(
			"start dir and marker must not be empty",
		)
	}

	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf(
			"absolute start dir: %w", err,
		)
	}

	for {
		if _, err := os.Stat(
			filepath.Join(dir, marker),
		); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf(
				"cannot find %q from %q",
				marker, startDir,
			)
		}

		dir = parent
	}
}

// ConfigPath 生成配置文件的绝对路径。
func ConfigPath(root string) string {
	return filepath.Join(root, defaultConfigRelPath)
}

// Load 读取 TOML 并解析到目标结构体。
func Load(root string, target interface{}) error {
	path := ConfigPath(root)

	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf(
			"stat config %q: %w", path, err,
		)
	}

	if _, err := toml.DecodeFile(
		path, target,
	); err != nil {
		return fmt.Errorf(
			"decode config %q: %w", path, err,
		)
	}

	return nil
}
