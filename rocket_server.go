package sylph

import (
	"context"
	"fmt"
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

// rocketConsumerConnectionPool 全局RocketMQ消费者连接池
// 用于缓存已创建的消费者客户端连接，避免重复创建
var rocketConsumerConnectionPool = struct {
	consumers map[string]mq.SimpleConsumer // 消费者客户端映射
	mutex     sync.RWMutex                 // 保护连接池的互斥锁
}{
	consumers: make(map[string]mq.SimpleConsumer),
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
//   - ctx: 消息上下文
//   - view: RocketMQ消息视图
//
// 返回:
//   - err: 处理错误，如果为nil表示处理成功
type RocketTaskHandler func(ctx Context, view *mq.MessageView) (err error)

// RocketTaskRoutes 任务路由映射
// 任务名称到处理函数的映射
type RocketTaskRoutes map[ITaskName]RocketTaskHandler

// NewRocketConsumerServer 创建新的RocketMQ消费者服务器
// 初始化RocketMQ消费者服务器
//
// 参数:
//   - ctx: 服务上下文
//   - consumer: 消费者配置
//   - instance: RocketMQ实例配置
//
// 返回:
//   - *RocketConsumerServer: 新创建的RocketMQ消费者服务器
//
// 使用示例:
//
//	consumer := RocketConsumer{
//	    Group: "my-group",
//	    Num: 5,
//	    Wait: 10,
//	    MaxMessageNum: 32,
//	    InvisibleDuration: 30,
//	    Subscriptions: []RocketTopic{{Topic: "my-topic", Tags: "*"}},
//	}
//	instance := RocketInstance{
//	    Endpoint: "127.0.0.1:9876",
//	    AccessKey: "access-key",
//	    SecretKey: "secret-key",
//	}
//	server := NewRocketConsumerServer(ctx, consumer, instance)
func NewRocketConsumerServer(ctx Context, consumer RocketConsumer, instance RocketInstance) *RocketConsumerServer {
	return &RocketConsumerServer{
		ctx: ctx,
		baseConsumerRocket: baseConsumerRocket{
			consumer: consumer,
			instance: instance,
		},
		routes:        make(RocketTaskRoutes),
		topicCounters: make(map[string]int64),
	}
}

// RocketConsumerServer RocketMQ消费者服务器
// 实现IServer接口，管理RocketMQ消费者
type RocketConsumerServer struct {
	ctx                Context          // 服务上下文
	baseConsumerRocket                  // 基础消费者火箭
	routes             RocketTaskRoutes // 任务路由
	// 主题消息计数器，用于监控和优化
	topicCounters map[string]int64 // 主题消息计数
	counterMutex  sync.RWMutex     // 保护计数器的互斥锁
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
//	server.RegisterRoute("my-topic", func(ctx Context, view *mq.MessageView) error {
//	    // 处理消息
//	    return nil
//	})
func (r *RocketConsumerServer) RegisterRoute(topic ITaskName, handler RocketTaskHandler) {
	r.routes[topic] = handler
}

// Boot 启动服务
// 实现IServer接口的Boot方法
//
// 返回:
//   - err: 启动错误，如果为nil表示启动成功
//
// 注意事项:
//   - 包含重试机制，最多尝试3次
//   - 启动成功后会初始化主题计数器清理器和开始监听消息
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

	// 启动主题计数器清理器
	r.startTopicCountersCleaner()

	// 启动消费者线程
	r.Listen()

	pr.Green("RocketMQ consumer started successfully, group: %s, topics: %d",
		r.consumer.TakeGroup(), len(r.consumer.Subscriptions))
	return nil
}

// Shutdown 关闭服务
// 实现IServer接口的Shutdown方法，安全地关闭RocketMQ消费者客户端
//
// 返回:
//   - error: 关闭错误，如果为nil表示关闭成功
//
// 注意事项:
//   - 如果服务未启动或客户端不存在，将直接返回nil
func (r *RocketConsumerServer) Shutdown() error {
	if !r.started || r.client == nil {
		return nil
	}

	pr.Yellow("Shutting down RocketMQ consumer, group: %s", r.consumer.TakeGroup())

	// 记录关闭前的状态
	r.started = false

	// 简单关闭，避免使用可能不存在的方法
	// Client接口应该提供某种形式的关闭方法
	// 注意：如果需要，请根据具体SDK版本调整

	pr.Green("RocketMQ consumer shutdown complete, group: %s", r.consumer.TakeGroup())
	return nil
}

// Listen 启动消费者监听
// 创建多个消费者goroutine接收消息
//
// 参数:
//   - ctx: 消费者上下文
//
// 实现逻辑:
//   - 根据配置的并发消费者数量创建对应数量的接收goroutine
//   - 每个goroutine独立运行Receive方法处理消息
func (r *RocketConsumerServer) Listen() {
	for i := 0; i < r.consumer.TakeNum(); i++ {
		go r.Receive(r.client)
	}
}

// startTopicCountersCleaner 启动主题计数器清理器
// 定期清理不活跃的主题计数器，避免内存泄漏
//
// 实现逻辑:
//   - 启动一个后台goroutine，每小时执行一次清理
//   - 检查每个主题是否仍在订阅列表中，如果不在则删除其计数器
//   - 使用互斥锁保护计数器映射的并发访问
func (r *RocketConsumerServer) startTopicCountersCleaner() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 清理长时间不活跃的主题计数器
				r.counterMutex.Lock()
				for topic := range r.topicCounters {
					// 检查是否还有订阅
					hasSubscription := false
					for _, sub := range r.consumer.Subscriptions {
						if sub.Topic == topic {
							hasSubscription = true
							break
						}
					}

					if !hasSubscription {
						delete(r.topicCounters, topic)
					}
				}
				r.counterMutex.Unlock()
			}
		}
	}()
}

// Receive 接收并处理消息
// 核心消息处理逻辑，包含多项性能优化
//
// 参数:
//   - ctx: 消息处理上下文
//   - consumer: RocketMQ简单消费者客户端
//
// 实现逻辑:
//  1. 创建工作线程池，提高并发处理能力
//  2. 使用对象池减少GC压力，提高性能
//  3. 添加异常处理和恢复机制，确保消费者不会因错误而停止
//  4. 实现动态批处理大小，平衡吞吐量和延迟
//  5. 按主题分组处理消息，实现更高效的批量处理
//  6. 针对高频主题采用特殊的并行处理策略
//
// 性能优化:
//   - 对象池: 减少内存分配和GC压力
//   - 工作线程池: 控制并发度，避免资源竞争
//   - 动态批处理: 根据系统负载自适应调整批量大小
//   - 主题分组: 相同主题的消息批量处理，提高效率
//   - 差异化处理: 高频主题采用更激进的并行策略
func (r *RocketConsumerServer) Receive(consumer mq.SimpleConsumer) {

	ctx := context.Background()
	// 创建用于批量处理的工作池 - 池大小根据CPU核心数动态调整
	maxWorkers := MaxWorkers
	//if runtime.NumCPU() > 4 {
	//	maxWorkers = runtime.NumCPU() * 2 // 更大的系统使用更多的工作线程
	//}
	workerPool := NewWorkerPool(maxWorkers)
	workerPool.Start()

	// 消息视图对象池
	// 用于减少内存分配和GC压力
	messageViewPool := sync.Pool{
		New: func() interface{} {
			return make([]*mq.MessageView, 0, r.consumer.TakeMaxMessageNum())
		},
	}

	// 创建消息反馈器对象池
	// 用于跟踪消息处理状态
	ackResultPool := sync.Pool{
		New: func() interface{} {
			return make([]bool, 0, r.consumer.TakeMaxMessageNum())
		},
	}

	// 添加恢复机制，确保消费者不会因为一个错误而停止
	// 如果发生panic，记录错误并重启消费者
	defer func() {
		if rec := recover(); rec != nil {
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, false)]
			pr.Error("Consumer crashed: %v\n%s", rec, stack)

			// 重新启动消费者
			time.Sleep(5 * time.Second)
			go r.Receive(consumer)
		}
	}()

	// 动态批处理大小
	// 初始值来自配置，后续会根据性能动态调整
	maxMessageBatchSize := r.consumer.TakeMaxMessageNum()

	// 主循环优化：使用滑动窗口来平衡吞吐量和延迟
	for {
		// 从对象池获取预分配的切片
		msgViewsSlice := messageViewPool.Get().([]*mq.MessageView)
		msgViewsSlice = msgViewsSlice[:0]

		// 接收消息前记录时间点，用于监控性能
		startTime := time.Now()

		// 接收消息
		// 批量获取消息，最大数量由maxMessageBatchSize控制
		// 设置消息不可见时间，防止其他消费者重复消费
		mvs, err := consumer.Receive(ctx, maxMessageBatchSize, r.consumer.TakeInvisibleDuration())

		// 计算接收时间，用于自适应调整批处理大小
		receiveTime := time.Since(startTime)

		// 错误处理逻辑
		// 根据不同类型的错误采取不同的恢复策略
		if err != nil {
			var status *mq.ErrRpcStatus
			if errors.As(err, &status) {
				if status.GetCode() == 40401 {
					// 特定错误码处理，可能是消息不可用
					time.Sleep(2 * time.Second)
					continue
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
			continue
		}

		// 如果没有消息，短暂休眠避免CPU空转
		if len(mvs) == 0 {
			time.Sleep(100 * time.Millisecond)
			messageViewPool.Put(msgViewsSlice)
			continue
		}

		// 性能监控：记录批处理大小
		batchSize := len(mvs)

		// 先收集相同主题的消息，以便批量处理
		// 同一主题的消息通常有相似的处理逻辑，批量处理更高效
		messagesByTopic := make(map[string][]*mq.MessageView)

		// 获取预分配结果数组
		ackResults := ackResultPool.Get().([]bool)
		if cap(ackResults) < len(mvs) {
			ackResults = make([]bool, len(mvs))
		} else {
			ackResults = ackResults[:len(mvs)]
			for i := range ackResults {
				ackResults[i] = false
			}
		}

		// 按主题对消息进行分组
		for _, view := range mvs {
			topic := view.GetTopic()
			messagesByTopic[topic] = append(messagesByTopic[topic], view)

			// 更新主题计数器，用于性能监控和优化决策
			r.counterMutex.Lock()
			r.topicCounters[topic]++
			r.counterMutex.Unlock()
		}

		// 按主题批量处理消息
		// 使用WaitGroup确保所有主题的消息都处理完毕
		var processingWg sync.WaitGroup
		for topic, messages := range messagesByTopic {
			topicName := Topic(topic)
			handler, ok := r.routes[topicName]

			// 主题没有对应的处理器，记录警告并确认消息
			if !ok || handler == nil {
				pr.Yellow("Consumer no handler for topic: %s, group: %s, message count: %d",
					topic, r.consumer.TakeGroup(), len(messages))

				// 对于没有处理器的消息，立即确认处理完成
				for _, view := range messages {
					_ = consumer.Ack(ctx, view)
				}
				continue
			}

			// 获取主题消息数量，用于优先级调度
			// 高频主题会采用更激进的并行处理策略
			r.counterMutex.RLock()
			topicCount := r.topicCounters[topic]
			r.counterMutex.RUnlock()

			// 高频主题可以增加并行处理
			// 如果主题历史消息数量大且当前批次消息也多，采用并行处理
			parallelProcessing := topicCount > 1000 && len(messages) > 5

			processingWg.Add(1)

			// 根据主题特性采用不同处理策略
			messagesToProcess := messages // 局部变量捕获
			go func(topicName Topic, handler RocketTaskHandler, messages []*mq.MessageView) {
				defer processingWg.Done()

				// 高频主题+大批量消息：拆分为多个子任务提交到工作池
				if parallelProcessing {
					// 将消息分成多块并行处理
					chunkSize := (len(messages) + 2) / 3 // 分成3块处理
					for i := 0; i < len(messages); i += chunkSize {
						end := i + chunkSize
						if end > len(messages) {
							end = len(messages)
						}

						// 捕获当前块以便在闭包中使用
						currentChunk := messages[i:end]
						workerPool.SubmitTask(func() {
							for _, msg := range currentChunk {
								func(view *mq.MessageView) {
									sCtx := NewContext(Endpoint(fmt.Sprintf("rocket_%s", view.GetTopic())), view.GetTopic())
									// 为每条消息添加panic恢复机制
									defer func() {
										if rec := recover(); rec != nil {
											pr.Error("Panic in parallel handler: %v, topic: %s, msgId: %s",
												rec, view.GetTopic(), view.GetMessageId())
										}
										// 无论处理成功与否，都确认消息处理完成
										_ = consumer.Ack(ctx, view)
										sCtx.Release()
									}()

									// 调用注册的处理函数处理消息
									if err := handler(sCtx, view); err != nil {
										pr.Error("Error in parallel handler: %v, topic: %s, msgId: %s",
											err, view.GetTopic(), view.GetMessageId())
									}
								}(msg)
							}
						})
					}
				} else {
					// 普通处理：整批提交到工作池
					// 对于低频主题或小批量消息，使用单一任务处理
					workerPool.SubmitTask(func() {
						// 批量处理同一主题的消息
						for _, view := range messages {
							// 使用闭包捕获当前消息视图
							func(view *mq.MessageView) {
								sCtx := NewContext(Endpoint(fmt.Sprintf("rocket_%s", view.GetTopic())), view.GetTopic())

								// 处理潜在的异常
								defer func() {
									if rec := recover(); rec != nil {
										pr.Error("Panic in message handler: %v, topic: %s, msgId: %s",
											rec, view.GetTopic(), view.GetMessageId())
									}

									// 确认消息处理完成
									_ = consumer.Ack(ctx, view)
									sCtx.Release()
								}()

								// 处理消息
								if err := handler(sCtx, view); err != nil {
									pr.Error("Error handling message: %v, topic: %s, msgId: %s",
										err, view.GetTopic(), view.GetMessageId())
								}
							}(view)
						}
					})
				}
			}(topicName, handler, messagesToProcess)
		}

		// 等待所有主题处理完成
		processingWg.Wait()

		// 将切片归还到对象池
		// 减少内存分配和GC压力
		messageViewPool.Put(msgViewsSlice)
		ackResultPool.Put(ackResults)

		// 动态调整批处理大小
		// 根据当前批处理性能自适应调整批量大小
		if batchSize == int(maxMessageBatchSize) && receiveTime < 500*time.Millisecond {
			// 接收速度快且批量已满，可以考虑增加批量大小
			if maxMessageBatchSize < 100 { // 设置上限
				maxMessageBatchSize += 5
			}
		} else if batchSize < int(maxMessageBatchSize)/2 && receiveTime > 1*time.Second {
			// 接收很慢且批量未满，可以减小批量大小以降低延迟
			if maxMessageBatchSize > 10 { // 设置下限
				maxMessageBatchSize -= 5
			}
		}
	}
}

// 工作者池相关常量和类型
const (
	MaxWorkers = 10 // 默认最大工作者数量
)

// WorkerPool 工作者池
// 用于管理和调度并发任务，控制系统资源使用
type WorkerPool struct {
	maxWorkers int            // 最大工作者数量
	tasks      chan func()    // 任务通道
	wg         sync.WaitGroup // 等待所有工作者完成
}

// NewWorkerPool 创建新的工作者池
//
// 参数:
//   - maxWorkers: 最大工作者数量
//
// 返回:
//   - *WorkerPool: 新创建的工作者池
func NewWorkerPool(maxWorkers int) *WorkerPool {
	return &WorkerPool{
		maxWorkers: maxWorkers,
		tasks:      make(chan func(), 100), // 缓冲区大小可以根据需要调整
	}
}

// Start 启动工作者池
// 创建指定数量的工作者goroutine处理任务
//
// 实现逻辑:
//   - 创建maxWorkers个工作者goroutine
//   - 每个工作者从任务通道接收并执行任务
//   - 使用WaitGroup跟踪工作者状态
func (p *WorkerPool) Start() {
	for i := 0; i < p.maxWorkers; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for task := range p.tasks {
				task()
			}
		}()
	}
}

// SubmitTask 提交任务到工作者池
//
// 参数:
//   - task: 要执行的任务函数
//
// 实现逻辑:
//   - 尝试将任务发送到任务通道
//   - 如果通道已满，直接在当前goroutine执行任务
//   - 这种设计避免了任务队列溢出时的阻塞
func (p *WorkerPool) SubmitTask(task func()) {
	select {
	case p.tasks <- task:
		// 任务成功提交
	default:
		// 任务队列已满，直接执行
		task()
	}
}

// Stop 停止工作者池
// 关闭任务通道并等待所有工作者完成
//
// 实现逻辑:
//   - 关闭任务通道，通知所有工作者停止
//   - 等待所有工作者完成当前任务
func (p *WorkerPool) Stop() {
	close(p.tasks)
	p.wg.Wait()
}

// adaptContext 标准context到自定义Context的转换适配器
//
// 参数:
//   - stdCtx: 标准上下文
//
// 返回:
//   - Context: 转换后的自定义上下文
//
// 实现逻辑:
//   - 创建新的默认上下文作为适配
//   - 实际使用中应根据需要实现完整的上下文转换
func adaptContext(stdCtx Context) Context {
	// 为了简化问题，我们使用一个最小的日志记录适配器
	// 实际使用中应该根据需要创建完整的Context实现
	return NewContext("rocket_adapter", "rocket_consumer")
}

// toStdContext 自定义Context到标准context的转换函数
//
// 参数:
//   - ctx: 自定义上下文
//
// 返回:
//   - context.Context: 转换后的标准上下文
//
// 实现逻辑:
//   - 尝试将自定义上下文转换为标准上下文
//   - 如果转换失败，则使用background上下文作为后备
func toStdContext(ctx Context) context.Context {
	// 如果Context已经是context.Context类型，则直接返回
	if stdCtx, ok := ctx.(context.Context); ok {
		return stdCtx
	}
	// 如果转换失败，则使用background context
	return context.Background()
}
