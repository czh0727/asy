package config

import (
	"fmt"
	"os"

	"github.com/czh0727/asy/internal/configloader"
)

var Conf *TomlConfig

// TomlConfig 对应 flowsvr/src/config/config.toml。
type TomlConfig struct {
	Common commonConfig
	MySQL  mysqlConfig
	Redis  redisConfig
	Task   TaskConfig
}

type commonConfig struct {
	Port    int  `toml:"port"`
	OpenTLS bool `toml:"open_tls"`
}

type mysqlConfig struct {
	Url         string `toml:"url"`
	User        string `toml:"user"`
	Pwd         string `toml:"pwd"`
	Dbname      string `toml:"db_name"`
	MaxIdleConn int    `toml:"max_idle"`
	MaxConn     int    `toml:"max_active"`
	IdleTimeout int    `toml:"idle_timeout"`
}

type redisConfig struct {
	Url             string `toml:"url"`
	Auth            string `toml:"auth"`
	MaxIdle         int    `toml:"max_idle"`
	MaxActive       int    `toml:"max_active"`
	IdleTimeout     int    `toml:"idle_timeout"`
	CacheTimeoutDay int    `toml:"cache_timeout_day"`
}

type TaskConfig struct {
	TableMaxRows        int   `toml:"table_max_rows"`
	AliveThreshold      int   `toml:"alive_threshold"`
	SplitInterval       int   `toml:"split_interval"`
	LongProcessInterval int   `toml:"long_process_interval"`
	MoveInterval        int   `toml:"move_interval"`
	MaxProcessTime      int64 `toml:"max_process_time"`
}

// Validate 校验配置项。
func (c *TomlConfig) Validate() error {
	if c.Common.Port <= 0 {
		return fmt.Errorf(
			"common.port must be greater than 0",
		)
	}
	if c.MySQL.Url == "" {
		return fmt.Errorf("mysql.url is required")
	}
	if c.MySQL.User == "" {
		return fmt.Errorf("mysql.user is required")
	}
	if c.MySQL.Dbname == "" {
		return fmt.Errorf("mysql.db_name is required")
	}
	if c.Redis.Url == "" {
		return fmt.Errorf("redis.url is required")
	}

	if c.MySQL.MaxIdleConn < 0 ||
		c.MySQL.MaxConn < 0 ||
		c.MySQL.IdleTimeout < 0 {
		return fmt.Errorf(
			"mysql pool settings must be non-negative",
		)
	}

	if c.Redis.MaxIdle < 0 ||
		c.Redis.MaxActive < 0 ||
		c.Redis.IdleTimeout < 0 ||
		c.Redis.CacheTimeoutDay < 0 {
		return fmt.Errorf(
			"redis settings must be non-negative",
		)
	}

	if c.Task.TableMaxRows < 0 ||
		c.Task.AliveThreshold < 0 ||
		c.Task.SplitInterval < 0 ||
		c.Task.LongProcessInterval < 0 ||
		c.Task.MoveInterval < 0 ||
		c.Task.MaxProcessTime < 0 {
		return fmt.Errorf(
			"task settings must be non-negative",
		)
	}

	return nil
}

// LoadConfigFromRoot 根据指定组件根目录加载配置。
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

// LoadConfig 保留原项目的调用风格。
func (c *TomlConfig) LoadConfig() {
	if err := LoadConfigFromRoot(
		rootDir, c,
	); err != nil {
		panic(err)
	}
}

// Init 初始化全局配置。
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
