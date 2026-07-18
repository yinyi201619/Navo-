// Package config 负责加载与解析 YAML 配置
package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 全局配置结构
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Site     SiteConfig     `mapstructure:"site"`
	Admin    AdminConfig    `mapstructure:"admin"`
	Log      LogConfig      `mapstructure:"log"`
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	DSN      string `mapstructure:"dsn"`
	MaxIdle  int    `mapstructure:"max_idle"`
	MaxOpen  int    `mapstructure:"max_open"`
	LogLevel string `mapstructure:"log_level"`
}

type JWTConfig struct {
	Secret  string        `mapstructure:"secret"`
	Issuer  string        `mapstructure:"issuer"`
	Expire  time.Duration `mapstructure:"expire"`
}

type SiteConfig struct {
	Name          string `mapstructure:"name"`
	Description   string `mapstructure:"description"`
	PerPage       int    `mapstructure:"per_page"`
	AllowRegister bool   `mapstructure:"allow_register"`
	UploadLimitMB int    `mapstructure:"upload_limit_mb"`
}

type AdminConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Email    string `mapstructure:"email"`
}

type LogConfig struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
}

// Load 从指定路径加载配置
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("NAVO")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 默认值
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "data/navo.db")
	v.SetDefault("database.max_idle", 10)
	v.SetDefault("database.max_open", 100)
	v.SetDefault("database.log_level", "warn")
	v.SetDefault("jwt.expire", "168h")
	v.SetDefault("site.per_page", 20)
	v.SetDefault("site.allow_register", true)
	v.SetDefault("site.upload_limit_mb", 5)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.encoding", "json")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
