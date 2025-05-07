package sylph

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// 批处理常量定义
const (
	defaultLogChanSize    = 10000                 // 增大缓冲区
	defaultLogWorkerCount = 4                     // 默认工作协程数
	maxBatchSize          = 100                   // 最大批处理大小
	batchProcessInterval  = 10 * time.Millisecond // 批处理间隔
)

// AsyncLogger 异步日志处理器，实现 ILogger 接口
type AsyncLogger struct {
	logger       ILogger        // 内部封装实际的日志记录器
	logCh        chan *logEntry // 日志请求通道
	wg           sync.WaitGroup // 等待组，用于优雅关闭
	done         chan struct{}  // 关闭通知
	fallbackLog  bool           // 队列满时是否降级为同步处理
	dropCount    int64          // 丢弃的日志计数（使用原子操作）
	workers      int            // 工作协程数量
	flushTimeout time.Duration  // 关闭时刷新超时时间
}

// logEntryPool 用于重用日志条目，减少内存分配
var logEntryPool = sync.Pool{
	New: func() interface{} {
		return &logEntry{}
	},
}

// getLogEntry 从对象池获取一个日志条目
func getLogEntry() *logEntry {
	return logEntryPool.Get().(*logEntry)
}

// putLogEntry 将日志条目归还到对象池
func putLogEntry(entry *logEntry) {
	// 清空引用，帮助GC
	entry.message = nil
	entry.err = nil
	logEntryPool.Put(entry)
}

// logEntry 表示一个日志条目
type logEntry struct {
	level   logrus.Level
	message *LoggerMessage
	err     error // 仅用于 Error 级别
}

// AsyncLoggerOption 配置选项函数
type AsyncLoggerOption func(*AsyncLogger)

// WithWorkers 设置工作协程数量
func WithWorkers(n int) AsyncLoggerOption {
	return func(a *AsyncLogger) {
		if n > 0 {
			a.workers = n
		}
	}
}

// WithFallback 设置是否启用降级
func WithFallback(enabled bool) AsyncLoggerOption {
	return func(a *AsyncLogger) {
		a.fallbackLog = enabled
	}
}

// WithFlushTimeout 设置关闭时刷新超时时间
func WithFlushTimeout(timeout time.Duration) AsyncLoggerOption {
	return func(a *AsyncLogger) {
		if timeout > 0 {
			a.flushTimeout = timeout
		}
	}
}

// NewAsyncLogger 创建异步日志处理器
func NewAsyncLogger(logger ILogger, bufferSize int, options ...AsyncLoggerOption) *AsyncLogger {
	if bufferSize <= 0 {
		bufferSize = defaultLogChanSize // 使用更大的默认缓冲区
	}

	async := &AsyncLogger{
		logger:       logger,
		logCh:        make(chan *logEntry, bufferSize),
		done:         make(chan struct{}),
		fallbackLog:  true,                  // 默认启用降级
		workers:      defaultLogWorkerCount, // 使用默认工作协程数
		flushTimeout: 3 * time.Second,       // 默认3秒超时
	}

	// 应用配置选项
	for _, option := range options {
		option(async)
	}

	// 启动工作协程池
	for i := 0; i < async.workers; i++ {
		async.wg.Add(1)
		go func() {
			defer async.wg.Done()
			async.processLogs()
		}()
	}

	return async
}

// processLogs 处理日志队列
func (a *AsyncLogger) processLogs() {
	// 预分配批处理数组
	batch := make([]*logEntry, maxBatchSize)
	ticker := time.NewTicker(batchProcessInterval)
	defer ticker.Stop()

	for {
		// 批处理计数
		n := 0

		// 首先尝试获取一个日志条目
		select {
		case entry, ok := <-a.logCh:
			if !ok {
				// 通道已关闭
				return
			}
			batch[0] = entry
			n = 1
		case <-a.done:
			// 收到关闭信号，退出循环
			return
		case <-ticker.C:
			// 定时器触发，检查是否有日志需要处理
			continue
		}

		// 尝试批量获取更多日志条目
	batchCollect:
		for n < maxBatchSize {
			select {
			case entry, ok := <-a.logCh:
				if !ok {
					break batchCollect
				}
				batch[n] = entry
				n++
			default:
				// 没有更多日志可以立即获取
				break batchCollect
			}
		}

		// 处理当前批次的日志
		for i := 0; i < n; i++ {
			// 调用内部logger处理日志
			a.processLogEntry(batch[i])
			putLogEntry(batch[i]) // 归还对象到池
			a.wg.Done()
			batch[i] = nil // 避免内存泄漏
		}
	}
}

// processLogEntry 处理单个日志条目
func (a *AsyncLogger) processLogEntry(entry *logEntry) {
	switch entry.level {
	case logrus.InfoLevel:
		a.logger.Info(entry.message)
	case logrus.TraceLevel:
		a.logger.Trace(entry.message)
	case logrus.DebugLevel:
		a.logger.Debug(entry.message)
	case logrus.WarnLevel:
		a.logger.Warn(entry.message)
	case logrus.ErrorLevel:
		a.logger.Error(entry.message, entry.err)
	case logrus.FatalLevel:
		a.logger.Fatal(entry.message)
	case logrus.PanicLevel:
		a.logger.Panic(entry.message)
	}
}

// GetDropCount 获取丢弃的日志计数
func (a *AsyncLogger) GetDropCount() int64 {
	return atomic.LoadInt64(&a.dropCount)
}

// Close 关闭异步日志处理器
func (a *AsyncLogger) Close() error {
	// 通知工作协程准备关闭
	close(a.done)

	// 使用带超时的等待
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 所有日志处理完成
	case <-time.After(a.flushTimeout):
		// 超时了，可能还有部分日志未处理
	}

	// 关闭日志通道，确保所有工作协程退出
	close(a.logCh)

	// 关闭内部logger
	if closer, ok := a.logger.(interface{ Close() error }); ok {
		return closer.Close()
	}

	return nil
}

// DisableFallback 禁用队列满时的降级处理
func (a *AsyncLogger) DisableFallback() {
	a.fallbackLog = false
}

// 实现 ILogger 接口
func (a *AsyncLogger) Info(message *LoggerMessage) {
	a.enqueueOrFallback(logrus.InfoLevel, message, nil)
}

func (a *AsyncLogger) Trace(message *LoggerMessage) {
	a.enqueueOrFallback(logrus.TraceLevel, message, nil)
}

func (a *AsyncLogger) Debug(message *LoggerMessage) {
	a.enqueueOrFallback(logrus.DebugLevel, message, nil)
}

func (a *AsyncLogger) Warn(message *LoggerMessage) {
	a.enqueueOrFallback(logrus.WarnLevel, message, nil)
}

func (a *AsyncLogger) Fatal(message *LoggerMessage) {
	// Fatal 级别强制同步处理，因为会导致程序退出
	a.logger.Fatal(message)
}

func (a *AsyncLogger) Panic(message *LoggerMessage) {
	// Panic 级别强制同步处理，因为会导致panic
	a.logger.Panic(message)
}

func (a *AsyncLogger) Error(message *LoggerMessage, err error) {
	a.enqueueOrFallback(logrus.ErrorLevel, message, err)
}

// 实现简化API方法
func (a *AsyncLogger) Infof(format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	a.Info(msg)
}

func (a *AsyncLogger) Tracef(format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	a.Trace(msg)
}

func (a *AsyncLogger) Debugf(format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	a.Debug(msg)
}

func (a *AsyncLogger) Warnf(format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	a.Warn(msg)
}

func (a *AsyncLogger) Errorf(err error, format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	a.Error(msg, err)
}

func (a *AsyncLogger) Fatalf(format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	a.Fatal(msg)
}

func (a *AsyncLogger) Panicf(format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	a.Panic(msg)
}

// IsClosed 检查日志记录器是否已关闭
// 直接委托给内部logger
func (a *AsyncLogger) IsClosed() bool {
	if closer, ok := a.logger.(interface{ IsClosed() bool }); ok {
		return closer.IsClosed()
	}
	return false
}

// WithContext 设置上下文
// 创建新的AsyncLogger实例，包装具有新上下文的内部logger
func (a *AsyncLogger) WithContext(ctx context.Context) ILogger {
	if ctx == nil {
		return a
	}

	// 获取内部logger的上下文版本
	innerWithCtx := a.logger.WithContext(ctx)

	// 创建新的异步包装器
	return NewAsyncLogger(
		innerWithCtx,
		cap(a.logCh),
		WithWorkers(a.workers),
		WithFallback(a.fallbackLog),
		WithFlushTimeout(a.flushTimeout),
	)
}

// enqueueOrFallback 将日志加入队列或降级为同步处理
func (a *AsyncLogger) enqueueOrFallback(level logrus.Level, message *LoggerMessage, err error) {
	// 优先处理级别高的日志
	if level == logrus.FatalLevel || level == logrus.PanicLevel {
		a.processLogEntry(&logEntry{level: level, message: message, err: err})
		return
	}

	// 从对象池获取一个日志条目
	entry := getLogEntry()
	entry.level = level
	entry.message = message
	entry.err = err

	a.wg.Add(1)

	// 尝试异步处理
	select {
	case a.logCh <- entry:
		// 成功加入队列
	case <-a.done:
		// 如果正在关闭，直接返回，减少计数
		a.wg.Done()
		putLogEntry(entry) // 归还对象
	default:
		// 队列已满
		a.wg.Done()
		putLogEntry(entry) // 归还对象

		// 增加丢弃计数
		atomic.AddInt64(&a.dropCount, 1)

		// 如果启用了降级，则降级为同步处理
		if a.fallbackLog {
			// 创建临时条目进行处理，避免污染对象池
			tempEntry := &logEntry{level: level, message: message, err: err}
			a.processLogEntry(tempEntry)
		}
	}
}
