package config

import (
	"fmt"
	"os"

	"github.com/czh0727/asy/internal/configloader"
)

var Conf *TomlConfig

type TomlConfig struct {
	FlowsvrAddr       string `toml:"flowsvr_addr"`
	RedisLockAddr     string `toml:"redis_lock_addr"`
	RedisLockPassword string `toml:"redis_lock_password"`
}

func (c *TomlConfig) Validate() error {
	if c.FlowsvrAddr == "" {
		return fmt.Errorf(
			"flowsvr_addr is required",
		)
	}
	if c.RedisLockAddr == "" {
		return fmt.Errorf(
			"redis_lock_addr is required",
		)
	}
	return nil
}

func LoadConfigFromRoot(
	root string,
	c *TomlConfig,
) error {
	if c == nil {
		return fmt.Errorf("config target is nil")
	}

	if err := configloader.Load(root, c); err != nil {
		return err
	}

	return c.Validate()
}

func (c *TomlConfig) LoadConfig() {
	if err := LoadConfigFromRoot(
		rootDir, c,
	); err != nil {
		panic(err)
	}
}

func Init() {
	Conf = new(TomlConfig)
	Conf.LoadConfig()
}

func GetConfigPath() string {
	return configloader.ConfigPath(rootDir)
}

var rootDir string

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	rootDir, err = configloader.FindRoot(
		cwd, "main.go",
	)
	if err != nil {
		panic(err)
	}
}
