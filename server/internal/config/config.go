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
	AIModel         string `yaml:"ai_model"`
	AIAPIKey        string `yaml:"ai_api_key"`
	AIEndpoint      string `yaml:"ai_endpoint"`
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
