package frontier

import "time"

// 超时配置常量
const (
	// writeWait 写入等待时间
	writeWait = 10 * time.Second

	// pongWait 等待pong消息的时间
	pongWait = 60 * time.Second

	// pingPeriod 发送ping消息的间隔（必须小于pongWait）
	pingPeriod = (pongWait * 9) / 10

	// maxMessageSize 最大消息大小
	maxMessageSize = 512 * 1024 // 512KB
)

// Config WebSocket服务配置
type Config struct {
	// 服务地址
	Host string `yaml:"host"`
	Port string `yaml:"port"`

	// 心跳配置
	HeartbeatInterval time.Duration `yaml:"heartbeatInterval"`
	ConnectionTimeout time.Duration `yaml:"connectionTimeout"`

	// 消息配置
	MaxMessageSize    int64 `yaml:"maxMessageSize"`
	SendBufferSize    int   `yaml:"sendBufferSize"`
	ReceiveBufferSize int   `yaml:"receiveBufferSize"`

	// TLS配置
	EnableTLS bool   `yaml:"enableTLS"`
	CertFile  string `yaml:"certFile"`
	KeyFile   string `yaml:"keyFile"`

	// 其他配置
	MaxConnections int           `yaml:"maxConnections"`
	ReadTimeout    time.Duration `yaml:"readTimeout"`
	WriteTimeout   time.Duration `yaml:"writeTimeout"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Host:              "0.0.0.0",
		Port:              "8080",
		HeartbeatInterval: 30 * time.Second,
		ConnectionTimeout: 90 * time.Second,
		MaxMessageSize:    512 * 1024,
		SendBufferSize:    256,
		ReceiveBufferSize: 1024,
		EnableTLS:         false,
		MaxConnections:    10000,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = 30 * time.Second
	}
	if c.ConnectionTimeout == 0 {
		c.ConnectionTimeout = 90 * time.Second
	}
	if c.MaxMessageSize == 0 {
		c.MaxMessageSize = 512 * 1024
	}
	return nil
}
