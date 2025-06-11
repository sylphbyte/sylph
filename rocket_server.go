package sylph

import (
	"context"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sylphbyte/pr"

	mq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/pkg/errors"
)

// 静态断言确保RocketConsumerServer实现了IServer接口
var _ IServer = (*RocketConsumerServer)(nil)

// init 初始化函数
// 在包导入时自动重置RocketMQ的配置
func init() {
	ResetRocketConfig()
}

// ResetRocketConfig 重置RocketMQ配置
// 设置环境变量以配置RocketMQ客户端的日志行为
//
// 注意事项:
//   - 禁用控制台日志输出
//   - 设置日志级别为error
//   - 设置日志文件目录为./logs
//   - 设置日志文件名为rocket_system.log
func ResetRocketConfig() {
	_ = os.Setenv(mq.ENABLE_CONSOLE_APPENDER, "false")
	_ = os.Setenv(mq.CLIENT_LOG_LEVEL, "error")
	_ = os.Setenv(mq.CLIENT_LOG_ROOT, "./logs")
	_ = os.Setenv(mq.CLIENT_LOG_FILENAME, "rocket_system.log")
	//
	//CLIENT_LOG_ROOT     = "rocketmq.client.logRoot"
	//CLIENT_LOG_MAXINDEX = "rocketmq.client.logFileMaxIndex"
	//CLIENT_LOG_FILESIZE = "rocketmq.client.logFileMaxSize"
	//CLIENT_LOG_LEVEL    = "rocketmq.client.logLevel"
	//// CLIENT_LOG_ADDITIVE        = "rocketmq.client.log.additive"
	//CLIENT_LOG_FILENAME = "rocketmq.client.logFileName"
	//// CLIENT_LOG_ASYNC_QUEUESIZE = "rocketmq.client.logAsyncQueueSize"
	//ENABLE_CONSOLE_APPENDER = "mq.consoleAppender.enabled"

	mq.ResetLogger()
}

// RocketConsumers RocketMQ消费者配置列表
// 用于配置多组消费者
type RocketConsumers []RocketConsumer

// RocketConsumer RocketMQ消费者配置
// 定义消费者的属性和行为
type RocketConsumer struct {
	Group             string       `yaml:"group" mapstructure:"group"`                           // 消费者组名
	Num               int          `yaml:"num" mapstructure:"num"`                               // 并发消费者数量
	Wait              int          `yaml:"wait" mapstructure:"wait"`                             // 等待时间(秒)
	MaxMessageNum     int32        `yaml:"max_message_num" mapstructure:"max_message_num"`       // 最大消息数
	InvisibleDuration int          `yaml:"invisible_duration" mapstructure:"invisible_duration"` // 消息不可见时间(秒)
	Subscriptions     RocketTopics `yaml:"subscriptions" mapstructure:"subscriptions"`           // 订阅的主题列表
	UseGroup          bool         `yaml:"use_group" mapstructure:"use_group"`                   // 是否使用组名作为主题名
	MaxGoroutines     int          `yaml:"max_goroutines" mapstructure:"max_goroutines"`         // 最大并发处理goroutine数，默认1000
	ShutdownTimeout   int          `yaml:"shutdown_timeout" mapstructure:"shutdown_timeout"`     // 关闭超时时间(秒)，默认30秒
	HandlerTimeout    int          `yaml:"handler_timeout" mapstructure:"handler_timeout"`       // 消息处理超时时间(秒)，默认30秒
}

// TakeGroup 获取消费者组名
//
// 返回:
//   - string: 消费者组名
func (r RocketConsumer) TakeGroup() string {
	return r.Group
}

// makeSubscriptionExpressions 创建订阅表达式
// 为每个主题创建过滤表达式
//
// 返回:
//   - expressions: 主题到过滤表达式的映射
//
// 注意事项:
//   - 如果标签为空，将使用"*"表示匹配所有标签
func (r RocketConsumer) makeSubscriptionExpressions() (expressions map[string]*mq.FilterExpression) {
	expressions = make(map[string]*mq.FilterExpression)
	for _, subscription := range r.Subscriptions {
		if strings.Trim(subscription.Tags, " ") == "" {
			subscription.Tags = "*"
		}

		expressions[subscription.Topic] = mq.NewFilterExpression(subscription.Tags)
	}

	return
}

// TakeOptions 获取消费者选项
// 创建RocketMQ客户端配置选项
//
// 返回:
//   - []mq.SimpleConsumerOption: 客户端配置选项列表
func (r RocketConsumer) TakeOptions() []mq.SimpleConsumerOption {
	return []mq.SimpleConsumerOption{
		mq.WithAwaitDuration(r.TakeWait()),
		mq.WithSubscriptionExpressions(r.makeSubscriptionExpressions()),
	}
}

// TakeNum 获取并发消费者数量
//
// 返回:
//   - int: 并发消费者数量
func (r RocketConsumer) TakeNum() int {
	return r.Num
}

// TakeWait 获取等待时间
//
// 返回:
//   - time.Duration: 等待时间
func (r RocketConsumer) TakeWait() time.Duration {
	return time.Duration(r.Wait) * time.Second
}

// TakeMaxMessageNum 获取最大消息数
//
// 返回:
//   - int32: 最大消息数
func (r RocketConsumer) TakeMaxMessageNum() int32 {
	return r.MaxMessageNum
}

// TakeInvisibleDuration 获取消息不可见时间
//
// 返回:
//   - time.Duration: 消息不可见时间
func (r RocketConsumer) TakeInvisibleDuration() time.Duration {
	return time.Duration(r.InvisibleDuration) * time.Second
}

// TakeMaxGoroutines 获取最大goroutine数
// 如果未设置，默认为1000
//
// 返回:
//   - int: 最大goroutine数
func (r RocketConsumer) TakeMaxGoroutines() int {
	if r.MaxGoroutines <= 0 {
		return 1000 // 默认最大1000个并发goroutine
	}
	return r.MaxGoroutines
}

// TakeShutdownTimeout 获取关闭超时时间
// 如果未设置，默认为30秒
//
// 返回:
//   - time.Duration: 关闭超时时间
func (r RocketConsumer) TakeShutdownTimeout() time.Duration {
	if r.ShutdownTimeout <= 0 {
		return 30 * time.Second // 默认30秒
	}
	return time.Duration(r.ShutdownTimeout) * time.Second
}

// TakeHandlerTimeout 获取消息处理超时时间
// 如果未设置，默认为30秒
//
// 返回:
//   - time.Duration: 消息处理超时时间
func (r RocketConsumer) TakeHandlerTimeout() time.Duration {
	if r.HandlerTimeout <= 0 {
		return 30 * time.Second // 默认30秒
	}
	return time.Duration(r.HandlerTimeout) * time.Second
}

// NewMessage 创建新的消息对象
// 封装RocketMQ消息和上下文
//
// 参数:
//   - ctx: 消息上下文
//   - view: RocketMQ消息视图
//
// 返回:
//   - *Message: 新创建的消息对象
func NewMessage(ctx Context, view *mq.MessageView) *Message {
	return &Message{
		ctx:  ctx,
		view: view,
	}
}

// Message 消息对象
// 封装消息上下文和RocketMQ消息视图
type Message struct {
	ctx  Context         // 消息上下文
	view *mq.MessageView // RocketMQ消息视图
}

// Context 获取消息上下文
//
// 返回:
//   - Context: 消息上下文
func (m *Message) Context() Context {
	return m.ctx
}

// View 获取RocketMQ消息视图
//
// 返回:
//   - *mq.MessageView: RocketMQ消息视图
func (m *Message) View() *mq.MessageView {
	return m.view
}

// RocketTaskHandler 消息处理函数类型
// 处理接收到的消息
//
// 参数:
//   - view: RocketMQ消息视图，包含所有需要的消息信息
//   - ack: 消息确认函数，处理成功后调用以确认消息
//
// 返回:
//   - err: 处理错误，如果为nil表示处理成功
//
// 注意:
//   - handler内部可以通过sylph.NewContext()自行创建所需的上下文
//   - handler自主决定何时调用ack()确认消息，支持异步处理
//   - 如果处理失败，建议不调用ack()让消息重新投递
type RocketTaskHandler func(view *mq.MessageView, ack func())

// RocketTaskRoutes 任务路由映射
// 任务名称到处理函数的映射
type RocketTaskRoutes map[string]RocketTaskHandler

// NewRocketConsumerServer 创建新的RocketMQ消费者服务器
// 初始化RocketMQ消费者服务器
//
// 参数:
//   - consumer: 消费者配置
//   - instance: RocketMQ实例配置
//
// 返回:
//   - *RocketConsumerServer: 新创建的RocketMQ消费者服务器
//
// 使用示例:
//
//		consumer := RocketConsumer{
//		    Group: "my-group",
//		    Num: 5,
//		    Wait: 10,
//		    MaxMessageNum: 32,
//		    InvisibleDuration: 30,
//	     	UseGroup: true,             // 使用组进行函数标记
//		    MaxGoroutines: 1000,        // 最大并发goroutine数
//		    ShutdownTimeout: 30,        // 关闭超时时间(秒)
//		    HandlerTimeout: 30,         // 消息处理超时时间(秒)
//		    Subscriptions: []RocketTopic{{Topic: "my-topic", Tags: "*"}},
//		}
//		instance := RocketInstance{
//		    Endpoint: "127.0.0.1:9876",
//		    AccessKey: "access-key",
//		    SecretKey: "secret-key",
//		}
//		server := NewRocketConsumerServer(consumer, instance)
func NewRocketConsumerServer(consumer RocketConsumer, instance RocketInstance) *RocketConsumerServer {
	server := &RocketConsumerServer{
		baseConsumerRocket: baseConsumerRocket{
			consumer: consumer,
			instance: instance,
		},
		routes:      make(RocketTaskRoutes),
		semaphore:   make(chan struct{}, consumer.TakeMaxGoroutines()),
		ctx:         context.Background(),
		shutdown:    make(chan struct{}),
		workerGroup: sync.WaitGroup{},
	}

	server.ctx, server.cancel = context.WithCancel(server.ctx)
	return server
}

// RocketConsumerServer RocketMQ消费者服务器
// 实现IServer接口，管理RocketMQ消费者
type RocketConsumerServer struct {
	baseConsumerRocket                  // 基础消费者rocket
	routes             RocketTaskRoutes // 任务路由

	// 并发控制
	semaphore   chan struct{} // 控制最大并发goroutine数
	activeCount int64         // 当前活跃的goroutine数量

	// 生命周期管理
	ctx         context.Context    // 服务上下文
	cancel      context.CancelFunc // 取消函数
	shutdown    chan struct{}      // 关闭信号
	workerGroup sync.WaitGroup     // 等待所有worker完成
}

// Name 获取服务名称
// 实现IServer接口的Name方法
//
// 返回:
//   - string: 服务名称，即消费者组名
func (r *RocketConsumerServer) Name() string {
	return r.consumer.Group
}

// RegisterRoute 注册任务路由
// 将任务名称与处理函数关联
//
// 参数:
//   - topic: 任务名称，通常是主题名
//   - handler: 消息处理函数
//
// 使用示例:
//
//	server.RegisterRoute("my-topic", func(view *mq.MessageView, ack func()) {
//	    // 处理消息
//	    ack() // 处理成功后确认消息
//	})
func (r *RocketConsumerServer) RegisterRoute(topic ITaskName, handler RocketTaskHandler) {
	r.routes[topic.Name()] = handler
}

// Boot 启动服务
// 实现IServer接口的Boot方法
//
// 返回:
//   - err: 启动错误，如果为nil表示启动成功
//
// 注意事项:
//   - 包含重试机制，最多尝试3次
//   - 启动成功后开始监听消息
func (r *RocketConsumerServer) Boot() (err error) {
	// 添加重试机制
	var retryCount int
	for retryCount = 0; retryCount < 3; retryCount++ {
		if err = r.makeClient(); err == nil {
			break
		}

		if retryCount < 2 {
			pr.Yellow("Failed to create RocketMQ consumer client, retrying... (%d/3)", retryCount+1)
			time.Sleep(time.Duration(retryCount+1) * time.Second)
		}
	}

	if err != nil {
		return errors.Wrap(err, "failed to create consumer client after 3 attempts")
	}

	if r.started {
		return nil
	}

	// 启动消费者客户端
	if err = r.client.Start(); err != nil {
		return errors.Wrap(err, "failed to start consumer client")
	}

	r.started = true

	// 启动消费者线程
	r.Listen()

	pr.Green("RocketMQ consumer started successfully, group: %s, topics: %d, max goroutines: %d",
		r.consumer.TakeGroup(), len(r.consumer.Subscriptions), r.consumer.TakeMaxGoroutines())
	return nil
}

// Shutdown 关闭服务
// 实现IServer接口的Shutdown方法，安全地关闭RocketMQ消费者客户端
//
// 返回:
//   - error: 关闭错误，如果为nil表示关闭成功
//
// 注意事项:
//   - 优雅关闭，等待所有goroutine完成
//   - 有超时机制防止无限等待
func (r *RocketConsumerServer) Shutdown() error {
	if !r.started || r.client == nil {
		return nil
	}

	pr.Yellow("Shutting down RocketMQ consumer, group: %s", r.consumer.TakeGroup())

	// 标记停止状态
	r.started = false

	// 发送关闭信号
	close(r.shutdown)

	// 取消上下文，通知所有goroutine停止
	if r.cancel != nil {
		r.cancel()
	}

	// 等待所有worker完成，设置超时
	done := make(chan struct{})
	go func() {
		r.workerGroup.Wait()
		close(done)
	}()

	select {
	case <-done:
		pr.Green("All workers shut down gracefully")
	case <-time.After(r.consumer.TakeShutdownTimeout()):
		pr.Yellow("Shutdown timeout (%v), some workers may still be running", r.consumer.TakeShutdownTimeout())
	}

	pr.Green("RocketMQ consumer shutdown complete, group: %s", r.consumer.TakeGroup())
	return nil
}

// Listen 启动消费者监听
// 创建多个消费者goroutine接收消息，带有安全的重启机制
//
// 实现逻辑:
//   - 根据配置的并发消费者数量创建对应数量的接收goroutine
//   - 每个goroutine独立运行Receive方法处理消息
//   - 如果goroutine异常退出，会自动重启（有重试限制）
func (r *RocketConsumerServer) Listen() {
	for i := 0; i < r.consumer.TakeNum(); i++ {
		r.workerGroup.Add(1)
		go r.receiveWithRestart(i)
	}
}

// receiveWithRestart 带重启机制的接收方法
// 为每个消费者goroutine提供安全的重启逻辑
func (r *RocketConsumerServer) receiveWithRestart(workerID int) {
	defer r.workerGroup.Done()

	maxRetries := 5 // 最大重试次数
	retryCount := 0

	for {
		// 检查服务器状态和上下文
		select {
		case <-r.ctx.Done():
			pr.Info("Worker %d exiting, context cancelled", workerID)
			return
		case <-r.shutdown:
			pr.Info("Worker %d exiting, server is shutting down", workerID)
			return
		default:
		}

		if !r.started {
			pr.Info("Worker %d exiting, server stopped", workerID)
			return
		}

		// 检查重试次数
		if retryCount >= maxRetries {
			pr.Error("Worker %d exceeded max retries (%d), exiting", workerID, maxRetries)
			return
		}

		// 启动接收循环
		r.Receive(r.client)

		// 如果到达这里，说明Receive异常退出了
		retryCount++
		waitTime := time.Duration(retryCount) * 5 * time.Second // 递增等待时间

		pr.Yellow("Worker %d crashed, retry %d/%d after %v", workerID, retryCount, maxRetries, waitTime)

		// 可中断的等待
		select {
		case <-time.After(waitTime):
		case <-r.ctx.Done():
			return
		case <-r.shutdown:
			return
		}
	}
}

// Receive 接收并处理消息
// 优化的消息处理逻辑，使用有限制的goroutine处理消息
//
// 参数:
//   - consumer: RocketMQ简单消费者客户端
//
// 实现逻辑:
//   - 批量接收消息
//   - 使用semaphore控制并发goroutine数量，防止内存溢出
//   - 支持上下文取消和优雅关闭
//   - 安全的panic恢复机制
func (r *RocketConsumerServer) Receive(consumer mq.SimpleConsumer) {
	// 安全的恢复机制，记录错误但不重启，避免死循环
	defer func() {
		if rec := recover(); rec != nil {
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, false)]
			pr.Error("Consumer crashed and will exit: %v\n%s", rec, stack)
		}
	}()

	for {
		// 检查服务器状态和上下文
		select {
		case <-r.ctx.Done():
			pr.Info("Consumer receive loop exiting, context cancelled")
			return
		case <-r.shutdown:
			pr.Info("Consumer receive loop exiting, server is shutting down")
			return
		default:
		}

		if !r.started {
			pr.Info("Consumer receive loop exiting, server stopped")
			return
		}

		// 批量接收消息，使用服务的上下文
		mvs, err := consumer.Receive(r.ctx, r.consumer.TakeMaxMessageNum(), r.consumer.TakeInvisibleDuration())

		if err != nil {
			// 检查是否是因为上下文取消导致的错误
			if r.ctx.Err() != nil {
				pr.Info("Receive cancelled due to context cancellation")
				return
			}
			r.handleReceiveError(err)
			continue
		}

		// 如果没有消息，短暂休眠避免CPU空转
		if len(mvs) == 0 {
			select {
			case <-time.After(100 * time.Millisecond):
			case <-r.ctx.Done():
				return
			case <-r.shutdown:
				return
			}
			continue
		}

		// 为每条消息启动受限制的goroutine处理
		for _, view := range mvs {
			// 使用非阻塞方式尝试获取semaphore
			select {
			case r.semaphore <- struct{}{}:
				// 获取到许可，启动goroutine处理消息
				go r.handleMessage(r.ctx, consumer, view)
			case <-r.ctx.Done():
				// 上下文被取消，停止处理新消息
				return
			case <-r.shutdown:
				// 收到关闭信号，停止处理新消息
				return
			default:
				// 无法获取许可，说明已达到最大并发数
				// 记录警告并跳过此消息（实际应用中可能需要重新投递或其他策略）
				pr.Yellow("Max goroutines reached (%d), skipping message from topic: %s",
					r.consumer.TakeMaxGoroutines(), view.GetTopic())
			}
		}
	}
}

// handleReceiveError 处理接收消息时的错误
// 根据错误类型采用不同的恢复策略
func (r *RocketConsumerServer) handleReceiveError(err error) {
	var status *mq.ErrRpcStatus
	if errors.As(err, &status) {
		if status.GetCode() == 40401 {
			// 特定错误码处理，可能是消息不可用
			time.Sleep(2 * time.Second)
			return
		}
	}

	// 错误回退策略：根据错误类型调整等待时间
	var waitTime time.Duration = 2 * time.Second
	if strings.Contains(err.Error(), "timeout") {
		waitTime = 5 * time.Second
	} else if strings.Contains(err.Error(), "connection") {
		waitTime = 10 * time.Second
	}

	pr.Error("Failed to receive messages: %v, will retry in %v", err, waitTime)
	time.Sleep(waitTime)
}

// handleMessage 处理单条消息
// 优化的消息处理逻辑，带资源管理和并发控制
func (r *RocketConsumerServer) handleMessage(ctx context.Context, consumer mq.SimpleConsumer, view *mq.MessageView) {
	// 释放semaphore许可
	defer func() {
		<-r.semaphore
	}()

	// panic恢复机制
	defer func() {
		if rec := recover(); rec != nil {
			pr.Error("Panic handling message: %v, topic: %s, msgId: %s",
				rec, view.GetTopic(), view.GetMessageId())
			// panic时自动ack，避免消息无限重试
			_ = consumer.Ack(ctx, view)
		}
	}()

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		pr.Info("Message handler cancelled, topic: %s, msgId: %s", view.GetTopic(), view.GetMessageId())
		return
	default:
	}

	// 获取主题名称
	var topicName Topic
	if r.consumer.UseGroup {
		topicName = Topic(r.consumer.TakeGroup())
	} else {
		topicName = Topic(view.GetTopic())
	}

	// 查找处理器
	handler, ok := r.routes[topicName.Name()]
	if !ok || handler == nil {
		pr.Yellow("Consumer no handler for topic: %s, group: %s, msgId: %s",
			view.GetTopic(), r.consumer.TakeGroup(), view.GetMessageId())
		// 没有处理器，不ack消息，让其重新投递或被其他消费者处理
		return
	}

	// 创建带超时保护的ack函数，确保资源不会泄露
	var ackCalled bool
	ackHandle := func() {
		// 防止重复ack
		if !ackCalled {
			ackCalled = true
			_ = consumer.Ack(ctx, view)
		}
	}

	// 在goroutine中执行handler，支持超时控制
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler(view, ackHandle)
	}()

	// 等待处理完成或超时
	handlerTimeout := r.consumer.TakeHandlerTimeout()
	select {
	case <-done:
		// 处理完成
	case <-ctx.Done():
		// 上下文取消
		pr.Info("Message handler context cancelled, topic: %s, msgId: %s", view.GetTopic(), view.GetMessageId())
	case <-time.After(handlerTimeout):
		// 处理超时，记录警告但不强制ack，让handler自行决定
		pr.Yellow("Message handler timeout (%v), topic: %s, msgId: %s", handlerTimeout, view.GetTopic(), view.GetMessageId())
	}
}
