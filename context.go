package sylph

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
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
// 继承标准库context.Context，并扩展多种功能
type Context interface {
	context.Context      // 标准 context
	LogContext           // 日志功能
	DataContext          // 数据功能
	TakeHeader() IHeader // 获取请求头信息
	Clone() Context      // 创建上下文副本

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
	SendError(title string, err error, fields ...H) // 发送错误通知
	SendWarning(title string, fields ...H)          // 发送警告通知
	SendSuccess(title string, fields ...H)          // 发送成功通知
	SendInfo(title string, fields ...H)             // 发送信息通知
}

var _ Context = (*DefaultContext)(nil) // 确保DefaultContext实现了Context接口

// NewDefaultContext 创建一个默认上下文实例
// 参数:
//   - endpoint: 服务端点
//   - path: 请求路径
//
// 返回:
//   - Context: 上下文实例
func NewDefaultContext(endpoint Endpoint, path string) Context {
	return &DefaultContext{
		Context: context.Background(),
		Header: &Header{
			Endpoint:   endpoint,
			PathVal:    path,
			TraceIdVal: generateTraceId(),
		},
		logger: _loggerManager.Receive(string(endpoint)),
	}
}

// DefaultContext 默认上下文实现
type DefaultContext struct {
	context.Context          // 内嵌标准库上下文
	mapping         sync.Map // 键值存储
	Header          *Header  `json:"header"` // 请求头信息
	logger          ILogger  // 日志记录器
	event           *event   // 事件系统
}

// TakeHeader 获取请求头信息
func (d *DefaultContext) TakeHeader() IHeader {
	return d.Header
}

// robotHeader 构建机器人通知的头部信息
func (d *DefaultContext) robotHeader() (h H) {
	now := time.Now()
	traceId := d.TakeHeader().TraceId()

	h = H{
		"Mark":    d.Header.MarkVal,
		"TraceId": traceId,
		"Command": fmt.Sprintf("grep %s /wider-logs/%s/%s.%d.*.log", traceId, now.Format("200601/02"), d.Header.Endpoint, now.Hour()),
	}
	return
}

// recover 恢复函数，用于捕获并处理panic
func (d *DefaultContext) recover() {
	if r := recover(); r != nil {
		d.Error("x.DefaultContext.recover", "context error", errors.Errorf("%v", r), H{
			"stack": takeStack(),
		})
	}
}

// takeEvent 获取事件系统，如果不存在则创建
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

// SendError 发送错误消息
// 参数:
//   - title: 错误标题
//   - err: 错误对象
//   - fields: 额外字段
func (d *DefaultContext) SendError(title string, err error, fields ...H) {
	SafeGo(d, "x.DefaultContext.SendError", func() {
		if errorRoboter == nil {
			return
		}

		fields = append([]H{d.robotHeader()}, fields...)
		if err1 := errorRoboter.Send(title, err.Error(), fields...); err1 != nil {
			d.Error("x.DefaultContext.SendError", "send failed", err1, H{})
		}
	})
}

// SendWarning 发送警告消息
// 参数:
//   - title: 警告标题
//   - fields: 额外字段
func (d *DefaultContext) SendWarning(title string, fields ...H) {
	SafeGo(d, "x.DefaultContext.SendWarning", func() {
		if warningRoboter == nil {
			return
		}

		fields = append([]H{d.robotHeader()}, fields...)
		if err := warningRoboter.Send(title, "", fields...); err != nil {
			d.Error("x.DefaultContext.SendWarning", "send failed", err, H{})
		}
	})
}

// SendSuccess 发送成功消息
// 参数:
//   - title: 成功标题
//   - fields: 额外字段
func (d *DefaultContext) SendSuccess(title string, fields ...H) {
	SafeGo(d, "x.DefaultContext.SendSuccess", func() {
		if successRoboter == nil {
			return
		}

		fields = append([]H{d.robotHeader()}, fields...)
		if err := successRoboter.Send(title, "", fields...); err != nil {
			d.Error("x.DefaultContext.SendSuccess", "send failed", err, H{})
		}
	})
}

// SendInfo 发送信息消息
// 参数:
//   - title: 信息标题
//   - fields: 额外字段
func (d *DefaultContext) SendInfo(title string, fields ...H) {
	SafeGo(d, "x.DefaultContext.SendInfo", func() {
		if infoRoboter == nil {
			return
		}

		fields = append([]H{d.robotHeader()}, fields...)
		if err := infoRoboter.Send(title, "", fields...); err != nil {
			d.Error("x.DefaultContext.SendInfo", "send failed", err, H{})
		}
	})
}

// makeLoggerMessage 创建日志消息对象
// 将上下文信息整合到日志消息中
func (d *DefaultContext) makeLoggerMessage(location string, msg string, data any) (message *LoggerMessage) {
	return &LoggerMessage{
		Header:   d.Header,
		Location: location,
		Message:  msg,
		Data:     data,
	}
}

// Info 记录信息级别日志
func (d *DefaultContext) Info(location string, msg string, data any) {
	d.logger.Info(d.makeLoggerMessage(location, msg, data))
}

// Trace 记录跟踪级别日志
func (d *DefaultContext) Trace(location string, msg string, data any) {
	d.logger.Trace(d.makeLoggerMessage(location, msg, data))
}

// Debug 记录调试级别日志
func (d *DefaultContext) Debug(location string, msg string, data any) {
	d.logger.Debug(d.makeLoggerMessage(location, msg, data))
}

// Warn 记录警告级别日志
func (d *DefaultContext) Warn(location string, msg string, data any) {
	d.logger.Warn(d.makeLoggerMessage(location, msg, data))
}

// Fatal 记录致命级别日志
func (d *DefaultContext) Fatal(location string, msg string, data any) {
	d.logger.Fatal(d.makeLoggerMessage(location, msg, data))
}

// Panic 记录恐慌级别日志
func (d *DefaultContext) Panic(location string, msg string, data any) {
	d.logger.Panic(d.makeLoggerMessage(location, msg, data))
}

// Error 记录错误级别日志
func (d *DefaultContext) Error(location string, message string, err error, data any) {
	d.logger.Error(d.makeLoggerMessage(location, message, data), err)
}

// TakeLogger 获取日志记录器
func (d *DefaultContext) TakeLogger() ILogger {
	return d.logger
}

// Clone 创建上下文副本
// 返回一个新的Context实例，包含原始上下文的头信息和日志记录器
func (d *DefaultContext) Clone() (ctx Context) {
	ctx = &DefaultContext{
		Context: context.Background(),
		Header:  d.Header.Clone(),
		logger:  d.logger,
	}

	return
}

// Get 获取指定键的值
// 实现DataContext接口
func (d *DefaultContext) Get(key string) (val any, ok bool) {
	return d.mapping.Load(key)
}

func (d *DefaultContext) Set(key string, val any) {
	// 类型白名单检查
	//switch val.(type) {
	//case string, int, int64, float64, bool, []string, []int, map[string]string:
	//	// 允许的类型
	//default:
	//	// 对于复杂类型，尝试序列化以确保安全
	//	if _, err := _json.Marshal(val); err != nil {
	//		// 记录警告并拒绝不安全的值
	//		d.Warn("x.DefaultContext.Set", "unsafe value type", H{
	//			"key":  key,
	//			"err":  err.Error(),
	//			"type": fmt.Sprintf("%T", val),
	//		})
	//		return
	//	}
	//}

	d.mapping.Store(key, val)
}

func (d *DefaultContext) StoreJwtClaim(claim IJwtClaim) {
	d.mapping.Store(jwtClaimKey, claim)
}

func (d *DefaultContext) JwtClaim() (claim IJwtClaim) {
	val, ok := d.mapping.Load(jwtClaimKey)
	if !ok {
		return
	}

	return val.(IJwtClaim)
}

// 新增统一的错误处理工具函数
func handleError(ctx Context, location string, operation string, err error, data H) {
	if err == nil {
		return
	}

	if data == nil {
		data = H{}
	}

	data["operation"] = operation
	ctx.Error(location, operation+" failed", err, data)
}
