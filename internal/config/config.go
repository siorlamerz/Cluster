package config

import (
	"encoding/json"
	"os"

	"github.com/google/uuid"
)

// // -ldflags
var (
	DefaultGithubURL  = "https://raw.githubusercontent.com/..."
	DefaultRedisAddr  = "0.0.0.0:6379"
	DefaultRedisPass  = ""
	ServiceName       = "CLUSTERAgent"
	BinaryInstallPath = "/usr/local/bin/CLUSTERAgent_service"

	HeartbeatInterval = "30s" // интервал пуша в редис для агента
	ResponseTimeout   = "8s"  // таймаут ответа в клиенте
)

type Config struct {
	DeviceID  string `json:"device_id"`
	Hostname  string `json:"hostname"`
	GithubURL string `json:"github_url"`
	RedisAddr string `json:"redis_addr"`
	RedisPass string `json:"redis_pass"`
	IsRoot    bool   `json:"is_root"`
}

const ConfigFileName = "config.json"

// запущен ли от root
func CheckRoot() bool {
	return os.Geteuid() == 0
}

func getConfigPath() string {
	if CheckRoot() {
		return "/etc/cluster/config.json"
	}
	return ConfigFileName
}

func LoadOrCreate() (*Config, error) {
	path := getConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// еслии мы root нужно создать папку /etc/cluster
		if CheckRoot() {
			os.MkdirAll("/etc/cluster", 0755)
		}

		hostname, _ := os.Hostname()
		newConfig := &Config{
			DeviceID:  uuid.New().String(),
			Hostname:  hostname,
			GithubURL: DefaultGithubURL,
			RedisAddr: DefaultRedisAddr,
			RedisPass: DefaultRedisPass,
		}
		err := SaveConfig(newConfig)
		if err != nil {
			return nil, err
		}
		return newConfig, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	json.Unmarshal(data, &cfg)
	return &cfg, nil
}

func createDefaultConfig() (*Config, error) {
	hostname, _ := os.Hostname()
	newConfig := &Config{
		DeviceID:  uuid.New().String(),
		Hostname:  hostname,
		GithubURL: DefaultGithubURL,
		RedisAddr: DefaultRedisAddr,
		RedisPass: DefaultRedisPass,
	}
	err := SaveConfig(newConfig)
	if err != nil {
		return nil, err
	}
	return newConfig, nil
}

func SaveConfig(cfg *Config) error {
	path := getConfigPath()
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
