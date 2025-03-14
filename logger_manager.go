package sylph

import (
	"fmt"
	"sync"
)

var (
	_loggerManager = newLoggerManager()
)

type ILoggerManager interface {
	Receive(name string) (logger ILogger)
	EnableAsync(bufferSize int) // 启用异步日志
	DisableAsync()              // 禁用异步日志
	Close()                     // 关闭资源
}

// 这里 只有在服务中使用
type loggerManager struct {
	loggers       sync.Map
	asyncEnabled  bool
	asyncBuffSize int
	asyncLoggers  sync.Map   // 存储异步包装器
	mu            sync.Mutex // 保护配置修改
}

func newLoggerManager() ILoggerManager {
	return &loggerManager{
		asyncEnabled:  false,
		asyncBuffSize: 1000, // 默认缓冲区大小
	}
}

func (l *loggerManager) makeKey(name string) string {
	return fmt.Sprintf("x:logger:%s", name)
}

func (l *loggerManager) make(name string) (logger ILogger) {
	logger = DefaultLogger(name)

	if l.asyncEnabled {
		return NewAsyncLogger(logger, l.asyncBuffSize)
	}

	return logger
}

// EnableAsync 启用异步日志
func (l *loggerManager) EnableAsync(bufferSize int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.asyncEnabled = true
	if bufferSize > 0 {
		l.asyncBuffSize = bufferSize
	}

	// 对已存在的logger进行异步包装
	l.loggers.Range(func(key, value interface{}) bool {
		if logger, ok := value.(ILogger); ok {
			// 检查是否已经是异步logger
			if _, isAsync := logger.(*AsyncLogger); !isAsync {
				asyncLogger := NewAsyncLogger(logger, l.asyncBuffSize)
				l.asyncLoggers.Store(key, asyncLogger)
				l.loggers.Store(key, asyncLogger)
			}
		}
		return true
	})
}

// DisableAsync 禁用异步日志
func (l *loggerManager) DisableAsync() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.asyncEnabled = false

	// 关闭所有异步logger
	l.asyncLoggers.Range(func(key, value interface{}) bool {
		if asyncLogger, ok := value.(*AsyncLogger); ok {
			asyncLogger.Close()

			// 恢复原始logger，如果可能的话
			if origLogger := asyncLogger.logger; origLogger != nil {
				l.loggers.Store(key, origLogger)
			}
		}
		return true
	})

	// 清空异步logger缓存
	l.asyncLoggers = sync.Map{}
}

// Close 关闭所有资源
func (l *loggerManager) Close() {
	// 关闭所有异步logger
	l.DisableAsync()

	// 清理其他资源，如果有的话
}

func (l *loggerManager) Receive(name string) (logger ILogger) {
	key := l.makeKey(name)
	if value, ok := l.loggers.Load(key); ok {
		return value.(ILogger)
	}

	logger = l.make(name)
	l.loggers.Store(key, logger)
	return
}
