package sylph

import (
	"context"
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
//	        logCtx.Error("HandleRequest", "处理请求失败", err, map[string]any{"request_id": reqId})
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
	GetBool(key string) (bool, bool)

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
//	    ctx.Info("HandleUserRequest", "开始处理用户请求", map[string]any{
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
//	    // 错误处理
//	    if err := processRequest(); err != nil {
//	        ctx.Error("HandleUserRequest", "处理请求失败", err, nil)
//	    }
//	}
//
// 注意事项:
//   - Context包含多种功能，应根据实际需求使用适当的方法
type Context interface {
	context.Context             // 标准 context，提供取消、超时和值传递能力
	LogContext                  // 日志功能，提供多级别日志记录能力
	DataContext                 // 数据功能，提供键值存储能力
	TakeHeader() IHeader        // 获取请求头信息
	StoreHeader(header IHeader) // 设置请求头信息
	WithMark(marks ...string)   // 设置标记，用于请求分类和追踪
	TakeMarks() map[string]any  // 获取标记信息
	Clone() Context             // 创建上下文副本，用于派生新的上下文
	SetAbort()
	IsAbort() bool
	BindErrorInfo(err error)
	TakeErrorInfo() error

	TakeLogger() ILogger // 获取日志记录器，用于自定义日志记录

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

	// 数据库支持
	// WithMysqlHandle(handle func(name string) any)
	// WithRedisHandle(handle func(name string) any)
	// Mysql(name string) any
	// Redis(name string) any

	// // 其他数据库支持
	// WithDBHandle(category string, handle func(name string) any)
	// DB(category, name string) any
}

// 确保DefaultContext实现了Context接口
var _ Context = (*DefaultContext)(nil)

// NewContext 创建一个默认上下文实例
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
//	// 使用上下文处理请求...
func NewContext(endpoint Endpoint, path string) Context {
	header := &Header{
		EndpointVal: endpoint,
		PathVal:     path,
		TraceIdVal:  generateTraceId(),
	}

	return &DefaultContext{
		ctxInternal: context.Background(),
		Header:      header,
		logger:      _loggerManager.Receive(string(endpoint)),
		// _db:         &ContextDB{},
	}
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
//   - dataCache: 并发安全的键值存储（sync.Map），用于请求生命周期内的数据存储
//   - rwMutex: 读写锁，保护 Marks 的并发访问
//   - Header: 请求头信息，包含端点、路径、追踪ID等
//   - Marks: 自定义标记列表，用于请求分类和追踪
//   - logger: 日志记录器，用于记录不同级别的日志
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
	ctxInternal context.Context // 内部上下文
	dataCache   sync.Map        // 并发安全的键值存储，无需额外锁
	rwMutex     sync.RWMutex    // 保护 Marks 的读写锁
	Header      *Header         `json:"header"` // 请求头信息
	Marks       []string        `json:"marks"`  // 自定义标记

	logger ILogger // 日志记录器
	abort  int32   // 使用原子操作保护，1表示已中止，0表示未中止
	errors error

	// _db *ContextDB
}

func (d *DefaultContext) BindErrorInfo(err error) {
	if err == nil {
		return
	}

	if d.errors == nil {
		d.SetAbort()
		d.errors = err
	} else {
		d.errors = errors.Wrap(d.errors, err.Error())
	}
}

func (d *DefaultContext) TakeErrorInfo() error {
	return d.errors
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
	ctx, cancel := context.WithTimeout(d.ctxInternal, duration)
	return &DefaultContext{
		ctxInternal: ctx,
		Header:      d.Header,
		Marks:       d.Marks,
		logger:      d.logger,
		// _db:         d._db,
	}, cancel
}

// SetAbort 设置中止标志
// 并发安全：使用原子操作保护
func (d *DefaultContext) SetAbort() {
	atomic.StoreInt32(&d.abort, 1)
}

// IsAbort 检查是否已中止
// 并发安全：使用原子操作保护
func (d *DefaultContext) IsAbort() bool {
	return atomic.LoadInt32(&d.abort) == 1
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
	return &DefaultContext{
		ctxInternal: context.WithValue(d.ctxInternal, key, val),
		Header:      d.Header,
		Marks:       d.Marks,
		logger:      d.logger,
		// _db:         d._db,
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
//  1. 使用读锁保护 Marks 切片的读取
//  2. 遍历Marks切片中的所有标记
//  3. 对每个标记，尝试从dataCache中获取同名键的值
//  4. 如果找到对应值，则将标记和值加入返回的映射
//  5. 允许标记不仅作为标识，还可作为dataCache的键使用
//
// 并发安全：使用读锁保护 Marks 切片的读取，避免与 WithMark() 的写入操作冲突
func (d *DefaultContext) TakeMarks() map[string]any {
	d.rwMutex.RLock()
	marks := d.Marks // 复制引用，避免在锁内进行耗时操作
	d.rwMutex.RUnlock()

	marksMap := make(map[string]any)
	for _, mark := range marks {
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

// makeLoggerMessage 创建日志消息对象
// 将上下文信息整合到日志消息中
func (d *DefaultContext) makeLoggerMessage(location string, msg string, data any) *LoggerMessage {
	// 创建新的消息对象
	message := NewLoggerMessage()
	// 使用原子加载获取 Header，保证并发安全
	if h, ok := d.TakeHeader().(*Header); ok {
		message.Header = h
	}
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
	return &DefaultContext{
		ctxInternal: context.Background(),
		Header:      d.Header.Clone(),
		logger:      d.logger,
		// _db:         d._db,
	}
}

// Get 获取指定键的值
// 实现DataContext接口
// 并发安全：使用 sync.Map
func (d *DefaultContext) Get(key string) (val any, ok bool) {
	return d.dataCache.Load(key)
}

func (d *DefaultContext) GetString(key string) (string, bool) {
	get, b := d.Get(key)
	if b {
		return get.(string), b
	}
	return "", b
}

func (d *DefaultContext) GetInt(key string) (int, bool) {
	get, b := d.Get(key)
	if b {
		return get.(int), b
	}
	return 0, b
}

// GetBool(key string) (bool, bool)
func (d *DefaultContext) GetBool(key string) (bool, bool) {
	get, b := d.Get(key)
	if b {
		return get.(bool), b
	}
	return false, b
}

func (d *DefaultContext) MarkSet(key string, val any) {
	d.WithMark(key)
	d.Set(key, val)
}

// Set 设置指定键的值
// 并发安全：使用 sync.Map
func (d *DefaultContext) Set(key string, val any) {
	d.dataCache.Store(key, val)
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
	ctx, cancel := context.WithDeadline(d.ctxInternal, deadline)
	return &DefaultContext{
		ctxInternal: ctx,
		Header:      d.Header,
		Marks:       d.Marks,
		logger:      d.logger,
		// _db:         d._db,
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
	ctx, cancel := context.WithCancel(d.ctxInternal)
	return &DefaultContext{
		ctxInternal: ctx,
		Header:      d.Header,
		Marks:       d.Marks,
		logger:      d.logger,
		// _db:         d._db,
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
	ctx, cancel := context.WithCancelCause(d.ctxInternal)
	return &DefaultContext{
		ctxInternal: ctx,
		Header:      d.Header,
		Marks:       d.Marks,
		logger:      d.logger,
		// _db:         d._db,
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

// func (c *DefaultContext) WithMysqlHandle(handle func(name string) any) {
// 	c.WithDBHandle(dbMysqlMark, handle)
// }

// func (c *DefaultContext) Mysql(name string) any {
// 	return c.DB(dbMysqlMark, name)
// }

// func (c *DefaultContext) WithRedisHandle(handle func(name string) any) {
// 	c.WithDBHandle(dbRedisMark, handle)
// }

// func (c *DefaultContext) Redis(name string) any {
// 	return c.DB(dbRedisMark, name)
// }

// func (c *DefaultContext) WithDBHandle(category string, handle func(name string) any) {
// 	c._db.WithDBHandle(category, handle)
// }

// func (c *DefaultContext) DB(category, name string) any {
// 	return c._db.DB(category, name)
// }

const (
	dbMysqlMark = "mysql"
	dbRedisMark = "redis"
)

// type ContextDB struct {
// 	_db     sync.Map
// 	_handle sync.Map
// }

// func (c *ContextDB) WithDBHandle(category string, handle func(name string) any) {
// 	c._handle.Store(category, handle)
// }

// func (c *ContextDB) DB(category, name string) any {
// 	handleAny, ok := c._handle.Load(category)
// 	if !ok {
// 		return nil
// 	}

// 	handle := handleAny.(func(name string) any)

// 	key := c.makeLoadKey(category, name)
// 	if v, ok := c._db.Load(key); ok {
// 		return v
// 	}

// 	db := handle(name)
// 	actual, _ := c._db.LoadOrStore(key, db)
// 	return actual
// }

// func (c *ContextDB) makeLoadKey(category, name string) string {
// 	return category + ":" + name
// }
