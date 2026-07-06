package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	MySQL MysqlConfig `mapstructure:"mysql"`
	Redis RedisConfig `mapstructure:"redis"`
}

type MysqlConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

var Conf Config

func init() {
	viper.SetConfigType("yaml")
	viper.SetConfigFile("config.yaml")

	if err := viper.ReadInConfig(); err != nil {
		slog.Error("Failed to read config file", "error", err)
		os.Exit(1)
	}

	if err := viper.Unmarshal(&Conf); err != nil {
		slog.Error("Failed to parse config", "error", err)
		os.Exit(1)
	}

	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		time.Local = loc
	}

	// 环境变量覆盖（用于 Docker 容器）
	if v := os.Getenv("MYSQL_HOST"); v != "" {
		Conf.MySQL.Host = v
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		Conf.Redis.Host = v
	}
}

func GetYamlDsn() string {
	_ = time.Local
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Asia%%2FShanghai",
		Conf.MySQL.User,
		Conf.MySQL.Password,
		Conf.MySQL.Host,
		Conf.MySQL.Port,
		Conf.MySQL.DBName,
	)
}
