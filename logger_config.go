package sylph

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// LogLinkPathFormat 日志软链接路径格式
	// %s: 基础路径
	// %s: 服务名
	// %s: 日志级别
	// %s: 服务ID

	LogLinkPathFormat     = "%s/%s/%s.%s.log"
	LogLinkPathFormatServ = "%s/%s/%s.%s-%s.log"

	// LogFilePathFormat 日志文件路径格式
	// %s: 基础路径
	// %%Y%%m/%%d: 年月日目录
	// %s: 服务名
	// %%H: 小时
	// %s: 日志级别
	// %s: 服务ID
	LogFilePathFormat     = "%s/%%Y%%m/%%d/%s.%%H.%s.log"
	LogFilePathFormatServ = "%s/%%Y%%m/%%d/%s.%%H.%s-%s.log"
)

var (
	defaultLoggerConfig *LoggerConfig
)

func init() {
	// 如果没有显式设置配置，使用默认配置
	if defaultLoggerConfig == nil {
		defaultLoggerConfig = &LoggerConfig{
			Stdout:              true,
			PrintError:          true,
			Level:               "debug",
			BufferCap:           1000,
			BufferSize:          100,
			BufferFlushInterval: 5,
		}
	}
}

func InjectLoggerConfig(config *LoggerConfig) {
	defaultLoggerConfig = config
}

// GetDefaultLoggerConfig 获取默认日志配置
func GetDefaultLoggerConfig() *LoggerConfig {
	return defaultLoggerConfig
}

type LoggerConfig struct {
	Stdout        bool   `yaml:"stdout"`
	PrintError    bool   `yaml:"print_error" mapstructure:"print_error"`
	Path          string `yaml:"path"`
	Level         string `yaml:"level"`
	MaxAge        uint   `yaml:"max_age" mapstructure:"rotation_count"`
	RotationCount uint   `yaml:"rotation_count" mapstructure:"rotation_count"`
	Async         bool   `yaml:"async" mapstructure:"async"`

	// 缓存池设置
	BufferCap           uint `yaml:"buffer_cap" mapstructure:"buffer_cap"`
	BufferSize          uint `yaml:"buffer_size" mapstructure:"buffer_size"`
	BufferFlushInterval uint `yaml:"buffer_flush_interval" mapstructure:"buffer_flush_interval"`
	//  池配置
	SwitchPool bool `yaml:"switch_pool" mapstructure:"switch_pool"`
	PoolSize   uint `yaml:"pool_size" mapstructure:"pool_size"`
}

func (l LoggerConfig) stdout() io.Writer {
	if l.Stdout {
		return os.Stderr
	}

	file, _ := os.OpenFile(os.DevNull, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	return file
}

func (l LoggerConfig) level() (level logrus.Level) {
	level, _ = logrus.ParseLevel(l.Level)
	return
}

func (l LoggerConfig) TakeFlushDuration() time.Duration {
	return time.Duration(l.BufferFlushInterval) * time.Second
}

type LogPathConfig struct {
	BasePath string
	Name     string
	Level    logrus.Level
	LinkPath string
	FilePath string
}

func NewLogPathConfig(basePath string, name string, level logrus.Level) *LogPathConfig {
	return &LogPathConfig{BasePath: basePath, Name: name, Level: level}
}

func (c *LogPathConfig) Init() {
	date := time.Now()

	if servId == "" {
		c.LinkPath = fmt.Sprintf(LogLinkPathFormat, c.BasePath, date.Format("200601/02"), c.Name, c.Level.String())
		c.FilePath = fmt.Sprintf(LogFilePathFormat, c.BasePath, c.Name, c.Level.String())
		return
	}
	c.LinkPath = fmt.Sprintf(LogLinkPathFormatServ, c.BasePath, date.Format("200601/02"), c.Name, c.Level.String(), servId)
	c.FilePath = fmt.Sprintf(LogFilePathFormatServ, c.BasePath, c.Name, c.Level.String(), servId)
}
