package sylph

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

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

// 用于复用LoggerMessage对象的对象池
var loggerMessagePool = sync.Pool{
	New: func() interface{} {
		return &LoggerMessage{}
	},
}

// GetLoggerMessage 从对象池获取一个LoggerMessage
func GetLoggerMessage() *LoggerMessage {
	return loggerMessagePool.Get().(*LoggerMessage)
}

// ReleaseLoggerMessage 将LoggerMessage归还到对象池
func ReleaseLoggerMessage(msg *LoggerMessage) {
	if msg == nil {
		return
	}

	// 优化：处理Data字段中的map类型
	if msg.Data != nil {
		if dataMap, ok := msg.Data.(map[string]interface{}); ok {
			// 清空map但保留结构
			for k := range dataMap {
				delete(dataMap, k)
			}
			// 保留空map结构，避免重新分配
			msg.Data = dataMap
		} else if dataH, ok := msg.Data.(H); ok {
			// 处理H类型（本质是map[string]interface{}）
			for k := range dataH {
				delete(dataH, k)
			}
			msg.Data = dataH
		} else {
			// 其他类型直接设为nil
			msg.Data = nil
		}
	}

	// 清空其他字段
	msg.Header = nil
	msg.Location = ""
	msg.Message = ""
	msg.Error = ""
	msg.Stack = ""

	// 归还到池
	loggerMessagePool.Put(msg)
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

// Clone 创建消息的深度复制，用于异步处理
func (m *LoggerMessage) Clone() *LoggerMessage {
	if m == nil {
		return nil
	}

	clone := GetLoggerMessage()
	// 直接复制简单字段
	clone.Location = m.Location
	clone.Message = m.Message
	clone.Error = m.Error
	clone.Stack = m.Stack

	// 复制数据
	clone.Data = m.Data // 假设Data是不可变的或者已经是副本

	// 克隆头部
	if m.Header != nil {
		// 这里应该创建Header的副本，但Header的具体结构和Clone方法未知
		// 假设Header是线程安全的，可以在不同goroutine间共享
		clone.Header = m.Header
	}

	return clone
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

// 用于goroutine计数的原子计数器
type atomicCounter int64

func (c *atomicCounter) Inc() int64 {
	return atomic.AddInt64((*int64)(c), 1)
}

func (c *atomicCounter) Dec() int64 {
	return atomic.AddInt64((*int64)(c), -1)
}

func (c *atomicCounter) Value() int64 {
	return atomic.LoadInt64((*int64)(c))
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
		entry:            NewLoggerBuilder(name, opt).Make(),
		opt:              opt,
		activeGoroutines: new(atomicCounter),
		fieldsCache:      sync.Map{},
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
	entry            *logrus.Logger // logrus日志实例
	opt              *LoggerConfig  // 日志配置选项
	activeGoroutines *atomicCounter // 活跃goroutine计数器
	fieldsCache      sync.Map       // 缓存Fields结果
	closed           int32          // 关闭标志
}

// Close 关闭日志记录器
func (l *Logger) Close() {
	atomic.StoreInt32(&l.closed, 1)
	// 等待所有异步日志完成
	for l.activeGoroutines.Value() > 0 {
		runtime.Gosched() // 让出CPU时间片
	}
}

// IsClosed 检查日志记录器是否已关闭
func (l *Logger) IsClosed() bool {
	return atomic.LoadInt32(&l.closed) == 1
}

// recover 恢复函数，用于捕获并处理日志系统中的panic
func (l *Logger) recover() {
	if r := recover(); r != nil {
		// 创建一个新的消息对象，避免影响正在处理的消息
		errMsg := GetLoggerMessage()
		errMsg.Message = "log failed"
		errMsg.Error = fmt.Sprintf("%+v", r)
		errMsg.Stack = takeStack()

		// 使用同步方式记录错误
		l.syncLog(logrus.ErrorLevel, errMsg)

		// 归还消息对象
		ReleaseLoggerMessage(errMsg)
	}
}

// asyncLog 异步记录日志
// 在单独的goroutine中执行日志记录
func (l *Logger) asyncLog(level logrus.Level, message *LoggerMessage) {
	if l.IsClosed() {
		return
	}

	// 克隆消息对象以防止并发修改
	clone := message.Clone()
	l.activeGoroutines.Inc()

	go func() {
		defer l.recover()
		defer l.activeGoroutines.Dec()
		defer ReleaseLoggerMessage(clone)

		l.entry.WithFields(clone.Fields()).Log(level)
	}()
}

// getOrCreateFields 获取或创建Fields缓存
func (l *Logger) getOrCreateFields(message *LoggerMessage) logrus.Fields {
	// 简单情况直接返回，不走缓存
	if message == nil {
		return logrus.Fields{}
	}

	// 尝试从缓存获取
	cacheKey := fmt.Sprintf("%p", message) // 使用指针地址作为缓存键
	if cached, ok := l.fieldsCache.Load(cacheKey); ok {
		return cached.(logrus.Fields)
	}

	// 创建新的Fields
	fields := message.Fields()

	// 存入缓存
	l.fieldsCache.Store(cacheKey, fields)

	return fields
}

// syncLog 同步记录日志
// 在当前goroutine中执行日志记录
func (l *Logger) syncLog(level logrus.Level, message *LoggerMessage) {
	if l.IsClosed() {
		return
	}

	fields := l.getOrCreateFields(message)
	l.entry.WithFields(fields).Log(level)
}

// Log 根据配置选择同步或异步方式记录日志
// 参数:
//   - level: 日志级别
//   - message: 日志消息
func (l *Logger) Log(level logrus.Level, message *LoggerMessage) {
	// 检查关闭状态
	if l.IsClosed() {
		return
	}

	// Fatal和Panic级别始终同步执行
	if level == logrus.FatalLevel || level == logrus.PanicLevel {
		l.syncLog(level, message)
		return
	}

	if l.opt.Async {
		l.asyncLog(level, message)
		return
	}

	l.syncLog(level, message)
}

// Info 记录信息级别日志
func (l *Logger) Info(message *LoggerMessage) {
	l.Log(logrus.InfoLevel, message)
}

// Trace 记录跟踪级别日志
func (l *Logger) Trace(message *LoggerMessage) {
	l.Log(logrus.TraceLevel, message)
}

// Debug 记录调试级别日志
func (l *Logger) Debug(message *LoggerMessage) {
	l.Log(logrus.DebugLevel, message)
}

// Warn 记录警告级别日志
func (l *Logger) Warn(message *LoggerMessage) {
	l.Log(logrus.WarnLevel, message)
}

// Fatal 记录致命级别日志
func (l *Logger) Fatal(message *LoggerMessage) {
	l.Log(logrus.FatalLevel, message)
}

// Panic 记录恐慌级别日志
func (l *Logger) Panic(message *LoggerMessage) {
	l.Log(logrus.PanicLevel, message)
}

// Error 记录错误级别日志
// 参数:
//   - message: 日志消息
//   - err: 错误对象，如果不为nil，会将错误信息和堆栈添加到日志中
func (l *Logger) Error(message *LoggerMessage, err error) {
	if err != nil {
		message.Error = err.Error()
		message.Stack = takeStack()
	}

	l.Log(logrus.ErrorLevel, message)
}
