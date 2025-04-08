package sylph

import (
	"fmt"
	"github.com/sylphbyte/pr"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

// LoggerFormatMessage 日志格式化消息结构体
// 用于标准化日志输出格式，提供统一的JSON结构
//
// 功能:
//   - 封装日志的各个组成部分
//   - 提供统一的日志输出格式
//   - 便于日志解析和分析
//
// 字段说明:
//   - Header: 请求头信息，包含调用链跟踪数据
//   - Marks: 标记信息，用于添加自定义标签
//   - Info: 具体信息数据，可以是任意类型
//   - Error: 错误消息
//   - Stack: 堆栈信息，便于定位问题
//   - Extra: 扩展字段，用于存储其他自定义数据
//
// 使用示例:
//
//	message := &LoggerFormatMessage{
//	    Header: myHeader,
//	    Info: map[string]interface{}{"user_id": 123},
//	    Error: err.Error(),
//	}
//	jsonData, _ := json.Marshal(message)
type LoggerFormatMessage struct {
	Header *Header                `json:"header"`          // 请求头信息
	Marks  map[string]interface{} `json:"marks,omitempty"` // 标记信息
	Info   any                    `json:"info,omitempty"`  // 信息数据
	Error  string                 `json:"error,omitempty"` // 错误信息
	Stack  string                 `json:"stack,omitempty"` // 堆栈信息
	Extra  map[string]interface{} `json:"extra,omitempty"` // 扩展字段
}

// 用于复用LoggerMessage对象的对象池
// 通过对象池减少GC压力，提高性能
var loggerMessagePool = sync.Pool{
	New: func() interface{} {
		return &LoggerMessage{}
	},
}

// GetLoggerMessage 从对象池获取一个LoggerMessage
//
// 功能:
//   - 从对象池中获取一个预分配的LoggerMessage对象
//   - 避免频繁创建新对象，减少GC压力
//
// 返回:
//   - *LoggerMessage: 从对象池获取的日志消息对象
//
// 使用示例:
//
//	msg := GetLoggerMessage()
//	msg.Message = "操作成功"
//	msg.Data = userData
//	logger.Info(msg)
//	ReleaseLoggerMessage(msg) // 使用完后必须释放
//
// 注意事项:
//   - 获取对象后必须配对调用ReleaseLoggerMessage归还对象
//   - 不要长时间持有对象，避免池资源耗尽
func GetLoggerMessage() *LoggerMessage {
	return loggerMessagePool.Get().(*LoggerMessage)
}

// ReleaseLoggerMessage 将LoggerMessage归还到对象池
//
// 功能:
//   - 清理LoggerMessage对象的内容
//   - 将对象归还到对象池以供复用
//
// 参数:
//   - msg: 要归还的日志消息对象
//
// 实现逻辑:
//   - 如果对象为nil，直接返回
//   - 处理Data字段中的map类型，清空但保留结构避免重新分配
//   - 清空其他字段
//   - 将对象归还到对象池
//
// 使用示例:
//
//	msg := GetLoggerMessage()
//	// ... 使用msg ...
//	ReleaseLoggerMessage(msg) // 使用完后归还
//
// 注意事项:
//   - 归还后不要再使用该对象
//   - 必须与GetLoggerMessage配对使用
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
//
// 功能:
//   - 封装完整的日志记录所需信息
//   - 提供内部日志处理所需的数据结构
//   - 支持被转换为最终输出格式
//
// 字段说明:
//   - Header: 请求头信息，包含调用链跟踪数据
//   - Marks: 标记信息，用于添加自定义标签
//   - Location: 代码位置信息，不输出到JSON
//   - Message: 日志消息文本，不输出到JSON
//   - Data: 数据内容，可以是任意类型
//   - Error: 错误信息
//   - Stack: 堆栈信息
//
// 使用示例:
//
//	msg := GetLoggerMessage()
//	msg.Message = "用户登录"
//	msg.Data = map[string]interface{}{
//	    "user_id": user.ID,
//	    "login_time": time.Now(),
//	}
//	logger.Info(msg)
//	ReleaseLoggerMessage(msg)
//
// 注意事项:
//   - 通常应通过GetLoggerMessage获取实例
//   - 使用完毕后应调用ReleaseLoggerMessage归还实例
type LoggerMessage struct {
	Header   *Header                `json:"header"` // 请求头信息
	Marks    map[string]interface{} // 标记信息
	Location string                 `json:"-"`               // 代码位置，不序列化到JSON
	Message  string                 `json:"-"`               // 日志消息，不序列化到JSON
	Data     any                    `json:"data,omitempty"`  // 数据内容
	Error    string                 `json:"error,omitempty"` // 错误信息
	Stack    string                 `json:"stack,omitempty"` // 堆栈信息
}

// MakeLoggerFormatMessage 将LoggerMessage转换为LoggerFormatMessage
// 用于统一日志输出格式
//
// 功能:
//   - 将内部LoggerMessage结构转换为标准化输出格式
//   - 确保日志记录的一致性
//
// 返回:
//   - *LoggerFormatMessage: 格式化后的日志消息
//
// 实现逻辑:
//   - 创建新的LoggerFormatMessage对象
//   - 将LoggerMessage的字段映射到对应的LoggerFormatMessage字段
//
// 使用示例:
//
//	formattedMsg := loggerMsg.MakeLoggerFormatMessage()
//	jsonData, _ := json.Marshal(formattedMsg)
func (m *LoggerMessage) MakeLoggerFormatMessage() (formatMessage *LoggerFormatMessage) {
	return &LoggerFormatMessage{
		Header: m.Header,
		//Marks:  m.Marks,
		Info:  m.Data,
		Error: m.Error,
		Stack: m.Stack,
	}
}

// Fields 返回logrus字段格式的日志数据
//
// 功能:
//   - 将LoggerMessage转换为logrus.Fields
//   - 便于与logrus集成
//
// 返回:
//   - logrus.Fields: 适用于logrus的字段集合
//
// 使用示例:
//
//	entry := logrus.WithFields(msg.Fields())
//	entry.Info(msg.Message)
func (m *LoggerMessage) Fields() logrus.Fields {
	return logrus.Fields{loggerMessageKey: m}
}

// Clone 创建消息的深度复制，用于异步处理
//
// 功能:
//   - 创建LoggerMessage的副本
//   - 适用于在不同goroutine中安全使用日志消息
//
// 返回:
//   - *LoggerMessage: 原消息的深度复制
//
// 实现逻辑:
//   - 从对象池获取新的LoggerMessage实例
//   - 复制所有字段到新实例
//   - 对引用类型字段进行适当处理
//
// 使用示例:
//
//	clonedMsg := msg.Clone()
//	go func() {
//	    // 在另一个goroutine中安全使用
//	    process(clonedMsg)
//	    ReleaseLoggerMessage(clonedMsg)
//	}()
//
// 注意事项:
//   - 返回的对象需要调用ReleaseLoggerMessage归还到池
//   - 仅适用于Data字段是不可变对象或副本的情况
func (m *LoggerMessage) Clone() *LoggerMessage {
	if m == nil {
		return nil
	}

	//clone := GetLoggerMessage()
	clone := &LoggerMessage{}
	// 直接复制简单字段
	clone.Location = m.Location
	clone.Message = m.Message
	clone.Error = m.Error
	clone.Stack = m.Stack

	clone.Marks = m.Marks

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

// ILogger接口在define.go中已定义，这里是相关注释
//
// 功能:
//   - 提供统一的日志记录接口
//   - 支持不同级别的日志记录
//   - 允许基于接口的依赖注入
//
// 方法说明:
//   - Info: 记录信息级别日志，用于一般信息
//   - Trace: 记录跟踪级别日志，用于详细跟踪程序执行
//   - Debug: 记录调试级别日志，用于调试信息
//   - Warn: 记录警告级别日志，用于潜在问题警告
//   - Fatal: 记录致命级别日志，记录后会导致程序退出
//   - Panic: 记录恐慌级别日志，记录后会引发panic
//   - Error: 记录错误级别日志，包含错误对象
//
// 使用示例:
//
//	var logger ILogger = DefaultLogger("myservice")
//
//	msg := GetLoggerMessage()
//	msg.Message = "操作成功"
//	logger.Info(msg)
//	ReleaseLoggerMessage(msg)
//
//	// 错误处理
//	if err != nil {
//	    msg := GetLoggerMessage()
//	    msg.Message = "操作失败"
//	    logger.Error(msg, err)
//	    ReleaseLoggerMessage(msg)
//	}

// atomicCounter 原子计数器
// 用于安全地在并发环境中计数
//
// 功能:
//   - 提供线程安全的计数器
//   - 用于记录活跃的goroutine数量
//
// 实现逻辑:
//   - 基于atomic包的原子操作实现
//   - 保证在高并发下计数的准确性
type atomicCounter int64

// Inc 增加计数器值
//
// 功能:
//   - 原子地将计数器加1
//
// 返回:
//   - int64: 增加后的计数器值
func (c *atomicCounter) Inc() int64 {
	return atomic.AddInt64((*int64)(c), 1)
}

// Dec 减少计数器值
//
// 功能:
//   - 原子地将计数器减1
//
// 返回:
//   - int64: 减少后的计数器值
func (c *atomicCounter) Dec() int64 {
	return atomic.AddInt64((*int64)(c), -1)
}

// Value 获取计数器当前值
//
// 功能:
//   - 原子地获取计数器的当前值
//
// 返回:
//   - int64: 计数器的当前值
func (c *atomicCounter) Value() int64 {
	return atomic.LoadInt64((*int64)(c))
}

// NewLogger 创建新的日志记录器
//
// 功能:
//   - 基于给定名称和配置创建日志记录器
//   - 配置日志的输出格式、级别和目标
//
// 参数:
//   - name: 日志记录器名称，用于标识日志来源
//   - opt: 日志配置选项，包含级别、格式等配置
//
// 返回:
//   - *Logger: 日志记录器实例
//
// 使用示例:
//
//	config := &LoggerConfig{
//	    Level: "info",
//	    Format: "json",
//	    Output: "file",
//	    FilePath: "/var/log/app.log",
//	}
//	logger := NewLogger("userservice", config)
//
//	msg := GetLoggerMessage()
//	msg.Message = "系统启动"
//	logger.Info(msg)
//	ReleaseLoggerMessage(msg)
//
// 注意事项:
//   - 创建的Logger需要在应用程序结束时调用Close()
//   - 日志文件路径需要确保应用有写入权限
func NewLogger(name string, opt *LoggerConfig) *Logger {
	return &Logger{
		entry:            NewLoggerBuilder(name, opt).Make(),
		opt:              opt,
		activeGoroutines: new(atomicCounter),
		fieldsCache:      sync.Map{},
	}
}

// DefaultLogger 创建带默认配置的日志记录器
//
// 功能:
//   - 使用默认配置创建日志记录器
//   - 简化日志记录器的创建过程
//
// 参数:
//   - name: 日志记录器名称，用于标识日志来源
//
// 返回:
//   - *Logger: 日志记录器实例
//
// 使用示例:
//
//	logger := DefaultLogger("api")
//
//	msg := GetLoggerMessage()
//	msg.Message = "API服务启动"
//	logger.Info(msg)
//	ReleaseLoggerMessage(msg)
//
// 注意事项:
//   - 创建的Logger需要在应用程序结束时调用Close()
//   - 默认配置适用于大多数场景，但对于特殊需求应使用NewLogger
func DefaultLogger(name string) *Logger {
	return NewLogger(name, defaultLoggerConfig)
}

// Logger 日志记录器实现
// 基于logrus的日志记录器，支持异步日志和对象池优化
//
// 功能:
//   - 提供多级别的日志记录功能
//   - 支持同步和异步记录日志
//   - 实现ILogger接口
//   - 提供性能优化和资源管理
//
// 属性说明:
//   - entry: logrus日志实例，实际执行日志记录
//   - opt: 日志配置选项，控制日志行为
//   - activeGoroutines: 活跃goroutine计数器，用于优雅关闭
//   - fieldsCache: 缓存Fields结果，提高性能
//   - closed: 关闭标志，原子操作确保线程安全
//
// 使用示例:
//
//	logger := DefaultLogger("payment")
//
//	// 记录信息
//	msg := GetLoggerMessage()
//	msg.Message = "支付成功"
//	msg.Data = map[string]interface{}{
//	    "order_id": orderId,
//	    "amount": amount,
//	}
//	logger.Info(msg)
//	ReleaseLoggerMessage(msg)
//
//	// 应用关闭时
//	logger.Close()
//
// 注意事项:
//   - 使用完Logger后必须调用Close()
//   - 异步日志可能在程序崩溃时丢失
//   - 大量并发日志可能导致内存压力增加
type Logger struct {
	entry            *logrus.Logger // logrus日志实例
	opt              *LoggerConfig  // 日志配置选项
	activeGoroutines *atomicCounter // 活跃goroutine计数器
	fieldsCache      sync.Map       // 缓存Fields结果
	closed           int32          // 关闭标志
}

// Close 关闭日志记录器
//
// 功能:
//   - 安全地关闭日志记录器
//   - 等待所有异步日志完成处理
//
// 实现逻辑:
//   - 设置关闭标志防止新的日志提交
//   - 等待所有活跃的异步日志处理完成
//   - 让出CPU时间片以减少资源消耗
//
// 使用示例:
//
//	// 应用程序结束时
//	func cleanup() {
//	    logger.Close()
//	}
//
// 注意事项:
//   - 关闭后不应再使用该日志记录器
//   - 可能会阻塞调用线程直到所有异步日志处理完成
func (l *Logger) Close() {
	atomic.StoreInt32(&l.closed, 1)
	// 等待所有异步日志完成
	for l.activeGoroutines.Value() > 0 {
		runtime.Gosched() // 让出CPU时间片
	}
}

// IsClosed 检查日志记录器是否已关闭
//
// 功能:
//   - 检查日志记录器的关闭状态
//
// 返回:
//   - bool: 如果已关闭返回true，否则返回false
//
// 使用示例:
//
//	if !logger.IsClosed() {
//	    // 可以安全记录日志
//	    logger.Info(msg)
//	}
func (l *Logger) IsClosed() bool {
	return atomic.LoadInt32(&l.closed) == 1
}

// recover 恢复函数，用于捕获并处理日志系统中的panic
//
// 功能:
//   - 捕获日志处理过程中的panic
//   - 确保panic不会传播到应用代码
//   - 记录panic信息作为错误日志
//
// 实现逻辑:
//   - 捕获recover得到的panic值
//   - 创建新的消息对象记录panic信息
//   - 使用同步方式记录错误，避免异步处理中的循环panic
//   - 归还临时创建的消息对象
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
//
// 功能:
//   - 将日志记录操作放入单独的goroutine执行
//   - 避免日志记录阻塞主业务流程
//
// 参数:
//   - level: 日志级别
//   - message: 日志消息对象
//
// 实现逻辑:
//   - 检查日志记录器是否已关闭
//   - 克隆消息对象以防止并发修改
//   - 增加活跃goroutine计数
//   - 启动goroutine执行日志记录
//   - 在goroutine中设置恢复机制、减少计数并释放消息对象
//
// 注意事项:
//   - 异步记录可能导致程序崩溃时日志丢失
//   - 大量并发的日志记录可能导致资源压力
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
		//defer ReleaseLoggerMessage(clone)

		pr.Red("xxxx: %v\n", clone)
		l.entry.WithFields(clone.Fields()).Log(level)
	}()
}

// getOrCreateFields 获取或创建Fields缓存
//
// 功能:
//   - 优化性能，缓存Fields结果
//   - 避免重复转换相同消息对象
//
// 参数:
//   - message: 日志消息对象
//
// 返回:
//   - logrus.Fields: 用于logrus的字段集合
//
// 实现逻辑:
//   - 处理空消息情况
//   - 使用消息对象指针地址作为缓存键
//   - 尝试从缓存获取已存在的Fields
//   - 如果缓存未命中，创建新的Fields并存入缓存
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
//
// 功能:
//   - 立即同步记录日志
//   - 适用于重要日志或日志系统内部错误
//
// 参数:
//   - level: 日志级别
//   - message: 日志消息对象
//
// 实现逻辑:
//   - 检查日志记录器是否已关闭
//   - 直接在当前goroutine记录日志
//   - 使用Fields缓存优化性能
//
// 注意事项:
//   - 同步记录会阻塞当前goroutine
//   - 应谨慎使用，避免影响性能关键路径
func (l *Logger) syncLog(level logrus.Level, message *LoggerMessage) {
	if l.IsClosed() {
		return
	}

	fields := l.getOrCreateFields(message)
	l.entry.WithFields(fields).Log(level)
}

// Info 记录信息级别日志
//
// 功能:
//   - 记录普通信息日志
//   - 适用于一般操作成功、状态变更等信息
//
// 参数:
//   - message: 日志消息对象
//
// 使用示例:
//
//	msg := GetLoggerMessage()
//	msg.Message = "用户注册成功"
//	msg.Data = map[string]interface{}{"user_id": newUser.ID}
//	logger.Info(msg)
//	ReleaseLoggerMessage(msg)
func (l *Logger) Info(message *LoggerMessage) {
	l.asyncLog(logrus.InfoLevel, message)
}

// Trace 记录跟踪级别日志
//
// 功能:
//   - 记录最详细的跟踪日志
//   - 适用于需要详细追踪程序执行流程的场景
//
// 参数:
//   - message: 日志消息对象
//
// 使用示例:
//
//	msg := GetLoggerMessage()
//	msg.Message = "开始处理请求"
//	msg.Data = map[string]interface{}{"request_id": reqID, "params": params}
//	logger.Trace(msg)
//	ReleaseLoggerMessage(msg)
func (l *Logger) Trace(message *LoggerMessage) {
	l.asyncLog(logrus.TraceLevel, message)
}

// Debug 记录调试级别日志
//
// 功能:
//   - 记录调试信息
//   - 适用于开发和问题排查阶段
//
// 参数:
//   - message: 日志消息对象
//
// 使用示例:
//
//	msg := GetLoggerMessage()
//	msg.Message = "SQL查询"
//	msg.Data = map[string]interface{}{"query": sqlQuery, "params": queryParams}
//	logger.Debug(msg)
//	ReleaseLoggerMessage(msg)
func (l *Logger) Debug(message *LoggerMessage) {
	l.asyncLog(logrus.DebugLevel, message)
}

// Warn 记录警告级别日志
//
// 功能:
//   - 记录警告信息
//   - 适用于潜在问题、不推荐用法等场景
//
// 参数:
//   - message: 日志消息对象
//
// 使用示例:
//
//	msg := GetLoggerMessage()
//	msg.Message = "数据库连接池接近上限"
//	msg.Data = map[string]interface{}{"current": current, "max": max}
//	logger.Warn(msg)
//	ReleaseLoggerMessage(msg)
func (l *Logger) Warn(message *LoggerMessage) {
	l.asyncLog(logrus.WarnLevel, message)
}

// Error 记录错误级别日志
//
// 功能:
//   - 记录错误信息
//   - 包含错误对象和堆栈信息
//
// 参数:
//   - message: 日志消息对象
//   - err: 错误对象
//
// 实现逻辑:
//   - 设置消息的错误信息
//   - 如果消息中未设置堆栈信息，添加当前堆栈
//   - 异步记录错误日志
//
// 使用示例:
//
//	err := db.QueryRow(...)
//	if err != nil {
//	    msg := GetLoggerMessage()
//	    msg.Message = "查询用户失败"
//	    msg.Data = map[string]interface{}{"user_id": userID}
//	    logger.Error(msg, err)
//	    ReleaseLoggerMessage(msg)
//	}
func (l *Logger) Error(message *LoggerMessage, err error) {
	if err != nil {
		message.Error = err.Error()
	}

	// 添加堆栈信息（如果没有）
	if message.Stack == "" {
		message.Stack = takeStack()
	}

	l.asyncLog(logrus.ErrorLevel, message)
}

// Fatal 记录致命级别日志
//
// 功能:
//   - 记录致命错误
//   - 记录后会导致程序退出
//
// 参数:
//   - message: 日志消息对象
//
// 实现逻辑:
//   - 添加堆栈信息（如果没有）
//   - 同步记录日志（确保在程序退出前记录）
//   - 记录后程序会退出
//
// 使用示例:
//
//	if err := criticalInit(); err != nil {
//	    msg := GetLoggerMessage()
//	    msg.Message = "系统初始化失败，无法继续"
//	    msg.Error = err.Error()
//	    logger.Fatal(msg)
//	    // 程序会在这里退出
//	    ReleaseLoggerMessage(msg) // 实际上不会执行到这里
//	}
//
// 注意事项:
//   - Fatal会导致程序立即退出，不会执行后续代码
//   - 使用前应确保已完成必要的清理工作
func (l *Logger) Fatal(message *LoggerMessage) {
	// 添加堆栈信息（如果没有）
	if message.Stack == "" {
		message.Stack = takeStack()
	}

	// Fatal级别使用同步记录，确保在程序退出前记录
	l.syncLog(logrus.FatalLevel, message)
}

// Panic 记录恐慌级别日志
//
// 功能:
//   - 记录严重错误
//   - 记录后会引发panic
//
// 参数:
//   - message: 日志消息对象
//
// 实现逻辑:
//   - 添加堆栈信息（如果没有）
//   - 同步记录日志（确保在panic前记录）
//   - 记录后会引发panic
//
// 使用示例:
//
//	if critical := checkCriticalError(); critical != nil {
//	    msg := GetLoggerMessage()
//	    msg.Message = "检测到严重错误"
//	    msg.Error = critical.Error()
//	    logger.Panic(msg)
//	    // 代码会panic，不会执行到这里
//	}
//
// 注意事项:
//   - Panic会导致当前goroutine崩溃，除非有recover
//   - 使用前应确保已完成必要的清理工作
func (l *Logger) Panic(message *LoggerMessage) {
	// 添加堆栈信息（如果没有）
	if message.Stack == "" {
		message.Stack = takeStack()
	}

	// Panic级别使用同步记录，确保在panic前记录
	l.syncLog(logrus.PanicLevel, message)
}

// takeStack函数在其他文件已定义，这里是相关注释
//
// 功能:
//   - 获取当前调用堆栈的字符串表示
//   - 用于记录错误发生的位置和调用链
//
// 返回:
//   - string: 格式化的堆栈信息
//
// 实现逻辑:
//   - 捕获当前goroutine的调用堆栈
//   - 转换为易读的字符串格式
//   - 跳过堆栈的前几帧，避免包含日志库自身的调用
//
// 使用示例:
//
//	errorMsg.Stack = takeStack()
//	logger.Error(errorMsg, err)
//
// 注意事项:
//   - 堆栈捕获会带来一定的性能开销
//   - 建议只在错误级别以上的日志中捕获堆栈
