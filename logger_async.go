package sylph

import (
	"sync"

	"github.com/sirupsen/logrus"
)

// AsyncLogger 异步日志处理器，实现 ILogger 接口
type AsyncLogger struct {
	logger      ILogger        // 内部封装实际的日志记录器
	logCh       chan *logEntry // 日志请求通道
	wg          sync.WaitGroup // 等待组，用于优雅关闭
	done        chan struct{}  // 关闭通知
	fallbackLog bool           // 队列满时是否降级为同步处理
}

// logEntry 表示一个日志条目
type logEntry struct {
	level   logrus.Level
	message *LoggerMessage
	err     error // 仅用于 Error 级别
}

// NewAsyncLogger 创建异步日志处理器
func NewAsyncLogger(logger ILogger, bufferSize int) *AsyncLogger {
	if bufferSize <= 0 {
		bufferSize = 1000 // 默认缓冲区大小
	}

	async := &AsyncLogger{
		logger:      logger,
		logCh:       make(chan *logEntry, bufferSize),
		done:        make(chan struct{}),
		fallbackLog: true, // 默认启用降级
	}

	// 启动日志处理协程
	go async.processLogs()

	return async
}

// processLogs 处理日志队列
func (a *AsyncLogger) processLogs() {
	for {
		select {
		case entry := <-a.logCh:
			// 调用内部logger处理日志
			a.processLogEntry(entry)
			a.wg.Done()
		case <-a.done:
			// 处理剩余日志
			for {
				select {
				case entry := <-a.logCh:
					a.processLogEntry(entry)
					a.wg.Done()
				default:
					return
				}
			}
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

// Close 关闭异步日志处理器
func (a *AsyncLogger) Close() {
	// 先通知关闭
	close(a.done)
	// 等待所有日志处理完成
	a.wg.Wait()
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

// enqueueOrFallback 将日志加入队列或降级为同步处理
func (a *AsyncLogger) enqueueOrFallback(level logrus.Level, message *LoggerMessage, err error) {
	entry := &logEntry{
		level:   level,
		message: message,
		err:     err,
	}

	a.wg.Add(1)

	// 尝试异步处理
	select {
	case a.logCh <- entry:
		// 成功加入队列
	default:
		// 队列已满
		a.wg.Done()

		// 如果启用了降级，则降级为同步处理
		if a.fallbackLog {
			a.processLogEntry(entry)
		}
	}
}
