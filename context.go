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
type LogLevel int

// 日志级别常量定义
const (
	DebugLevel LogLevel = iota
	InfoLevel
	TraceLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	PanicLevel
)

const (
	jwtClaimKey = "x:jwt:claim" // JWT声明的存储键名
)

// IJwtClaim 定义JWT声明接口
// 提供获取用户ID、令牌、颁发者及验证颁发者的方法
type IJwtClaim interface {
	TakeId() string            // 获取用户ID
	TakeToken() string         // 获取令牌字符串
	TakeIssuer() string        // 获取颁发者名称
	IssuerIs(name string) bool // 验证颁发者是否匹配
}

// LogContext 日志上下文接口
// 定义了各种日志级别的记录方法
type LogContext interface {
	Info(location, msg string, data any)                 // 记录信息级别日志
	Trace(location, msg string, data any)                // 记录跟踪级别日志
	Debug(location, msg string, data any)                // 记录调试级别日志
	Warn(location, msg string, data any)                 // 记录警告级别日志
	Fatal(location, msg string, data any)                // 记录致命级别日志
	Panic(location, msg string, data any)                // 记录恐慌级别日志
	Error(location, message string, err error, data any) // 记录错误级别日志
}

// DataContext 数据上下文接口
// 提供键值存储功能
type DataContext interface {
	Get(key string) (val any, ok bool) // 获取指定键的值
	Set(key string, val any)           // 设置指定键的值
}

// Context 核心上下文接口
// 继承标准库Context，并扩展多种功能
type Context interface {
	context.Context             // 标准 context
	LogContext                  // 日志功能
	DataContext                 // 数据功能
	TakeHeader() IHeader        // 获取请求头信息
	StoreHeader(header IHeader) // 设置请求头信息
	Clone() Context             // 创建上下文副本

	TakeLogger() ILogger // 获取日志记录器
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
	StoreJwtClaim(claim IJwtClaim) // 存储JWT声明
	JwtClaim() (claim IJwtClaim)   // 获取JWT声明

	// 通知相关方法
	SendError(title string, err error, fields ...map[string]interface{}) // 发送错误通知
	SendWarning(title string, fields ...map[string]interface{})          // 发送警告通知
	SendSuccess(title string, fields ...map[string]interface{})          // 发送成功通知
	SendInfo(title string, fields ...map[string]interface{})             // 发送信息通知
}

// 确保DefaultContext实现了Context接口
var _ Context = (*DefaultContext)(nil)

// DefaultContextPool 默认上下文对象池
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

// NewDefaultContext 创建一个默认上下文实例
// 参数:
//   - endpoint: 服务端点
//   - path: 请求路径
//
// 返回:
//   - Context: 上下文实例
func NewDefaultContext(endpoint Endpoint, path string) Context {
	// 从对象池获取实例
	ctx := defaultContextPool.Get().(*DefaultContext)

	// 初始化必要字段
	ctx.ctxInternal = context.Background()
	header := &Header{
		Endpoint:   endpoint,
		PathVal:    path,
		TraceIdVal: generateTraceId(),
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
type DefaultContext struct {
	ctxInternal context.Context         // 内部上下文
	dataCache   map[string]interface{}  // 并发安全的键值存储，使用读写锁保护
	rwMutex     sync.RWMutex            // 保护dataCache的读写锁
	Header      *Header                 `json:"header"` // 请求头信息
	Marks       map[string]interface{}  `json:"marks"`  // 自定义标记
	logger      ILogger                 // 日志记录器
	event       *event                  // 事件系统（懒加载）
	robotCache  *map[string]interface{} // 机器人通知缓存
	activeTask  int32                   // 当前活跃任务计数（原子操作）
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
		"Command": "grep " + traceId + " /wider-logs/" + timeStr + "/" + string(d.Header.Endpoint) + "." + hourStr + ".*.log",
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
	message.Marks = d.Marks
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

// Clone 实现Context接口的Clone方法
func (w *ctxWrapper) Clone() Context {
	return w.parent.Clone()
}

// 委托所有其他方法给parent
func (w *ctxWrapper) TakeHeader() IHeader                  { return w.parent.TakeHeader() }
func (w *ctxWrapper) TakeLogger() ILogger                  { return w.parent.TakeLogger() }
func (w *ctxWrapper) Get(key string) (any, bool)           { return w.parent.Get(key) }
func (w *ctxWrapper) Set(key string, val any)              { w.parent.Set(key, val) }
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
