package sylph

import (
	"fmt"
	"sync"
	"sync/atomic"
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
	asyncEnabled  int32        // 使用原子操作代替布尔值
	asyncBuffSize int32        // 使用原子操作以避免锁竞争
	asyncLoggers  sync.Map     // 存储异步包装器
	mu            sync.RWMutex // 使用读写锁提高并发性能
	// 添加key缓存以减少字符串拼接开销
	keyCache sync.Map // 缓存已生成的key
}

func newLoggerManager() ILoggerManager {
	return &loggerManager{
		asyncEnabled:  0,    // 0表示禁用
		asyncBuffSize: 1000, // 默认缓冲区大小
	}
}

func (l *loggerManager) makeKey(name string) string {
	// 首先检查缓存中是否已存在此key
	if cachedKey, ok := l.keyCache.Load(name); ok {
		return cachedKey.(string)
	}

	// 不存在则创建并缓存
	key := fmt.Sprintf("x:logger:%s", name)
	l.keyCache.Store(name, key)
	return key
}

func (l *loggerManager) isAsyncEnabled() bool {
	return atomic.LoadInt32(&l.asyncEnabled) == 1
}

func (l *loggerManager) getAsyncBuffSize() int {
	return int(atomic.LoadInt32(&l.asyncBuffSize))
}

func (l *loggerManager) make(name string) (logger ILogger) {
	logger = DefaultLogger(name)

	if l.isAsyncEnabled() {
		return NewAsyncLogger(logger, l.getAsyncBuffSize())
	}

	return logger
}

// Receive 使用双重检查锁模式获取logger，减少锁竞争
func (l *loggerManager) Receive(name string) (logger ILogger) {
	key := l.makeKey(name)

	// 快速路径: 尝试直接加载已存在的logger
	if value, ok := l.loggers.Load(key); ok {
		return value.(ILogger)
	}

	// 慢路径: 创建新的logger（可能有多个goroutine同时进入）
	newLogger := l.make(name)

	// 使用LoadOrStore确保即使有并发创建，我们也只存储一个logger实例
	actualValue, loaded := l.loggers.LoadOrStore(key, newLogger)
	if loaded {
		// 如果另一个goroutine已经存储了一个logger，则使用那个并清理我们创建的
		if asyncLogger, ok := newLogger.(*AsyncLogger); ok && !l.isAsyncEnabled() {
			// 如果我们创建了AsyncLogger但不再需要它，关闭它
			asyncLogger.Close()
		}
		return actualValue.(ILogger)
	}

	// 我们存储的是新创建的logger
	return newLogger
}

// EnableAsync 启用异步日志
func (l *loggerManager) EnableAsync(bufferSize int) {
	// 先检查是否已启用，避免不必要的锁获取
	if l.isAsyncEnabled() && bufferSize <= l.getAsyncBuffSize() {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 更新配置
	if bufferSize > 0 {
		atomic.StoreInt32(&l.asyncBuffSize, int32(bufferSize))
	}

	// 已经启用则不需要再处理
	if atomic.LoadInt32(&l.asyncEnabled) == 1 {
		return
	}

	atomic.StoreInt32(&l.asyncEnabled, 1)

	// 对已存在的logger进行异步包装
	l.loggers.Range(func(key, value interface{}) bool {
		if logger, ok := value.(ILogger); ok {
			// 检查是否已经是异步logger
			if _, isAsync := logger.(*AsyncLogger); !isAsync {
				bufSize := l.getAsyncBuffSize()
				asyncLogger := NewAsyncLogger(logger, bufSize)
				l.asyncLoggers.Store(key, asyncLogger)
				l.loggers.Store(key, asyncLogger)
			}
		}
		return true
	})
}

// DisableAsync 禁用异步日志
func (l *loggerManager) DisableAsync() {
	// 快速检查，避免不必要的锁
	if !l.isAsyncEnabled() {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 已经禁用则不需要再处理
	if atomic.LoadInt32(&l.asyncEnabled) == 0 {
		return
	}

	atomic.StoreInt32(&l.asyncEnabled, 0)

	// 使用并发处理关闭所有异步logger
	var wg sync.WaitGroup
	l.asyncLoggers.Range(func(key, value interface{}) bool {
		if asyncLogger, ok := value.(*AsyncLogger); ok {
			wg.Add(1)
			go func(k interface{}, al *AsyncLogger) {
				defer wg.Done()

				// 关闭异步logger
				al.Close()

				// 恢复原始logger，如果可能的话
				if origLogger := al.logger; origLogger != nil {
					l.loggers.Store(k, origLogger)
				}
			}(key, asyncLogger)
		}
		return true
	})

	// 等待所有关闭操作完成
	wg.Wait()

	// 清空异步logger缓存
	l.asyncLoggers = sync.Map{}
}

// Close 关闭所有资源
func (l *loggerManager) Close() {
	// 先禁用异步日志
	l.DisableAsync()

	// 清理其他资源
	l.loggers.Range(func(key, value interface{}) bool {
		if closer, ok := value.(interface{ Close() }); ok {
			closer.Close()
		}
		l.loggers.Delete(key)
		return true
	})

	// 清空key缓存
	l.keyCache = sync.Map{}
}
