package sylph

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/pkg/errors"
)

// LogLevel 定义日志级别类型
// 表示系统中不同的日志严重程度，用于控制日志输出
type LogLevel int

// 日志级别常量定义
// 从调试级别到恐慌级别，严重程度依次增加
const (
	DebugLevel LogLevel = iota // 调试级别，详细的开发调试信息
	InfoLevel                  // 信息级别，常规操作信息
	TraceLevel                 // 跟踪级别，详细的调用链跟踪
	WarnLevel                  // 警告级别，潜在问题的提示
	ErrorLevel                 // 错误级别，运行时错误
	FatalLevel                 // 致命级别，导致应用程序终止的错误
	PanicLevel                 // 恐慌级别，导致恐慌的严重错误
)

const (
	jwtClaimKey = "x:jwt:claim" // JWT声明的存储键名，用于在上下文中存取JWT信息
)

// IJwtClaim 定义JWT声明接口
// 提供获取用户ID、令牌、颁发者及验证颁发者的方法
//
// 使用示例:
//
//	claim := ctx.JwtClaim()
//	if claim != nil && claim.IssuerIs("auth-service") {
//	    userId := claim.TakeId()
//	    // 处理认证用户的逻辑
//	}
type IJwtClaim interface {
	TakeId() string            // 获取用户ID
	TakeToken() string         // 获取令牌字符串
	TakeIssuer() string        // 获取颁发者名称
	IssuerIs(name string) bool // 验证颁发者是否匹配
}

// LogContext 日志上下文接口
// 定义了各种日志级别的记录方法，用于在不同场景下记录系统运行信息
//
// 使用示例:
//
//	func HandleRequest(logCtx LogContext) {
//	    logCtx.Info("HandleRequest", "开始处理请求", nil)
//	    // 处理逻辑
//	    if err != nil {
//	        logCtx.Error("HandleRequest", "处理请求失败", err, map[string]interface{}{"request_id": reqId})
//	    }
//	}
type LogContext interface {
	Info(location, msg string, data any)                 // 记录信息级别日志
	Trace(location, msg string, data any)                // 记录跟踪级别日志
	Debug(location, msg string, data any)                // 记录调试级别日志
	Warn(location, msg string, data any)                 // 记录警告级别日志
	Fatal(location, msg string, data any)                // 记录致命级别日志，可能导致程序退出
	Panic(location, msg string, data any)                // 记录恐慌级别日志，可能导致程序恐慌
	Error(location, message string, err error, data any) // 记录错误级别日志，包含错误对象
}

// DataContext 数据上下文接口
// 提供键值存储功能，用于在请求生命周期内存储和检索数据
//
// 使用示例:
//
//	func ProcessRequest(dataCtx DataContext) {
//	    // 存储数据
//	    dataCtx.Set("user_id", "12345")
//
//	    // 检索数据
//	    if userId, ok := dataCtx.Get("user_id"); ok {
//	        // 使用userId进行操作
//	    }
//	}
type DataContext interface {
	Get(key string) (val any, ok bool) // 获取指定键的值，第二个返回值表示键是否存在
	GetString(key string) (string, bool)
	GetInt(key string) (int, bool)

	Set(key string, val any) // 设置指定键的值，如果键已存在则覆盖
	MarkSet(key string, val any)
}

// Context 核心上下文接口
// 继承标准库Context，并扩展多种功能，是系统中请求处理的核心数据结构
//
// 功能说明:
//   - 提供请求的上下文环境和生命周期管理
//   - 集成日志记录、数据存储、请求头管理等功能
//   - 支持JWT认证和通知系统集成
//
// 使用示例:
//
//	func HandleUserRequest(ctx Context) {
//	    // 获取和处理请求头
//	    header := ctx.TakeHeader()
//	    traceId := header.TraceId()
//
//	    // 存储数据
//	    ctx.Set("start_time", time.Now())
//
//	    // 记录日志
//	    ctx.Info("HandleUserRequest", "开始处理用户请求", map[string]interface{}{
//	        "trace_id": traceId,
//	        "path": header.Path(),
//	    })
//
//	    // 处理JWT认证
//	    if claim := ctx.JwtClaim(); claim != nil {
//	        userId := claim.TakeId()
//	        // 处理认证用户的逻辑
//	    }
//
//	    // 错误处理和通知
//	    if err := processRequest(); err != nil {
//	        ctx.Error("HandleUserRequest", "处理请求失败", err, nil)
//	        ctx.SendError("用户请求处理失败", err, map[string]interface{}{
//	            "trace_id": traceId,
//	        })
//	    }
//	}
//
// 注意事项:
//   - Context包含多种功能，应根据实际需求使用适当的方法
//   - 在使用完毕后，应当释放Context以避免资源泄漏
type Context interface {
	context.Context             // 标准 context，提供取消、超时和值传递能力
	LogContext                  // 日志功能，提供多级别日志记录能力
	DataContext                 // 数据功能，提供键值存储能力
	TakeHeader() IHeader        // 获取请求头信息
	StoreHeader(header IHeader) // 设置请求头信息
	WithMark(marks ...string)   // 设置标记，用于请求分类和追踪
	TakeMarks() map[string]any  // 获取标记信息
	Clone() Context             // 创建上下文副本，用于派生新的上下文
	TakeLogger() ILogger        // 获取日志记录器，用于自定义日志记录
	//ReceiveDB(name string) *gorm.DB // 工厂
	//ReceiveRedis(name string) *redis.Client

	// 日志方法（实现LogContext接口）
	Info(location, msg string, data any)
	Trace(location, msg string, data any)
	Debug(location, msg string, data any)
	Warn(location, msg string, data any)
	Fatal(location, msg string, data any)
	Panic(location, msg string, data any)
	Error(location, message string, err error, data any)

	// JWT相关方法
	StoreJwtClaim(claim IJwtClaim) // 存储JWT声明，用于身份验证
	JwtClaim() (claim IJwtClaim)   // 获取JWT声明，用于身份验证

	// 通知相关方法
	SendError(title string, err error, fields ...map[string]interface{}) // 发送错误通知，适用于重要错误
	SendWarning(title string, fields ...map[string]interface{})          // 发送警告通知，适用于需要注意的情况
	SendSuccess(title string, fields ...map[string]interface{})          // 发送成功通知，适用于重要操作成功
	SendInfo(title string, fields ...map[string]interface{})             // 发送信息通知，适用于一般性通知
}

// 确保DefaultContext实现了Context接口
var _ Context = (*DefaultContext)(nil)

// DefaultContextPool 默认上下文对象池
// 用于减少上下文对象创建的开销，实现对象复用
var defaultContextPool = sync.Pool{
	New: func() interface{} {
		return &DefaultContext{
			ctxInternal: context.Background(),
			dataCache:   make(map[string]interface{}, 16), // 增加初始容量，减少扩容
			rwMutex:     sync.RWMutex{},                   // 预先初始化锁
			activeTask:  0,
		}
	},
}

// recycleContext 回收上下文对象到池中
// 清空数据但保留底层结构以减少内存分配
//
// 参数:
//   - ctx: 要回收的上下文对象
//
// 注意事项:
//   - 回收后的上下文不应再被使用，可能导致不可预期的行为
func recycleContext(ctx *DefaultContext) {
	if ctx == nil {
		return
	}

	// 清空数据缓存但保留底层map结构
	ctx.rwMutex.Lock()
	for k := range ctx.dataCache {
		delete(ctx.dataCache, k)
	}
	ctx.rwMutex.Unlock()

	// 重置状态
	ctx.Header = nil
	ctx.logger = nil
	ctx.event = nil
	ctx.robotCache = nil
	atomic.StoreInt32(&ctx.activeTask, 0)

	// 放回池中
	defaultContextPool.Put(ctx)
}

// NewContext 创建一个默认上下文实例
// 从对象池获取或创建新的上下文实例，并进行初始化
//
// 参数:
//   - endpoint: 服务端点，用于确定请求来源
//   - path: 请求路径，标识处理目标
//
// 返回:
//   - Context: 初始化完成的上下文实例
//
// 使用示例:
//
//	ctx := sylph.NewContext(sylph.EndpointWeb, "/api/users")
//	defer func() {
//	    // 使用完毕后释放上下文，通常在请求处理完成后
//	    if releaser, ok := ctx.(*sylph.DefaultContext); ok {
//	        releaser.Release()
//	    }
//	}()
func NewContext(endpoint Endpoint, path string) Context {
	// 从对象池获取实例
	ctx := defaultContextPool.Get().(*DefaultContext)

	// 初始化必要字段
	ctx.ctxInternal = context.Background()
	header := &Header{
		EndpointVal: endpoint,
		PathVal:     path,
		TraceIdVal:  generateTraceId(),
	}

	ctx.StoreHeader(header)
	ctx.logger = _loggerManager.Receive(string(endpoint))
	ctx.event = nil // 懒加载
	ctx.robotCache = nil

	// 确保数据映射初始化
	if ctx.dataCache == nil {
		ctx.dataCache = make(map[string]interface{}, 8)
	}
	ctx.rwMutex = sync.RWMutex{}

	return ctx
}

// DefaultContext 默认上下文实现
// 实现了Context接口，是系统中请求处理的核心数据结构
//
// 功能说明:
//   - 提供请求的上下文环境和生命周期管理
//   - 集成日志记录、数据存储、请求头管理等功能
//   - 支持JWT认证和通知系统集成
//   - 实现并发安全的数据存储和访问
//
// 字段说明:
//   - ctxInternal: 内部标准库上下文，提供取消和超时功能
//   - dataCache: 并发安全的键值存储，用于请求生命周期内的数据存储
//   - rwMutex: 读写锁，保护dataCache的并发访问
//   - Header: 请求头信息，包含端点、路径、追踪ID等
//   - Marks: 自定义标记列表，用于请求分类和追踪
//   - logger: 日志记录器，用于记录不同级别的日志
//   - event: 事件系统，用于发布和订阅事件
//   - robotCache: 机器人通知缓存，用于系统通知
//   - activeTask: 当前活跃任务计数，用于追踪异步任务
//
// 使用示例:
//
//	ctx := sylph.NewContext(sylph.EndpointWeb, "/api/users")
//	defer func() {
//	    if releaser, ok := ctx.(*sylph.DefaultContext); ok {
//	        releaser.Release() // 释放资源
//	    }
//	}()
//
//	// 使用上下文处理请求
//	ctx.Info("HandleRequest", "开始处理请求", nil)
//	ctx.Set("user_id", "12345")
//	// ...其他操作
type DefaultContext struct {
	ctxInternal context.Context         // 内部上下文
	dataCache   map[string]interface{}  // 并发安全的键值存储，使用读写锁保护
	rwMutex     sync.RWMutex            // 保护dataCache的读写锁
	Header      *Header                 `json:"header"` // 请求头信息
	Marks       []string                `json:"marks"`  // 自定义标记
	logger      ILogger                 // 日志记录器
	event       *event                  // 事件系统（懒加载）
	robotCache  *map[string]interface{} // 机器人通知缓存
	activeTask  int32                   // 当前活跃任务计数（原子操作）
}

// WithTimeout 创建一个带超时限制的新上下文
// 基于当前上下文创建一个新的上下文，在指定时间后自动取消
//
// 参数:
//   - duration: 超时时间
//
// 返回:
//   - timeoutCtx: 带超时限制的新上下文，超时后会自动取消
//   - cancel: 取消函数，可提前终止上下文
//
// 使用示例:
//
//	timeoutCtx, cancel := ctx.WithTimeout(5 * time.Second)
//	defer cancel() // 确保资源被释放
//
//	// 使用带超时的上下文执行操作
//	result, err := performOperationWithContext(timeoutCtx)
//	if err != nil {
//	    if errors.Is(err, context.DeadlineExceeded) {
//	        // 处理超时情况
//	    }
//	}
//
// 注意事项:
//   - 应始终调用返回的cancel函数，即使上下文已超时，以释放资源
//   - 超时后，通过上下文派生的所有操作都应被取消
func (d *DefaultContext) WithTimeout(duration time.Duration) (timeoutCtx Context, cancel context.CancelFunc) {
	var ctx context.Context
	ctx, cancel = context.WithTimeout(d.ctxInternal, duration)
	return &DefaultContext{
		ctxInternal: ctx,
		dataCache:   make(map[string]interface{}, 8),
		rwMutex:     sync.RWMutex{},
		Header:      d.Header,
		Marks:       d.Marks,
		logger:      d.logger,
		event:       d.event,
		robotCache:  d.robotCache,
	}, cancel
}

// WithValue 创建一个带键值对的新上下文
//
// 参数:
//   - key: 键名，可以是任意类型，但通常建议使用自定义结构体类型避免冲突
//   - val: 值，可以是任意类型
//
// 返回:
//   - Context: 新的上下文实例，包含已添加的键值对
//
// 实现逻辑:
//  1. 基于内部ctxInternal创建带值的标准库上下文
//  2. 创建新的DefaultContext实例，复制原上下文的关键属性
//  3. 新实例使用带值上下文替代原内部上下文
//  4. 值存储在标准库context中，可通过Value()方法获取，而非dataCache
func (d *DefaultContext) WithValue(key, val any) Context {
	// 创建内部标准库上下文
	newCtx := context.WithValue(d.ctxInternal, key, val)

	// 创建新的DefaultContext
	return &DefaultContext{
		ctxInternal: newCtx,
		dataCache:   make(map[string]interface{}, 8),
		rwMutex:     sync.RWMutex{},
		Header:      d.Header,
		Marks:       d.Marks,
		logger:      d.logger,
		event:       d.event,
		robotCache:  d.robotCache,
	}
}

// WithMark 向上下文添加标记，用于日志和追踪
//
// 参数:
//   - marks: 一个或多个标记字符串
//
// 实现逻辑:
//  1. 使用读写锁保护并发访问
//  2. 利用map检查重复，确保每个标记只添加一次
//  3. 标记存储在Marks切片中，而非标准库context
//  4. 直接修改当前上下文，不返回新实例
func (d *DefaultContext) WithMark(marks ...string) {
	d.rwMutex.Lock()
	defer d.rwMutex.Unlock()

	if d.Marks == nil {
		d.Marks = make([]string, 0, 8)
	}

	// 快速检查重复，使用map而不是嵌套循环
	if len(d.Marks) > 0 {
		// 创建现有标记的映射，用于快速查找
		existingMarks := make(map[string]struct{}, len(d.Marks))
		for _, mark := range d.Marks {
			existingMarks[mark] = struct{}{}
		}

		// 过滤出不重复的标记
		for _, newMark := range marks {
			if _, exists := existingMarks[newMark]; !exists {
				d.Marks = append(d.Marks, newMark)
			}
		}
	} else {
		// 没有现有标记，直接添加所有新标记
		d.Marks = append(d.Marks, marks...)
	}
}

// TakeMarks 获取上下文中的所有标记及其对应值
//
// 返回:
//   - map[string]any: 标记及其对应值的映射
//
// 实现逻辑:
//  1. 遍历Marks切片中的所有标记
//  2. 对每个标记，尝试从dataCache中获取同名键的值
//  3. 如果找到对应值，则将标记和值加入返回的映射
//  4. 允许标记不仅作为标识，还可作为dataCache的键使用
func (d *DefaultContext) TakeMarks() map[string]any {
	marksMap := make(map[string]any)

	for _, mark := range d.Marks {
		if val, ok := d.Get(mark); ok {
			marksMap[mark] = val
		}
	}

	return marksMap
}

// Deadline 实现context.Context接口
func (d *DefaultContext) Deadline() (deadline time.Time, ok bool) {
	return d.ctxInternal.Deadline()
}

// Done 实现context.Context接口
func (d *DefaultContext) Done() <-chan struct{} {
	return d.ctxInternal.Done()
}

// Err 实现context.Context接口
func (d *DefaultContext) Err() error {
	return d.ctxInternal.Err()
}

// Value 实现context.Context接口
func (d *DefaultContext) Value(key interface{}) interface{} {
	return d.ctxInternal.Value(key)
}

// TakeHeader 获取请求头信息
func (d *DefaultContext) TakeHeader() IHeader {
	// 使用原子加载避免锁竞争
	return (*Header)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&d.Header))))
}

// robotHeader 构建机器人通知的头部信息（带缓存）
func (d *DefaultContext) robotHeader() map[string]interface{} {
	// 使用缓存提高性能
	if d.robotCache != nil {
		return *d.robotCache
	}

	now := time.Now()
	traceId := d.TakeHeader().TraceId()

	// 预先计算格式化字符串，减少格式化开销
	timeStr := now.Format("200601/02")
	hourStr := fmt.Sprintf("%d", now.Hour())

	// 创建缓存并返回
	h := map[string]interface{}{
		"Mark":    d.Header.MarkVal,
		"TraceId": traceId,
		"Command": "grep " + traceId + " /wider-logs/" + timeStr + "/" + string(d.Header.EndpointVal) + "." + hourStr + ".*.log",
	}

	d.robotCache = &h
	return h
}

// recover 恢复函数，用于捕获并处理panic
func (d *DefaultContext) recover() {
	if r := recover(); r != nil {
		d.Error("x.DefaultContext.recover", "context error", errors.Errorf("%v", r), map[string]interface{}{
			"stack": takeStack(),
		})
	}
}

// takeEvent 获取事件系统，如果不存在则创建（懒加载）
func (d *DefaultContext) takeEvent() *event {
	if d.event == nil {
		d.event = newEvent()
	}
	return d.event
}

// On 订阅事件
func (d *DefaultContext) On(eventName string, handlers ...EventHandler) {
	d.takeEvent().On(eventName, handlers...)
}

// OffEvent 取消订阅事件
func (d *DefaultContext) OffEvent(eventName string) {
	d.takeEvent().Off(eventName)
}

// Emit 触发事件（同步执行）
func (d *DefaultContext) Emit(eventName string, payload interface{}) {
	d.takeEvent().Emit(d, eventName, payload)
}

// AsyncEmit 异步触发事件(不等待)
// 启动处理程序但不等待其完成
func (d *DefaultContext) AsyncEmit(eventName string, payload interface{}) {
	d.takeEvent().AsyncEmitNoWait(d, eventName, payload)
}

// AsyncEmitAndWait 异步触发事件并等待完成
// 启动处理程序并等待所有处理完成
func (d *DefaultContext) AsyncEmitAndWait(eventName string, payload interface{}) {
	d.takeEvent().AsyncEmit(d, eventName, payload)
}

// increaseActiveTask 增加活跃任务计数
func (d *DefaultContext) increaseActiveTask() {
	atomic.AddInt32(&d.activeTask, 1)
}

// decreaseActiveTask 减少活跃任务计数
func (d *DefaultContext) decreaseActiveTask() {
	atomic.AddInt32(&d.activeTask, -1)
}

// taskDone 在任务完成时调用
func (d *DefaultContext) taskDone() {
	d.decreaseActiveTask()
}

// safeGo 安全启动协程
func (d *DefaultContext) safeGo(location string, fn func()) {
	d.increaseActiveTask()
	go func() {
		defer d.recover()
		defer d.taskDone()
		fn()
	}()
}

// SendError 发送错误消息
// 参数:
//   - title: 错误标题
//   - err: 错误对象
//   - fields: 额外字段
func (d *DefaultContext) SendError(title string, err error, fields ...map[string]interface{}) {
	// 快速检查，避免不必要的goroutine启动
	if errorRoboter == nil {
		return
	}

	d.safeGo("x.DefaultContext.SendError", func() {
		// 再次检查，因为可能在goroutine启动前后发生变化
		if errorRoboter == nil {
			return
		}

		// 预先分配足够容量的切片，避免后续扩容
		allFields := make([]map[string]interface{}, 0, len(fields)+1)
		allFields = append(allFields, d.robotHeader())
		allFields = append(allFields, fields...)

		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}

		if err1 := errorRoboter.Send(title, errMsg, allFields...); err1 != nil {
			d.Error("x.DefaultContext.SendError", "send failed", err1, map[string]interface{}{})
		}
	})
}

// SendWarning 发送警告消息
// 参数:
//   - title: 警告标题
//   - fields: 额外字段
func (d *DefaultContext) SendWarning(title string, fields ...map[string]interface{}) {
	if warningRoboter == nil {
		return
	}

	d.safeGo("x.DefaultContext.SendWarning", func() {
		if warningRoboter == nil {
			return
		}

		allFields := make([]map[string]interface{}, 0, len(fields)+1)
		allFields = append(allFields, d.robotHeader())
		allFields = append(allFields, fields...)

		if err := warningRoboter.Send(title, "", allFields...); err != nil {
			d.Error("x.DefaultContext.SendWarning", "send failed", err, map[string]interface{}{})
		}
	})
}

// SendSuccess 发送成功消息
// 参数:
//   - title: 成功标题
//   - fields: 额外字段
func (d *DefaultContext) SendSuccess(title string, fields ...map[string]interface{}) {
	if successRoboter == nil {
		return
	}

	d.safeGo("x.DefaultContext.SendSuccess", func() {
		if successRoboter == nil {
			return
		}

		allFields := make([]map[string]interface{}, 0, len(fields)+1)
		allFields = append(allFields, d.robotHeader())
		allFields = append(allFields, fields...)

		if err := successRoboter.Send(title, "", allFields...); err != nil {
			d.Error("x.DefaultContext.SendSuccess", "send failed", err, map[string]interface{}{})
		}
	})
}

// SendInfo 发送信息消息
// 参数:
//   - title: 信息标题
//   - fields: 额外字段
func (d *DefaultContext) SendInfo(title string, fields ...map[string]interface{}) {
	if infoRoboter == nil {
		return
	}

	d.safeGo("x.DefaultContext.SendInfo", func() {
		if infoRoboter == nil {
			return
		}

		allFields := make([]map[string]interface{}, 0, len(fields)+1)
		allFields = append(allFields, d.robotHeader())
		allFields = append(allFields, fields...)

		if err := infoRoboter.Send(title, "", allFields...); err != nil {
			d.Error("x.DefaultContext.SendInfo", "send failed", err, map[string]interface{}{})
		}
	})
}

// makeLoggerMessage 创建日志消息对象
// 将上下文信息整合到日志消息中
func (d *DefaultContext) makeLoggerMessage(location string, msg string, data any) *LoggerMessage {
	// 从对象池获取并初始化对象
	message := GetLoggerMessage() // 使用logger.go中定义的全局函数
	message.Header = d.Header
	message.Marks = d.TakeMarks()
	message.Location = location
	message.Message = msg
	message.Data = data
	return message
}

// 高效处理日志的内部方法
// 统一了日志处理逻辑，减少代码重复
func (d *DefaultContext) logWithLevel(level LogLevel, location string, msg string, data any, err error) {
	// 获取对象池中的消息对象
	message := d.makeLoggerMessage(location, msg, data)

	// 根据日志级别调用不同的处理方法
	switch level {
	case InfoLevel:
		d.logger.Info(message)
	case TraceLevel:
		d.logger.Trace(message)
	case DebugLevel:
		d.logger.Debug(message)
	case WarnLevel:
		d.logger.Warn(message)
	case ErrorLevel:
		d.logger.Error(message, err)
	case FatalLevel:
		d.logger.Fatal(message)
	case PanicLevel:
		d.logger.Panic(message)
	default:
		// 未知级别默认使用Info
		d.logger.Info(message)
	}
}

// Info 记录信息级别日志
func (d *DefaultContext) Info(location string, msg string, data any) {
	d.logWithLevel(InfoLevel, location, msg, data, nil)
}

// Trace 记录跟踪级别日志
func (d *DefaultContext) Trace(location string, msg string, data any) {
	d.logWithLevel(TraceLevel, location, msg, data, nil)
}

// Debug 记录调试级别日志
func (d *DefaultContext) Debug(location string, msg string, data any) {
	d.logWithLevel(DebugLevel, location, msg, data, nil)
}

// Warn 记录警告级别日志
func (d *DefaultContext) Warn(location string, msg string, data any) {
	d.logWithLevel(WarnLevel, location, msg, data, nil)
}

// Fatal 记录致命级别日志
func (d *DefaultContext) Fatal(location string, msg string, data any) {
	d.logWithLevel(FatalLevel, location, msg, data, nil)
}

// Panic 记录恐慌级别日志
func (d *DefaultContext) Panic(location string, msg string, data any) {
	d.logWithLevel(PanicLevel, location, msg, data, nil)
}

// Error 记录错误级别日志
func (d *DefaultContext) Error(location string, message string, err error, data any) {
	d.logWithLevel(ErrorLevel, location, message, data, err)
}

// TakeLogger 获取日志记录器
func (d *DefaultContext) TakeLogger() ILogger {
	return d.logger
}

// Clone 创建上下文副本
// 返回一个新的Context实例，包含原始上下文的头信息和日志记录器
func (d *DefaultContext) Clone() Context {
	// 从对象池获取新的上下文
	newCtx := defaultContextPool.Get().(*DefaultContext)

	// 初始化新上下文
	newCtx.ctxInternal = context.Background()
	newCtx.StoreHeader(d.Header.Clone())
	newCtx.logger = d.logger
	newCtx.event = nil      // 懒加载，不复制
	newCtx.robotCache = nil // 不复制缓存

	// 初始化空的数据存储，不复制原有数据
	if newCtx.dataCache == nil {
		newCtx.dataCache = make(map[string]interface{}, 8)
	} else {
		// 清空已有映射
		for k := range newCtx.dataCache {
			delete(newCtx.dataCache, k)
		}
	}

	return newCtx
}

// Get 获取指定键的值
// 实现DataContext接口
func (d *DefaultContext) Get(key string) (val any, ok bool) {
	d.rwMutex.RLock()
	val, ok = d.dataCache[key]
	d.rwMutex.RUnlock()
	return
}

func (d *DefaultContext) GetString(key string) (string, bool) {
	get, b := d.Get(key)
	return get.(string), b
}

func (d *DefaultContext) GetInt(key string) (int, bool) {
	get, b := d.Get(key)
	return get.(int), b
}

func (d *DefaultContext) MarkSet(key string, val any) {
	d.WithMark(key)
	d.Set(key, val)
}

// Set 设置指定键的值
func (d *DefaultContext) Set(key string, val any) {
	// 可以在此添加安全类型检查
	// 但为了高性能，暂时不添加

	d.rwMutex.Lock()
	d.dataCache[key] = val
	d.rwMutex.Unlock()
}

// StoreJwtClaim 存储JWT声明
func (d *DefaultContext) StoreJwtClaim(claim IJwtClaim) {
	d.Set(jwtClaimKey, claim)
}

// JwtClaim 获取JWT声明
func (d *DefaultContext) JwtClaim() (claim IJwtClaim) {
	val, ok := d.Get(jwtClaimKey)
	if !ok {
		return nil
	}

	if claim, ok := val.(IJwtClaim); ok {
		return claim
	}
	return nil
}

// Release 释放上下文资源，归还对象池
// 需要在上下文使用完毕后显式调用
func (d *DefaultContext) Release() {
	// 清理数据，避免内存泄漏
	d.rwMutex.Lock()
	for k := range d.dataCache {
		delete(d.dataCache, k)
	}
	d.rwMutex.Unlock()

	// 重置字段
	d.Header = nil
	d.logger = nil
	d.event = nil
	d.robotCache = nil
	d.ctxInternal = nil

	// 确保没有正在执行的异步任务
	if atomic.LoadInt32(&d.activeTask) == 0 {
		recycleContext(d)
	} else {
		// 有活跃任务，记录日志但不归还池
		// 避免在使用中归还导致问题
		fmt.Printf("警告: 尝试释放有活跃任务的上下文，任务数: %d\n", d.activeTask)
	}
}

// 新增统一的错误处理工具函数
func handleError(ctx Context, location string, operation string, err error, data map[string]interface{}) {
	if err == nil {
		return
	}

	if data == nil {
		data = map[string]interface{}{}
	}

	data["operation"] = operation
	ctx.Error(location, operation+" failed", err, data)
}

// SafeGo 安全启动协程函数
// 优化版本，针对DefaultContext有更高效的实现
func SafeGo(ctx Context, location string, fn func()) {
	if dctx, ok := ctx.(*DefaultContext); ok {
		// 使用DefaultContext的优化版本
		dctx.safeGo(location, fn)
	} else {
		// 普通实现
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// 直接使用预分配的缓冲区获取堆栈，减少内存分配
					stack := takeStack()

					var err error
					switch v := r.(type) {
					case error:
						err = v
					case string:
						err = errors.New(v)
					default:
						err = fmt.Errorf("%v", r)
					}

					if ctx != nil {
						ctx.Error(location+".recover", "goroutine panic", err, map[string]interface{}{
							"stack": stack,
						})
					} else {
						// 当 ctx 为 nil 时，使用标准错误输出
						_, _ = fmt.Fprintf(os.Stderr, "Goroutine panic in %s: %v\n%s\n",
							location, err, stack)
					}
				}
			}()
			fn()
		}()
	}
}

// WithTimeout 创建带超时的上下文
// 返回带超时的新上下文和取消函数
func WithTimeout(parent Context, timeout time.Duration) (Context, context.CancelFunc) {
	if dctx, ok := parent.(*DefaultContext); ok {
		// 创建带超时的内部上下文
		ctx, cancel := context.WithTimeout(dctx.ctxInternal, timeout)

		// 创建新的派生上下文
		newCtx := dctx.Clone().(*DefaultContext)
		newCtx.ctxInternal = ctx

		return newCtx, cancel
	}

	// 不是DefaultContext，回退到标准实现
	// 这种情况性能较差，因为需要包装整个Context接口
	ctxWithTimeout, cancel := context.WithTimeout(parent, timeout)
	return &ctxWrapper{Context: ctxWithTimeout, parent: parent}, cancel
}

// ctxWrapper 上下文包装器
// 用于非DefaultContext的情况
type ctxWrapper struct {
	context.Context
	parent Context
}

func (w *ctxWrapper) WithMark(marks ...string) {
	w.parent.WithMark(marks...)
}

func (w *ctxWrapper) TakeMarks() map[string]interface{} {
	return w.parent.TakeMarks()
}

// Clone 实现Context接口的Clone方法
func (w *ctxWrapper) Clone() Context {
	return w.parent.Clone()
}

// 委托所有其他方法给parent
func (w *ctxWrapper) TakeHeader() IHeader        { return w.parent.TakeHeader() }
func (w *ctxWrapper) TakeLogger() ILogger        { return w.parent.TakeLogger() }
func (w *ctxWrapper) Get(key string) (any, bool) { return w.parent.Get(key) }
func (w *ctxWrapper) GetString(key string) (string, bool) {
	get, b := w.Get(key)
	return get.(string), b
}
func (w *ctxWrapper) GetInt(key string) (int, bool) {
	get, b := w.Get(key)
	return get.(int), b
}
func (w *ctxWrapper) Set(key string, val any) { w.parent.Set(key, val) }

func (w *ctxWrapper) MarkSet(key string, val any) {
	w.parent.MarkSet(key, val)
}

func (w *ctxWrapper) Info(location, msg string, data any)  { w.parent.Info(location, msg, data) }
func (w *ctxWrapper) Trace(location, msg string, data any) { w.parent.Trace(location, msg, data) }
func (w *ctxWrapper) Debug(location, msg string, data any) { w.parent.Debug(location, msg, data) }
func (w *ctxWrapper) Warn(location, msg string, data any)  { w.parent.Warn(location, msg, data) }
func (w *ctxWrapper) Fatal(location, msg string, data any) { w.parent.Fatal(location, msg, data) }
func (w *ctxWrapper) Panic(location, msg string, data any) { w.parent.Panic(location, msg, data) }
func (w *ctxWrapper) Error(location, message string, err error, data any) {
	w.parent.Error(location, message, err, data)
}
func (w *ctxWrapper) StoreJwtClaim(claim IJwtClaim) { w.parent.StoreJwtClaim(claim) }
func (w *ctxWrapper) JwtClaim() IJwtClaim           { return w.parent.JwtClaim() }
func (w *ctxWrapper) SendError(title string, err error, fields ...map[string]interface{}) {
	w.parent.SendError(title, err, fields...)
}
func (w *ctxWrapper) SendWarning(title string, fields ...map[string]interface{}) {
	w.parent.SendWarning(title, fields...)
}
func (w *ctxWrapper) SendSuccess(title string, fields ...map[string]interface{}) {
	w.parent.SendSuccess(title, fields...)
}
func (w *ctxWrapper) SendInfo(title string, fields ...map[string]interface{}) {
	w.parent.SendInfo(title, fields...)
}
func (w *ctxWrapper) StoreHeader(header IHeader) { w.parent.StoreHeader(header) }

// StoreHeader 原子操作设置请求头信息
func (d *DefaultContext) StoreHeader(header IHeader) {
	if h, ok := header.(*Header); ok {
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&d.Header)), unsafe.Pointer(h))
	}
}

// WithDeadline 创建一个具有截止时间的新上下文
//
// 参数:
//   - deadline: 截止时间点，到达该时间点后上下文自动取消
//
// 返回:
//   - deadlineCtx: 带截止时间的新上下文
//   - cancel: 取消函数，可提前终止上下文
//
// 实现逻辑:
//  1. 基于内部ctxInternal创建带截止时间的标准库上下文
//  2. 创建新的DefaultContext实例，复制原上下文的关键属性
//  3. 新实例使用带截止时间上下文替代原内部上下文
func (d *DefaultContext) WithDeadline(deadline time.Time) (deadlineCtx Context, cancel context.CancelFunc) {
	var ctx context.Context
	ctx, cancel = context.WithDeadline(d.ctxInternal, deadline)
	return &DefaultContext{
		ctxInternal: ctx,
		dataCache:   make(map[string]interface{}, 8),
		rwMutex:     sync.RWMutex{},
		Header:      d.Header,
		Marks:       d.Marks,
		logger:      d.logger,
		event:       d.event,
		robotCache:  d.robotCache,
	}, cancel
}

// WithCancel 创建一个可取消的新上下文
//
// 返回:
//   - cancelCtx: 可取消的新上下文
//   - cancel: 取消函数，调用后会终止上下文
//
// 实现逻辑:
//  1. 基于内部ctxInternal创建可取消的标准库上下文
//  2. 创建新的DefaultContext实例，复制原上下文的关键属性
//  3. 新实例使用可取消上下文替代原内部上下文
func (d *DefaultContext) WithCancel() (cancelCtx Context, cancel context.CancelFunc) {
	var ctx context.Context
	ctx, cancel = context.WithCancel(d.ctxInternal)
	return &DefaultContext{
		ctxInternal: ctx,
		dataCache:   make(map[string]interface{}, 8),
		rwMutex:     sync.RWMutex{},
		Header:      d.Header,
		Marks:       d.Marks,
		logger:      d.logger,
		event:       d.event,
		robotCache:  d.robotCache,
	}, cancel
}

// WithCancelCause 创建一个可带原因取消的新上下文
//
// 返回:
//   - cancelCtx: 可取消的新上下文
//   - cancel: 带原因的取消函数，可传入错误说明取消原因
//
// 实现逻辑:
//  1. 基于内部ctxInternal创建带取消原因的标准库上下文
//  2. 创建新的DefaultContext实例，复制原上下文的关键属性
//  3. 新实例使用可取消上下文替代原内部上下文
//  4. 取消函数支持传入error作为取消原因
func (d *DefaultContext) WithCancelCause() (cancelCtx Context, cancel context.CancelCauseFunc) {
	var ctx context.Context
	ctx, cancel = context.WithCancelCause(d.ctxInternal)
	return &DefaultContext{
		ctxInternal: ctx,
		dataCache:   make(map[string]interface{}, 8),
		rwMutex:     sync.RWMutex{},
		Header:      d.Header,
		Marks:       d.Marks,
		logger:      d.logger,
		event:       d.event,
		robotCache:  d.robotCache,
	}, cancel
}

// Cause 获取上下文取消的原因
//
// 返回:
//   - error: 上下文的取消原因，如果上下文未取消或取消时未提供原因则为nil
//
// 实现逻辑:
//   - 直接委托给标准库的context.Cause函数处理内部上下文
func (d *DefaultContext) Cause() error {
	return context.Cause(d.ctxInternal)
}
