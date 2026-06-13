package config

import (
	"os"
	"strings"
	"time"

	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
	"gopkg.in/yaml.v3"
)

// Config 配置结构
type Config struct {
	Registry RegistryConfig  `yaml:"registry"`
	Redis    RedisConfig     `yaml:"redis"`
	Frontier frontier.Config `yaml:"frontier"`
}

// RegistryConfig 注册中心配置
type RegistryConfig struct {
	Etcd EtcdConfig `yaml:"etcd"`
}

// EtcdConfig etcd配置
type EtcdConfig struct {
	Endpoints []string `yaml:"endpoints"`
	RootPath  string   `yaml:"rootPath"`
}

// RedisConfig Redis服务配置
type RedisConfig struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"` // Redis 密码
}

// Load 加载配置
func Load(configPath string) (*Config, error) {
	// 优先使用环境变量
	if endpoints := os.Getenv("ETCD_ENDPOINTS"); endpoints != "" {
		return &Config{
			Registry: RegistryConfig{
				Etcd: EtcdConfig{
					Endpoints: strings.Split(endpoints, ","),
					RootPath:  getEnvOrDefault("ETCD_ROOT_PATH", "/long-connection-service"),
				},
			},
			Redis: RedisConfig{
				Address:  getEnvOrDefault("REDIS_ADDRESS", "localhost:6379"),
				Password: os.Getenv("REDIS_PASSWORD"),
			},
		}, nil
	}

	// 从配置文件读取
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// 设置默认值
	setFrontierDefaults(&config.Frontier)
	setRedisDefaults(&config.Redis)

	return &config, nil
}

func setRedisDefaults(r *RedisConfig) {
	if r.Address == "" {
		r.Address = "localhost:6379"
	}
}

// setFrontierDefaults 设置 Frontier 配置的默认值
func setFrontierDefaults(f *frontier.Config) {
	if f.Host == "" {
		f.Host = "0.0.0.0"
	}
	if f.Port == "" {
		f.Port = "8080"
	}
	if f.HeartbeatInterval == 0 {
		f.HeartbeatInterval = 30 * time.Second
	}
	if f.ConnectionTimeout == 0 {
		f.ConnectionTimeout = 90 * time.Second
	}
	if f.MaxMessageSize == 0 {
		f.MaxMessageSize = 512 * 1024
	}
	if f.SendBufferSize == 0 {
		f.SendBufferSize = 256
	}
	if f.ReceiveBufferSize == 0 {
		f.ReceiveBufferSize = 1024
	}
	if f.MaxConnections == 0 {
		f.MaxConnections = 10000
	}
	if f.ReadTimeout == 0 {
		f.ReadTimeout = 60 * time.Second
	}
	if f.WriteTimeout == 0 {
		f.WriteTimeout = 10 * time.Second
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
