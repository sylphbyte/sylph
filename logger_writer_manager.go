package sylph

import (
	"fmt"
	"sync"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

var (
	// _logWriterManager 全局日志写入器管理器实例
	_logWriterManager = newLogWriterManager()
)

// LogWriterManager 日志写入器管理接口
type LogWriterManager interface {
	GetWriter(path, name string, level logrus.Level) (writer *rotatelogs.RotateLogs)
}

// logWriterManager 日志写入器管理器实现
type logWriterManager struct {
	writers sync.Map
}

// newLogWriterManager 创建新的日志写入器管理器
func newLogWriterManager() LogWriterManager {
	return &logWriterManager{}
}

// makeWriterKey 生成写入器缓存的键
func (m *logWriterManager) makeWriterKey(name string, level logrus.Level) string {
	return fmt.Sprintf("x:log:writer:%s:%s", name, level.String())
}

// createWriter 创建新的日志写入器
func (m *logWriterManager) createWriter(path string, name string, level logrus.Level) (writer *rotatelogs.RotateLogs) {
	if path == "" || name == "" {
		panic("path and name cannot be empty")
	}

	paths := NewLogPathConfig(path, name, level)
	paths.Init()

	// 默认配置
	config := struct {
		maxAge        uint
		rotationCount uint
	}{
		maxAge:        defaultLoggerConfig.MaxAge,        // 默认保留7天
		rotationCount: defaultLoggerConfig.RotationCount, // 默认保留168个文件（按小时切割，对应7天）
	}

	var maxAgeOption rotatelogs.Option
	if config.maxAge == 0 {
		maxAgeOption = rotatelogs.WithMaxAge(0)
	} else {
		maxAgeOption = rotatelogs.WithMaxAge(time.Duration(config.maxAge) * time.Hour)
	}

	options := []rotatelogs.Option{
		rotatelogs.WithLinkName(paths.LinkPath),
		maxAgeOption,
		rotatelogs.WithRotationCount(config.rotationCount),
		rotatelogs.WithRotationTime(time.Hour),
		rotatelogs.WithClock(rotatelogs.Local),
	}

	var err error
	if writer, err = rotatelogs.New(paths.FilePath, options...); err != nil {
		panic(fmt.Sprintf("Failed to initialize log writer: path=%s, name=%s, level=%s, error=%v",
			path, name, level.String(), err))
	}

	return writer
}

// GetWriter 获取或创建日志写入器
func (m *logWriterManager) GetWriter(path, name string, level logrus.Level) (writer *rotatelogs.RotateLogs) {
	key := m.makeWriterKey(name, level)
	if value, ok := m.writers.Load(key); ok {
		return value.(*rotatelogs.RotateLogs)
	}

	writer = m.createWriter(path, name, level)
	m.writers.Store(key, writer)
	return
}
