package sylph

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

// LoggerMessage 日志消息结构体
// 简化设计，合并了之前的LoggerMessage和LoggerFormatMessage
// 直接使用这一个消息类型处理所有日志场景
type LoggerMessage struct {
	Header   *Header                `json:"header,omitempty"` // 请求头信息
	Marks    map[string]interface{} `json:"-"`                // 标记信息
	Location string                 `json:"-"`                // 代码位置
	Message  string                 `json:"-"`                // 日志消息
	Data     any                    `json:"data,omitempty"`   // 数据内容
	Error    string                 `json:"error,omitempty"`  // 错误信息
	Stack    string                 `json:"stack,omitempty"`  // 堆栈信息
	Extra    map[string]interface{} `json:"extra,omitempty"`  // 扩展字段
}

// NewLoggerMessage 创建新的日志消息对象
// 简化的创建函数，不使用对象池
func NewLoggerMessage() *LoggerMessage {
	return &LoggerMessage{}
}

// WithField 添加单个数据字段
// 链式调用API，使用更便捷
func (m *LoggerMessage) WithField(key string, value interface{}) *LoggerMessage {
	if m.Data == nil {
		m.Data = make(map[string]interface{})
	}

	if data, ok := m.Data.(map[string]interface{}); ok {
		data[key] = value
	} else {
		// 如果当前Data不是map，创建新map并保留原值
		newData := map[string]interface{}{
			"_previous": m.Data,
			key:         value,
		}
		m.Data = newData
	}

	return m
}

// WithFields 批量添加数据字段
func (m *LoggerMessage) WithFields(fields map[string]interface{}) *LoggerMessage {
	if fields == nil {
		return m
	}

	if m.Data == nil {
		m.Data = fields
		return m
	}

	if data, ok := m.Data.(map[string]interface{}); ok {
		for k, v := range fields {
			data[k] = v
		}
	} else {
		// 如果当前Data不是map，创建新map并保留原值
		newData := map[string]interface{}{"_previous": m.Data}
		for k, v := range fields {
			newData[k] = v
		}
		m.Data = newData
	}

	return m
}

// WithError 添加错误信息
func (m *LoggerMessage) WithError(err error) *LoggerMessage {
	if err != nil {
		m.Error = err.Error()
	}
	return m
}

// WithLocation 设置代码位置
func (m *LoggerMessage) WithLocation(location string) *LoggerMessage {
	m.Location = location
	return m
}

// WithHeader 设置请求头
func (m *LoggerMessage) WithHeader(header *Header) *LoggerMessage {
	m.Header = header
	return m
}

// WithStack 设置堆栈信息
func (m *LoggerMessage) WithStack(stack string) *LoggerMessage {
	m.Stack = stack
	return m
}

// ToLogrusFields 将消息转换为logrus字段
// 内部使用，用于与logrus集成
func (m *LoggerMessage) ToLogrusFields() logrus.Fields {
	return logrus.Fields{"_message": m}
}

// Logger 日志记录器实现
// 基于logrus的日志记录器，支持同步和异步日志
type Logger struct {
	entry  *logrus.Logger     // logrus日志实例
	opt    *LoggerConfig      // 日志配置选项
	ctx    context.Context    // 上下文，用于控制生命周期
	cancel context.CancelFunc // 取消函数
	wg     sync.WaitGroup     // 等待异步日志完成
	closed int32              // 关闭标志，确保线程安全
	name   string             // 日志记录器名称
}

// NewLogger 创建新的日志记录器
func NewLogger(name string, opt *LoggerConfig) *Logger {
	ctx, cancel := context.WithCancel(context.Background())

	return &Logger{
		entry:  NewLoggerBuilder(name, opt).Make(),
		opt:    opt,
		ctx:    ctx,
		cancel: cancel,
		closed: 0,
		name:   name,
	}
}

// DefaultLogger 创建带默认配置的日志记录器
func DefaultLogger(name string) *Logger {
	return NewLogger(name, defaultLoggerConfig)
}

// WithContext 设置上下文
// 允许通过上下文控制日志记录器的生命周期
func (l *Logger) WithContext(ctx context.Context) ILogger {
	if ctx == nil {
		return l
	}

	childCtx, cancel := context.WithCancel(ctx)

	// 创建新实例而不是修改当前实例
	return &Logger{
		entry:  l.entry,
		opt:    l.opt,
		ctx:    childCtx,
		cancel: cancel,
		closed: l.closed,
		name:   l.name,
	}
}

// Close 关闭日志记录器
// 优雅关闭，等待所有异步日志写入完成
func (l *Logger) Close() error {
	// 已经关闭则直接返回
	if !atomic.CompareAndSwapInt32(&l.closed, 0, 1) {
		return nil
	}

	// 通知异步工作停止
	l.cancel()

	// 等待所有异步日志写入完成
	l.wg.Wait()

	// 关闭可能的底层资源
	if l.entry != nil && l.entry.Out != nil {
		if closer, ok := l.entry.Out.(interface{ Close() error }); ok {
			return closer.Close()
		}
	}

	return nil
}

// IsClosed 检查日志记录器是否已关闭
func (l *Logger) IsClosed() bool {
	return atomic.LoadInt32(&l.closed) == 1
}

// logMessage 通用日志记录处理
// 根据配置选择同步或异步方式记录日志
func (l *Logger) logMessage(level logrus.Level, message *LoggerMessage) {
	// 已关闭则不处理
	if l.IsClosed() {
		return
	}

	// 根据配置选择同步或异步方式
	if l.opt.Async {
		l.asyncLog(level, message)
	} else {
		l.syncLog(level, message)
	}
}

// asyncLog 异步记录日志
// 启动goroutine处理日志，不阻塞调用方
func (l *Logger) asyncLog(level logrus.Level, message *LoggerMessage) {
	// 原子操作递增等待计数
	l.wg.Add(1)

	// 在goroutine中处理日志
	go func(msg *LoggerMessage) {
		defer l.wg.Done()

		select {
		case <-l.ctx.Done():
			// 上下文已取消，记录可能的错误但不阻止处理
			fmt.Printf("Warning: Logging after shutdown, message: %s\n", msg.Message)
		default:
			// 正常处理
			l.syncLog(level, msg)
		}
	}(message)
}

// syncLog 同步记录日志
// 直接处理日志消息
func (l *Logger) syncLog(level logrus.Level, message *LoggerMessage) {
	if l.entry == nil {
		return
	}

	// 创建日志条目
	entry := l.entry.WithFields(message.ToLogrusFields())

	// 记录日志
	switch level {
	case logrus.InfoLevel:
		entry.Info(message.Message)
	case logrus.TraceLevel:
		entry.Trace(message.Message)
	case logrus.DebugLevel:
		entry.Debug(message.Message)
	case logrus.WarnLevel:
		entry.Warn(message.Message)
	case logrus.ErrorLevel:
		entry.Error(message.Message)
	case logrus.FatalLevel:
		entry.Fatal(message.Message)
	case logrus.PanicLevel:
		entry.Panic(message.Message)
	}
}

// Info 记录信息级别日志
func (l *Logger) Info(message *LoggerMessage) {
	l.logMessage(logrus.InfoLevel, message)
}

// Trace 记录跟踪级别日志
func (l *Logger) Trace(message *LoggerMessage) {
	l.logMessage(logrus.TraceLevel, message)
}

// Debug 记录调试级别日志
func (l *Logger) Debug(message *LoggerMessage) {
	l.logMessage(logrus.DebugLevel, message)
}

// Warn 记录警告级别日志
func (l *Logger) Warn(message *LoggerMessage) {
	l.logMessage(logrus.WarnLevel, message)
}

// Error 记录错误级别日志
func (l *Logger) Error(message *LoggerMessage, err error) {
	if err != nil && message.Error == "" {
		message.Error = err.Error()
	}
	l.logMessage(logrus.ErrorLevel, message)
}

// Fatal 记录致命级别日志
func (l *Logger) Fatal(message *LoggerMessage) {
	l.logMessage(logrus.FatalLevel, message)
}

// Panic 记录恐慌级别日志
func (l *Logger) Panic(message *LoggerMessage) {
	l.logMessage(logrus.PanicLevel, message)
}

// 以下是简化API方法，支持直接使用格式化字符串

// Infof 记录信息级别的格式化日志
func (l *Logger) Infof(format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	l.Info(msg)
}

// Tracef 记录跟踪级别的格式化日志
func (l *Logger) Tracef(format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	l.Trace(msg)
}

// Debugf 记录调试级别的格式化日志
func (l *Logger) Debugf(format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	l.Debug(msg)
}

// Warnf 记录警告级别的格式化日志
func (l *Logger) Warnf(format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	l.Warn(msg)
}

// Errorf 记录错误级别的格式化日志
func (l *Logger) Errorf(err error, msgFormat string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(msgFormat, args...)
	l.Error(msg, err)
}

// Fatalf 记录致命级别的格式化日志
func (l *Logger) Fatalf(format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	l.Fatal(msg)
}

// Panicf 记录恐慌级别的格式化日志
func (l *Logger) Panicf(format string, args ...interface{}) {
	msg := NewLoggerMessage()
	msg.Message = fmt.Sprintf(format, args...)
	l.Panic(msg)
}
