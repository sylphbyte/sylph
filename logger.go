package sylph

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// LoggerFormatMessage 日志格式化消息结构体
// 用于标准化日志输出格式
type LoggerFormatMessage struct {
	Header *Header                `json:"header"`          // 请求头信息
	Info   any                    `json:"info,omitempty"`  // 信息数据
	Error  string                 `json:"error,omitempty"` // 错误信息
	Stack  string                 `json:"stack,omitempty"` // 堆栈信息
	Extra  map[string]interface{} `json:"extra,omitempty"` // 扩展字段
}

// LoggerMessage 日志消息结构体
// 包含日志的全部上下文信息
type LoggerMessage struct {
	Header   *Header `json:"header"`          // 请求头信息
	Location string  `json:"-"`               // 代码位置，不序列化到JSON
	Message  string  `json:"-"`               // 日志消息，不序列化到JSON
	Data     any     `json:"data,omitempty"`  // 数据内容
	Error    string  `json:"error,omitempty"` // 错误信息
	Stack    string  `json:"stack,omitempty"` // 堆栈信息
}

// MakeLoggerFormatMessage 将LoggerMessage转换为LoggerFormatMessage
// 用于统一日志输出格式
func (m *LoggerMessage) MakeLoggerFormatMessage() (formatMessage *LoggerFormatMessage) {
	return &LoggerFormatMessage{
		Header: m.Header,
		Info:   m.Data,
		Error:  m.Error,
		Stack:  m.Stack,
	}
}

// Fields 返回logrus字段格式的日志数据
func (m *LoggerMessage) Fields() logrus.Fields {
	return logrus.Fields{loggerMessageKey: m}
}

// ILogger 日志记录器接口
// 定义了各种日志级别的记录方法
type ILogger interface {
	Info(message *LoggerMessage)             // 记录信息级别日志
	Trace(message *LoggerMessage)            // 记录跟踪级别日志
	Debug(message *LoggerMessage)            // 记录调试级别日志
	Warn(message *LoggerMessage)             // 记录警告级别日志
	Fatal(message *LoggerMessage)            // 记录致命级别日志
	Panic(message *LoggerMessage)            // 记录恐慌级别日志
	Error(message *LoggerMessage, err error) // 记录错误级别日志
}

// NewLogger 创建新的日志记录器
// 参数:
//   - name: 日志记录器名称
//   - opt: 日志配置选项
//
// 返回:
//   - *Logger: 日志记录器实例
func NewLogger(name string, opt *LoggerConfig) *Logger {
	return &Logger{
		entry: NewLoggerBuilder(name, opt).Make(),
		opt:   opt,
	}
}

// DefaultLogger 创建带默认配置的日志记录器
// 参数:
//   - name: 日志记录器名称
//
// 返回:
//   - *Logger: 日志记录器实例
func DefaultLogger(name string) *Logger {
	return NewLogger(name, defaultLoggerConfig)
}

// Logger 日志记录器实现
type Logger struct {
	entry *logrus.Logger // logrus日志实例
	opt   *LoggerConfig  // 日志配置选项
}

// recover 恢复函数，用于捕获并处理日志系统中的panic
func (l Logger) recover() {
	if r := recover(); r != nil {
		l.Error(&LoggerMessage{
			Message: "log failed",
			Error:   fmt.Sprintf("%+v", r),
			Stack:   takeStack(),
		}, nil)
	}
}

// asyncLog 异步记录日志
// 在单独的goroutine中执行日志记录
func (l Logger) asyncLog(level logrus.Level, message *LoggerMessage) {
	defer l.recover()
	l.entry.WithFields(message.Fields()).Log(level)
}

// syncLog 同步记录日志
// 在当前goroutine中执行日志记录
func (l Logger) syncLog(level logrus.Level, message *LoggerMessage) {
	l.entry.WithFields(message.Fields()).Log(level)
}

// Log 根据配置选择同步或异步方式记录日志
// 参数:
//   - level: 日志级别
//   - message: 日志消息
func (l Logger) Log(level logrus.Level, message *LoggerMessage) {
	if l.opt.Async {
		go l.asyncLog(level, message)
		return
	}

	l.syncLog(level, message)
}

// Info 记录信息级别日志
func (l Logger) Info(message *LoggerMessage) {
	l.Log(logrus.InfoLevel, message)
}

// Trace 记录跟踪级别日志
func (l Logger) Trace(message *LoggerMessage) {
	l.Log(logrus.TraceLevel, message)
}

// Debug 记录调试级别日志
func (l Logger) Debug(message *LoggerMessage) {
	l.Log(logrus.DebugLevel, message)
}

// Warn 记录警告级别日志
func (l Logger) Warn(message *LoggerMessage) {
	l.Log(logrus.WarnLevel, message)
}

// Fatal 记录致命级别日志
func (l Logger) Fatal(message *LoggerMessage) {
	l.Log(logrus.FatalLevel, message)
}

// Panic 记录恐慌级别日志
func (l Logger) Panic(message *LoggerMessage) {
	l.Log(logrus.PanicLevel, message)
}

// Error 记录错误级别日志
// 参数:
//   - message: 日志消息
//   - err: 错误对象，如果不为nil，会将错误信息和堆栈添加到日志中
func (l Logger) Error(message *LoggerMessage, err error) {
	if err != nil {
		message.Error = err.Error()
		message.Stack = takeStack()
	}

	l.Log(logrus.ErrorLevel, message)
}
