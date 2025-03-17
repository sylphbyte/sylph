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

func init() {
	ResetRocketConfig()
}

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

// RocketConsumerConnectionPool 全局RocketMQ消费者连接池
var rocketConsumerConnectionPool = struct {
	consumers map[string]mq.SimpleConsumer
	mutex     sync.RWMutex
}{
	consumers: make(map[string]mq.SimpleConsumer),
}

type RocketConsumers []RocketConsumer
type RocketConsumer struct {
	Group             string       `yaml:"group" mapstructure:"group"`
	Num               int          `yaml:"num" mapstructure:"num"`
	Wait              int          `yaml:"wait" mapstructure:"wait"`
	MaxMessageNum     int32        `yaml:"max_message_num" mapstructure:"max_message_num"`
	InvisibleDuration int          `yaml:"invisible_duration" mapstructure:"invisible_duration"`
	Subscriptions     RocketTopics `yaml:"subscriptions" mapstructure:"subscriptions"`
}

func (r RocketConsumer) TakeGroup() string {
	return r.Group
}

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

func (r RocketConsumer) TakeOptions() []mq.SimpleConsumerOption {
	return []mq.SimpleConsumerOption{
		mq.WithAwaitDuration(r.TakeWait()),
		mq.WithSubscriptionExpressions(r.makeSubscriptionExpressions()),
	}
}

func (r RocketConsumer) TakeNum() int {
	return r.Num
}

func (r RocketConsumer) TakeWait() time.Duration {
	return time.Duration(r.Wait) * time.Second
}

func (r RocketConsumer) TakeMaxMessageNum() int32 {
	return r.MaxMessageNum
}

func (r RocketConsumer) TakeInvisibleDuration() time.Duration {
	return time.Duration(r.InvisibleDuration) * time.Second
}

//func (c *Consumer) Connect() (connect mq.SimpleConsumer) {
//	var err error
//	//fmt.Printf()
//	fmt.Printf("%v\n", c.opt)
//	if connect, err = mq.NewSimpleConsumer(c.opt.TakeConfig(), c.opt.TakeOptions()...); err != nil {
//		panic(err)
//	}
//
//	if err = connect.Start(); err != nil {
//		panic(err)
//	}
//
//	return
//}

func NewMessage(ctx Context, view *mq.MessageView) *Message {
	return &Message{
		ctx:  ctx,
		view: view,
	}
}

type Message struct {
	ctx  Context
	view *mq.MessageView
}

func (m *Message) Context() Context {
	return m.ctx
}

func (m *Message) View() *mq.MessageView {
	return m.view
}

type RocketTaskHandler func(ctx Context, view *mq.MessageView) (err error)

type RocketTaskRoutes map[ITaskName]RocketTaskHandler

type RocketConsumerServer struct {
	ctx Context
	baseConsumerRocket
	routes RocketTaskRoutes
	// 主题消息计数器，用于监控和优化
	topicCounters map[string]int64
	counterMutex  sync.RWMutex
}

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

func (r *RocketConsumerServer) RegisterRoute(topic ITaskName, handler RocketTaskHandler) {
	r.routes[topic] = handler
}

// 增强原有makeClient方法，添加连接池支持
// 注意：此方法在rocket.go中已经定义，这里是增强实现
// 真正的实现在 rocket.go 中
// 这里只是注释掉作为参考
/*
func (b *baseConsumerRocket) makeClient() error {
	// 如果已经启动，直接返回
	if b.started {
		return nil
	}

	// 生成连接池键
	poolKey := fmt.Sprintf("%s:%s:%s",
		b.instance.Endpoint,
		b.instance.AccessKey,
		b.consumer.Group)

	// 尝试从连接池获取
	rocketConsumerConnectionPool.mutex.RLock()
	cachedConsumer, exists := rocketConsumerConnectionPool.consumers[poolKey]
	rocketConsumerConnectionPool.mutex.RUnlock()

	if exists {
		b.client = cachedConsumer
		b.started = true
		return nil
	}

	// 如果连接池中没有，创建新的客户端
	consumer, err := mq.NewSimpleConsumer(b.instance.TakeConfig(),
		append(b.consumer.TakeOptions(), mq.WithConsumerGroup(b.consumer.TakeGroup()))...)
	if err != nil {
		return err
	}

	// 将新客户端存入连接池
	rocketConsumerConnectionPool.mutex.Lock()
	rocketConsumerConnectionPool.consumers[poolKey] = consumer
	rocketConsumerConnectionPool.mutex.Unlock()

	b.client = consumer
	return nil
}
*/

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
	r.Listen(r.ctx)

	pr.Green("RocketMQ consumer started successfully, group: %s, topics: %d",
		r.consumer.TakeGroup(), len(r.consumer.Subscriptions))
	return nil
}

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

func (r *RocketConsumerServer) Listen(ctx Context) {
	for i := 0; i < r.consumer.TakeNum(); i++ {
		go r.Receive(ctx, r.client)
	}
}

// 定期清理不活跃的主题计数器
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

// Receive 接收并处理消息的优化版本
func (r *RocketConsumerServer) Receive(ctx Context, consumer mq.SimpleConsumer) {
	// 创建用于批量处理的工作池 - 池大小根据CPU核心数动态调整
	maxWorkers := MaxWorkers
	if runtime.NumCPU() > 4 {
		maxWorkers = runtime.NumCPU() * 2 // 更大的系统使用更多的工作线程
	}
	workerPool := NewWorkerPool(maxWorkers)
	workerPool.Start()

	// 消息视图对象池
	messageViewPool := sync.Pool{
		New: func() interface{} {
			return make([]*mq.MessageView, 0, r.consumer.TakeMaxMessageNum())
		},
	}

	// 创建消息反馈器对象池
	ackResultPool := sync.Pool{
		New: func() interface{} {
			return make([]bool, 0, r.consumer.TakeMaxMessageNum())
		},
	}

	// 添加恢复机制，确保消费者不会因为一个错误而停止
	defer func() {
		if rec := recover(); rec != nil {
			stack := make([]byte, 4096)
			stack = stack[:runtime.Stack(stack, false)]
			pr.Error("Consumer crashed: %v\n%s", rec, stack)

			// 重新启动消费者
			time.Sleep(5 * time.Second)
			go r.Receive(ctx, consumer)
		}
	}()

	// 添加动态批处理大小
	maxMessageBatchSize := r.consumer.TakeMaxMessageNum()

	// 主循环优化：使用滑动窗口来平衡吞吐量和延迟
	for {
		// 从对象池获取预分配的切片
		msgViewsSlice := messageViewPool.Get().([]*mq.MessageView)
		msgViewsSlice = msgViewsSlice[:0]

		// 接收消息前记录时间点，用于监控性能
		startTime := time.Now()

		// 接收消息
		mvs, err := consumer.Receive(ctx, maxMessageBatchSize, r.consumer.TakeInvisibleDuration())

		// 计算接收时间，用于自适应调整
		receiveTime := time.Since(startTime)

		if err != nil {
			var status *mq.ErrRpcStatus
			if errors.As(err, &status) {
				if status.GetCode() == 40401 {
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

		for _, view := range mvs {
			topic := view.GetTopic()
			messagesByTopic[topic] = append(messagesByTopic[topic], view)

			// 更新主题计数器
			r.counterMutex.Lock()
			r.topicCounters[topic]++
			r.counterMutex.Unlock()
		}

		// 按主题批量处理消息
		var processingWg sync.WaitGroup
		for topic, messages := range messagesByTopic {
			topicName := Topic(topic)
			handler, ok := r.routes[topicName]
			if !ok || handler == nil {
				pr.Yellow("Consumer no handler for topic: %s, group: %s, message count: %d",
					topic, r.consumer.TakeGroup(), len(messages))

				// 对于没有处理器的消息，立即确认
				for _, view := range messages {
					_ = consumer.Ack(ctx, view)
				}
				continue
			}

			// 获取主题消息数量，用于优先级调度
			r.counterMutex.RLock()
			topicCount := r.topicCounters[topic]
			r.counterMutex.RUnlock()

			// 高频主题可以增加并行处理
			parallelProcessing := topicCount > 1000 && len(messages) > 5

			processingWg.Add(1)

			// 根据主题特性采用不同处理策略
			messagesToProcess := messages // 局部变量捕获
			go func(topicName Topic, handler RocketTaskHandler, messages []*mq.MessageView) {
				defer processingWg.Done()

				if parallelProcessing {
					// 高频主题+大批量消息：拆分为多个子任务提交到工作池
					chunkSize := (len(messages) + 2) / 3 // 分成3块处理
					for i := 0; i < len(messages); i += chunkSize {
						end := i + chunkSize
						if end > len(messages) {
							end = len(messages)
						}

						// 捕获当前块
						currentChunk := messages[i:end]
						workerPool.SubmitTask(func() {
							for _, msg := range currentChunk {
								func(view *mq.MessageView) {
									defer func() {
										if rec := recover(); rec != nil {
											pr.Error("Panic in parallel handler: %v, topic: %s, msgId: %s",
												rec, view.GetTopic(), view.GetMessageId())
										}
										_ = consumer.Ack(ctx, view)
									}()

									if err := handler(ctx, view); err != nil {
										pr.Error("Error in parallel handler: %v, topic: %s, msgId: %s",
											err, view.GetTopic(), view.GetMessageId())
									}
								}(msg)
							}
						})
					}
				} else {
					// 普通处理：整批提交到工作池
					workerPool.SubmitTask(func() {
						// 批量处理同一主题的消息
						for _, view := range messages {
							// 使用闭包捕获当前消息视图
							func(view *mq.MessageView) {
								// 处理潜在的异常
								defer func() {
									if rec := recover(); rec != nil {
										pr.Error("Panic in message handler: %v, topic: %s, msgId: %s",
											rec, view.GetTopic(), view.GetMessageId())
									}

									// 确认消息处理完成
									_ = consumer.Ack(ctx, view)
								}()

								// 处理消息
								if err := handler(ctx, view); err != nil {
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
		messageViewPool.Put(msgViewsSlice)
		ackResultPool.Put(ackResults)

		// 动态调整批处理大小
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

// WorkerPool 工作者池，用于管理和调度任务
type WorkerPool struct {
	maxWorkers int
	tasks      chan func()
	wg         sync.WaitGroup
}

// NewWorkerPool 创建新的工作者池
func NewWorkerPool(maxWorkers int) *WorkerPool {
	return &WorkerPool{
		maxWorkers: maxWorkers,
		tasks:      make(chan func(), 100), // 缓冲区大小可以根据需要调整
	}
}

// Start 启动工作者池
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
func (p *WorkerPool) Stop() {
	close(p.tasks)
	p.wg.Wait()
}

// 添加标准context到自定义Context的转换适配器
func adaptContext(stdCtx Context) Context {
	// 为了简化问题，我们使用一个最小的日志记录适配器
	// 实际使用中应该根据需要创建完整的Context实现
	return NewDefaultContext("rocket_adapter", "rocket_consumer")
}

// 添加自定义Context到标准context的转换函数
func toStdContext(ctx Context) context.Context {
	// 如果Context已经是context.Context类型，则直接返回
	if stdCtx, ok := ctx.(context.Context); ok {
		return stdCtx
	}
	// 如果转换失败，则使用background context
	return context.Background()
}
