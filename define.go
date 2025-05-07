package sylph

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// 服务类型常量，用于表示不同类型的服务
// 系统支持多种服务类型，每种类型有不同的实现和特性
const (
	HttpKind         Kind = iota + 1 // HTTP服务类型，提供Web API能力
	CrontabKind                      // 计划任务服务类型，提供定时任务调度能力
	MessageQueueKind                 // 消息队列服务类型，提供异步消息处理能力
	RedisQueueKind                   // Redis队列服务类型，基于Redis的轻量级队列实现
)

// Kind 服务类型枚举
// 用于区分不同的服务实现方式，便于服务管理和路由
//
// 使用示例:
//
//	if server.GetKind() == sylph.HttpKind {
//	    // 处理HTTP服务特有的逻辑
//	}
type Kind int

// 服务名称常量，用于标识具体服务实现
// 每个名称对应一种特定的服务实现技术
const (
	Gin    Name = "gin"    // Gin HTTP框架服务，基于Gin实现的HTTP服务
	Cron   Name = "cron"   // 计划任务服务，基于Cron实现的定时任务服务
	Rocket Name = "rocket" // RocketMQ消息队列服务，基于RocketMQ实现的消息服务
)

// Name 服务名称类型
// 用于标识具体的服务实现，便于服务工厂创建对应的服务实例
//
// 使用示例:
//
//	serverName := sylph.Gin
//	server := factory.CreateServer(serverName)
type Name string

// IProject 服务容器接口
// 管理多个服务的生命周期，提供统一的启动和关闭能力
//
// 使用示例:
//
//	project := sylph.NewProject()
//	project.Mounts(httpServer, cronServer)
//	if err := project.Boots(); err != nil {
//	    log.Fatal(err)
//	}
//	defer project.Shutdowns()
type IProject interface {
	// Mounts 装载服务
	// 将一个或多个服务添加到项目中
	//
	// 参数:
	//   - servers: 要添加的服务列表
	//
	// 返回:
	//   - IProject: 项目实例，支持链式调用
	Mounts(servers ...IServer) IProject

	// Boots 启动服务
	// 启动所有已装载的服务
	//
	// 返回:
	//   - error: 启动过程中的错误，如果全部成功则返回nil
	Boots() error

	// Shutdowns 关闭服务
	// 关闭所有已启动的服务
	//
	// 返回:
	//   - error: 关闭过程中的错误，如果全部成功则返回nil
	Shutdowns() error
}

// IServer 服务定义接口
// 所有服务必须实现此接口，提供基本的生命周期管理
//
// 使用示例:
//
//	type MyServer struct {}
//
//	func (s *MyServer) Name() string {
//	    return "my-server"
//	}
//
//	func (s *MyServer) Boot() error {
//	    // 启动逻辑
//	    return nil
//	}
//
//	func (s *MyServer) Shutdown() error {
//	    // 关闭逻辑
//	    return nil
//	}
type IServer interface {
	// Name 获取服务名称
	// 返回服务的唯一标识名称
	Name() string

	// Boot 启动服务
	// 执行服务的初始化和启动逻辑
	//
	// 返回:
	//   - error: 启动错误，nil表示成功启动
	Boot() error

	// Shutdown 关闭服务
	// 执行服务的清理和关闭逻辑
	//
	// 返回:
	//   - error: 关闭错误，nil表示成功关闭
	Shutdown() error
}

// ITaskName 定义任务名接口
// 用于在消息队列和定时任务系统中标识不同的任务类型
//
// 使用示例:
//
//	type MyTask string
//
//	func (t MyTask) Name() string {
//	    return string(t)
//	}
//
//	const TaskProcessOrder MyTask = "process-order"
type ITaskName interface {
	// Name 返回任务的名称字符串
	// 用于标识特定的任务类型
	Name() string
}

// 全局服务ID变量，用于日志标识
// 通过环境变量设置，用于在分布式系统中唯一标识服务实例
var (
	servId string
)

// 初始化函数，从环境变量获取服务ID
// 在程序启动时自动执行
func init() {
	servId = os.Getenv("LOG_SERV_ID")
}

// Roboter 简单机器人接口
// 提供基本的消息发送能力，用于系统通知和报警
//
// 使用示例:
//
//	robot := GetRobot()
//	if err := robot.Send("系统警报：CPU使用率超过90%"); err != nil {
//	    log.Printf("发送警报失败: %v", err)
//	}
type Roboter interface {
	// Send 发送纯文本消息
	// 参数:
	//   - text: 要发送的文本消息内容
	//
	// 返回:
	//   - error: 发送错误，如果为nil表示发送成功
	Send(text string) error
}

// IStringer 字符串化接口
// 定义对象可以被转换为字符串表示，用于日志记录和调试
//
// 使用示例:
//
//	type Person struct {
//	    Name string
//	    Age  int
//	}
//
//	func (p Person) String() string {
//	    return fmt.Sprintf("Person{Name: %s, Age: %d}", p.Name, p.Age)
//	}
type IStringer interface {
	// String 获取对象的字符串表示
	// 返回: 对象的字符串表示
	String() string
}

// KeyMaker 键生成器接口
// 用于生成系统中的唯一键标识，支持不同的键生成策略
//
// 使用示例:
//
//	keyMaker := sylph.NewPrefixKeyMaker("app")
//	redisKey := keyMaker.Make("user:profile")  // 生成 "app:user:profile"
type KeyMaker interface {
	// Make 生成键字符串
	// 参数:
	//   - key: 基础键名
	//
	// 返回:
	//   - string: 生成的完整键名
	Make(key string) string
}

// IObjectRegistry 对象注册表接口
// 提供对象注册和获取的能力，用于依赖注入和服务定位
//
// 使用示例:
//
//	registry := sylph.NewObjectRegistry()
//
//	// 注册服务
//	registry.Register(keyMaker, "userService", func() interface{} {
//	    return NewUserService()
//	})
//
//	// 获取服务
//	if userService, ok := registry.Receive(keyMaker, "userService", true); ok {
//	    service := userService.(UserService)
//	    service.DoSomething()
//	}
type IObjectRegistry interface {
	// Register 注册对象创建处理器
	// 参数:
	//   - name: 键名生成器
	//   - key: 基础键名
	//   - handler: 对象创建函数
	Register(name KeyMaker, key string, handler func() interface{})

	// Receive 获取已注册的对象
	// 参数:
	//   - name: 键名生成器
	//   - key: 基础键名
	//   - share: 是否共享同一实例
	//
	// 返回:
	//   - interface{}: 目标对象
	//   - bool: 是否成功获取
	Receive(name KeyMaker, key string, share bool) (interface{}, bool)

	// MustReceive 获取已注册的对象，如果不存在则抛出异常
	// 参数:
	//   - name: 键名生成器
	//   - key: 基础键名
	//   - share: 是否共享同一实例
	//
	// 返回:
	//   - interface{}: 目标对象
	//
	// 注意事项:
	//   - 如果对象不存在，此方法会导致程序崩溃，谨慎使用
	MustReceive(name KeyMaker, key string, share bool) interface{}
}

// takeStack 获取当前调用栈信息
// 用于错误追踪和日志记录，提供代码执行路径的详细信息
//
// 返回:
//   - string: 格式化的调用栈信息
//
// 使用示例:
//
//	if err != nil {
//	    log.Printf("发生错误: %v\n调用栈: %s", err, takeStack())
//	}
//
// 注意事项:
//   - 调用栈信息可能较长，请谨慎在生产环境中使用以避免日志过大
func takeStack() string {
	// 获取程序计数器
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(3, pcs[:]) // 跳过当前函数和调用者

	// 创建frames
	frames := runtime.CallersFrames(pcs[:n])

	// 用builder来构建字符串，避免多次内存分配
	var sb strings.Builder

	// 遍历调用栈
	for {
		frame, more := frames.Next()
		if !more {
			break
		}

		// 添加函数名和位置信息
		fmt.Fprintf(&sb, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)

		// 可以设置最大深度限制
		if sb.Len() > 8192 {
			sb.WriteString("...(stack trace too long, truncated)")
			break
		}
	}

	return sb.String()
}

// md5String 使用MD5哈希函数计算字符串的哈希值
// 注意: 仅用于向后兼容，不推荐用于安全场景
//
// 参数:
//   - data: 要哈希的字符串
//
// 返回:
//   - string: 十六进制表示的MD5哈希值
//
// 使用示例:
//
//	cacheKey := md5String("user:" + userId)
//
// 注意事项:
//   - MD5已被证明不安全，不要用于密码存储或安全敏感场景
//   - 安全场景请使用secureHash函数
func md5String(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// secureHash 使用SHA-256哈希函数计算字符串的哈希值
// 比MD5更安全，推荐用于安全场景
//
// 参数:
//   - data: 要哈希的字符串
//
// 返回:
//   - string: 十六进制表示的SHA-256哈希值
//
// 使用示例:
//
//	secureToken := secureHash(username + ":" + timestamp)
func secureHash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Endpoint 表示系统中的服务端点类型
// 用于标识和区分不同的服务组件，便于服务发现和路由
//
// 使用示例:
//
//	myEndpoint := sylph.EndpointWeb
//	if myEndpoint == sylph.EndpointWorker {
//	    // 执行工作节点特有的逻辑
//	}
type Endpoint string

// 系统中定义的各类端点常量
// 每种端点代表系统中的一种服务角色
const (
	EndpointScheduler Endpoint = "scheduler" // 调度器服务端点，负责任务调度和分配
	EndpointWorker    Endpoint = "worker"    // 工作节点服务端点，负责执行具体任务
	EndpointWeb       Endpoint = "web"       // Web服务端点，提供HTTP接口和用户界面
	EndpointBot       Endpoint = "bot"       // 机器人服务端点，提供自动化交互能力
	EndpointTest      Endpoint = "test"      // 测试环境端点，用于系统测试环境
	EndpointStub      Endpoint = "stub"      // 存根服务端点，通常用于模拟测试
)

// CronTask 定义计划任务接口
// 所有需要按计划执行的任务都应实现此接口
//
// 使用示例:
//
//	type EmailReminderTask struct{}
//
//	func (t *EmailReminderTask) Handle(ctx sylph.Context, args interface{}) error {
//	    params := args.(map[string]interface{})
//	    return sendReminderEmail(params["email"].(string), params["message"].(string))
//	}
//
//	// 注册任务
//	cronServer.Register("email-reminder", &EmailReminderTask{})
type CronTask interface {
	// Handle 处理任务的入口方法
	// 参数:
	//   - ctx: 任务执行上下文，可用于取消和日志记录
	//   - args: 任务参数，可以包含任何类型的数据
	//
	// 返回:
	//   - error: 任务执行的错误，如果为nil表示任务执行成功
	Handle(ctx Context, args interface{}) error
}

// IHeader 请求头接口
// 定义了获取请求元数据的方法，用于请求上下文传递
//
// 使用示例:
//
//	func HandleRequest(ctx sylph.Context) {
//	    header := ctx.GetHeader()
//	    traceId := header.TraceId()
//	    logger := ctx.Logger().WithField("trace_id", traceId)
//	    logger.Info("Processing request from %s to %s", header.Endpoint(), header.Path())
//	}
type IHeader interface {
	// Endpoint 获取请求来源的服务端点
	// 返回请求发起的端点类型
	Endpoint() Endpoint

	// Path 获取请求的路径
	// 返回请求的目标路径
	Path() string

	// Mark 获取请求的标记信息
	// 返回请求的自定义标记，用于分类和过滤
	Mark() string

	// TraceId 获取请求的追踪ID
	// 用于在分布式系统中追踪请求的流转
	// 返回唯一的跟踪标识符
	TraceId() string

	Ref() string

	StoreRef(ref string)

	StorePath(path string)

	WithTraceId(traceId string)

	GenerateTraceId()

	ResetTraceId()

	WithMark(mark string)

	StoreIP(ip string)

	IP() string

	// Clone 创建请求头的副本
	// 返回一个新的Header实例，包含相同的值
	Clone() *Header
}

/*

 */

// IMessage 消息接口
// 定义了消息的基本操作方法，用于消息队列系统
//
// 使用示例:
//
//	msg := sylph.NewMessage()
//	msg.SetTopic("orders")
//	msg.SetTags("new")
//	msg.SetData(orderData)
//
//	producer.Send(msg)
type IMessage interface {
	// SetStatus 设置消息状态
	// 参数:
	//   - status: 消息状态码
	SetStatus(status int32)

	// GetStatus 获取消息状态
	// 返回消息当前的状态码
	GetStatus() int32

	// SetMessageID 设置消息ID
	// 参数:
	//   - id: 唯一的消息标识符
	SetMessageID(id string)

	// GetMessageID 获取消息ID
	// 返回消息的唯一标识符
	GetMessageID() string

	// SetRoute 设置消息路由
	// 参数:
	//   - route: 消息的路由键
	SetRoute(route string)

	// GetRoute 获取消息路由
	// 返回消息的路由键
	GetRoute() string

	// SetTopic 设置消息主题
	// 参数:
	//   - topic: 消息发布的主题
	SetTopic(topic string)

	// GetTopic 获取消息主题
	// 返回消息的主题
	GetTopic() string

	// SetTags 设置消息标签
	// 参数:
	//   - tags: 消息的标签，用于分类
	SetTags(tags string)

	// GetTags 获取消息标签
	// 返回消息的标签
	GetTags() string

	// SetData 设置消息数据
	// 参数:
	//   - data: 消息的实际数据内容
	SetData(data interface{})

	// GetData 获取消息数据
	// 返回消息的实际数据内容
	GetData() interface{}
}

// IRobotMessage 机器人消息接口
// 定义了机器人消息的基本操作，用于创建结构化的通知
//
// 使用示例:
//
//	robotMsg := sylph.NewRobotMessage()
//	msgContent, err := robotMsg.Create(
//	    "系统警告",
//	    "数据库连接失败",
//	    map[string]interface{}{"severity": "high", "time": time.Now().String()}
//	)
//	if err == nil {
//	    robot.SendRaw(msgContent)
//	}
type IRobotMessage interface {
	// Create 创建消息内容
	// 参数:
	//   - title: 消息标题
	//   - content: 消息内容
	//   - fields: 附加字段信息
	//
	// 返回:
	//   - []byte: 序列化后的消息内容
	//   - error: 操作错误，如果为nil表示操作成功
	Create(title, content string, fields ...map[string]interface{}) ([]byte, error)
}

// IRobot 机器人接口
// 定义了机器人的发送消息能力，用于系统通知和报警
//
// 使用示例:
//
//	robot := sylph.GetRobot("slack")
//	err := robot.Send(
//	    "服务重启通知",
//	    "Web服务已成功重启",
//	    map[string]interface{}{"time": time.Now().Format(time.RFC3339)}
//	)
//	if err != nil {
//	    log.Printf("通知发送失败: %v", err)
//	}
type IRobot interface {
	// Send 发送消息
	// 参数:
	//   - title: 消息标题
	//   - content: 消息内容
	//   - fields: 可选的附加字段
	//
	// 返回:
	//   - error: 发送错误，如果为nil表示发送成功
	Send(title, content string, fields ...map[string]interface{}) error
}

// ILogger 日志记录器接口
// 定义了不同级别日志的记录能力，提供结构化日志支持
//
// 使用示例:
//
//	logger := ctx.Logger()
//	logger.Info(sylph.NewLoggerMessage("用户登录成功").WithField("user_id", userId))
//
//	if err != nil {
//	    logger.Error(sylph.NewLoggerMessage("操作失败").WithField("operation", "update"), err)
//	}
type ILogger interface {
	// 标准日志方法 - 结构化日志
	Info(message *LoggerMessage)
	Trace(message *LoggerMessage)
	Debug(message *LoggerMessage)
	Warn(message *LoggerMessage)
	Error(message *LoggerMessage, err error)
	Fatal(message *LoggerMessage)
	Panic(message *LoggerMessage)

	// 简化API - 格式化字符串
	Infof(format string, args ...interface{})
	Tracef(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(err error, format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panicf(format string, args ...interface{})

	// 资源管理
	Close() error
	IsClosed() bool

	// 上下文支持
	WithContext(ctx context.Context) ILogger
}

// RecoverFunc 定义恢复函数类型
// 用于处理panic时的恢复机制，保障系统稳定性
//
// 使用示例:
//
//	recover := func() {
//	    if r := recover(); r != nil {
//	        log.Printf("从panic中恢复: %v", r)
//	    }
//	}
//
//	go func() {
//	    defer recover()
//	    // 可能会panic的代码
//	}()
type RecoverFunc func()

// EventHandler 定义事件处理器函数类型
// 用于注册事件的处理逻辑，实现事件驱动架构
//
// 使用示例:
//
//	eventBus.Subscribe("user.created", func(ctx sylph.Context, payload interface{}) {
//	    user := payload.(User)
//	    ctx.Logger().Info(sylph.NewLoggerMessage("新用户创建").WithField("user_id", user.ID))
//	    // 处理用户创建事件
//	})
type EventHandler func(ctx Context, payload interface{})

// Handler 定义通用处理器函数类型
// 用于处理HTTP请求或其他类型的请求
//
// 使用示例:
//
//	router.GET("/api/users", func(ctx sylph.Context) {
//	    users, err := userService.GetAll()
//	    if err != nil {
//	        ctx.JSON(500, map[string]string{"error": err.Error()})
//	        return
//	    }
//	    ctx.JSON(200, users)
//	})
type Handler func(ctx Context)
