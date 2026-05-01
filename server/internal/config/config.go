package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig `yaml:"server"`
	Vision VisionConfig `yaml:"vision"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type VisionConfig struct {
	Provider string `yaml:"provider"`
	APIKey   string `yaml:"api_key"`
	Endpoint string `yaml:"endpoint"`
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
