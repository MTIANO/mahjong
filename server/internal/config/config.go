package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Vision VisionConfig `yaml:"vision"`
	MySQL  MySQLConfig  `yaml:"mysql"`
	WeChat WeChatConfig `yaml:"wechat"`
	JWT    JWTConfig    `yaml:"jwt"`
	Stock  StockConfig  `yaml:"stock"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type VisionConfig struct {
	Provider string `yaml:"provider"`
	APIKey   string `yaml:"api_key"`
	Endpoint string `yaml:"endpoint"`
	Model    string `yaml:"model"`
}

type MySQLConfig struct {
	DSN string `yaml:"dsn"`
}

type WeChatConfig struct {
	AppID  string `yaml:"appid"`
	Secret string `yaml:"secret"`
}

type JWTConfig struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire_hours"`
}

type StockConfig struct {
	AKShareEndpoint string `yaml:"akshare_endpoint"`
	// 题材初筛(Qwen):每次 cron 调一次,产出今日热点板块列表
	ThemeModel    string `yaml:"theme_model"`
	ThemeAPIKey   string `yaml:"theme_api_key"`
	ThemeEndpoint string `yaml:"theme_endpoint"`
	// 精打分(DeepSeek):每只股调一次,基于盘口 + 注入的热点列表打分
	ScorerModel    string `yaml:"scorer_model"`
	ScorerAPIKey   string `yaml:"scorer_api_key"`
	ScorerEndpoint string `yaml:"scorer_endpoint"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
