package sylph

import (
	"errors"
	"fmt"
	"sync"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

var (
	// _logWriterManager 全局日志写入器管理器实例
	_logWriterManager = newLogWriterManager()

	// ErrEmptyPathOrName 表示路径或名称为空的错误
	ErrEmptyPathOrName = errors.New("path and name cannot be empty")
)

// LogWriterManager 日志写入器管理接口
type LogWriterManager interface {
	GetWriter(path, name string, level logrus.Level) (writer *rotatelogs.RotateLogs)
	// 添加关闭方法，用于清理资源
	Close()
}

// logWriterManager 日志写入器管理器实现
type logWriterManager struct {
	writers   sync.Map
	keyCache  sync.Map // 缓存已生成的key，减少字符串拼接
	pathCache sync.Map // 缓存日志路径配置，减少重复计算
}

// newLogWriterManager 创建新的日志写入器管理器
func newLogWriterManager() LogWriterManager {
	return &logWriterManager{}
}

// makeWriterKey 生成写入器缓存的键
func (m *logWriterManager) makeWriterKey(name string, level logrus.Level) string {
	// 构造复合键用于缓存查找
	cacheKey := name + ":" + level.String()

	// 检查缓存
	if cachedKey, ok := m.keyCache.Load(cacheKey); ok {
		return cachedKey.(string)
	}

	// 生成并缓存key
	key := fmt.Sprintf("x:log:writer:%s:%s", name, level.String())
	m.keyCache.Store(cacheKey, key)
	return key
}

// getLogPathConfig 获取或创建日志路径配置
func (m *logWriterManager) getLogPathConfig(path, name string, level logrus.Level) *LogPathConfig {
	// 构造缓存键
	cacheKey := fmt.Sprintf("%s:%s:%s", path, name, level.String())

	// 检查缓存
	if cachedPaths, ok := m.pathCache.Load(cacheKey); ok {
		return cachedPaths.(*LogPathConfig)
	}

	// 创建新的路径配置
	paths := NewLogPathConfig(path, name, level)
	paths.Init()

	// 存入缓存
	m.pathCache.Store(cacheKey, paths)
	return paths
}

// createWriter 创建新的日志写入器
func (m *logWriterManager) createWriter(path string, name string, level logrus.Level) (*rotatelogs.RotateLogs, error) {
	if path == "" || name == "" {
		return nil, ErrEmptyPathOrName
	}

	paths := m.getLogPathConfig(path, name, level)

	// 默认配置 - 使用局部变量避免冲突
	maxAge := defaultLoggerConfig.MaxAge
	rotationCount := defaultLoggerConfig.RotationCount

	var maxAgeOption rotatelogs.Option
	if maxAge == 0 {
		maxAgeOption = rotatelogs.WithMaxAge(0)
	} else {
		maxAgeOption = rotatelogs.WithMaxAge(time.Duration(maxAge) * time.Hour)
	}

	options := []rotatelogs.Option{
		rotatelogs.WithLinkName(paths.LinkPath),
		maxAgeOption,
		rotatelogs.WithRotationCount(rotationCount),
		rotatelogs.WithRotationTime(time.Hour),
		rotatelogs.WithClock(rotatelogs.Local),
	}

	writer, err := rotatelogs.New(paths.FilePath, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize log writer: path=%s, name=%s, level=%s, error=%w",
			path, name, level.String(), err)
	}

	return writer, nil
}

// GetWriter 获取或创建日志写入器，使用双重检查锁模式
func (m *logWriterManager) GetWriter(path, name string, level logrus.Level) (writer *rotatelogs.RotateLogs) {
	// 参数验证
	if path == "" || name == "" {
		// 返回nil而不是panic，让调用方决定如何处理
		return nil
	}

	key := m.makeWriterKey(name, level)

	// 快速路径：检查是否已存在
	if value, ok := m.writers.Load(key); ok {
		return value.(*rotatelogs.RotateLogs)
	}

	// 慢路径：创建新的writer
	newWriter, err := m.createWriter(path, name, level)
	if err != nil {
		// 记录错误但不panic，返回nil
		fmt.Printf("Error creating log writer: %v\n", err)
		return nil
	}

	// 尝试存储，如果已存在则使用已有的
	actualWriter, loaded := m.writers.LoadOrStore(key, newWriter)
	if loaded {
		// 如果另一个goroutine已经创建了writer，我们就关闭我们创建的这个
		// 避免资源泄漏
		_ = newWriter.Close()
		return actualWriter.(*rotatelogs.RotateLogs)
	}

	// 我们存储的是新创建的writer
	return newWriter
}

// Close 关闭并清理所有writer
func (m *logWriterManager) Close() {
	// 并发关闭所有writer
	var wg sync.WaitGroup

	m.writers.Range(func(key, value interface{}) bool {
		if writer, ok := value.(*rotatelogs.RotateLogs); ok {
			wg.Add(1)
			go func(w *rotatelogs.RotateLogs) {
				defer wg.Done()
				// 忽略关闭错误，只需尽最大努力关闭
				_ = w.Close()
			}(writer)
		}
		// 从map中删除
		m.writers.Delete(key)
		return true
	})

	// 等待所有writer关闭
	wg.Wait()

	// 清空缓存
	m.keyCache = sync.Map{}
	m.pathCache = sync.Map{}
}
