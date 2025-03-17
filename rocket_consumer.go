package sylph

import (
	"context"
	"os"
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
}

func NewRocketConsumerServer(ctx Context, consumer RocketConsumer, instance RocketInstance) *RocketConsumerServer {
	return &RocketConsumerServer{
		ctx: ctx,
		baseConsumerRocket: baseConsumerRocket{
			consumer: consumer,
			instance: instance,
		},
		routes: make(RocketTaskRoutes),
	}
}

func (r *RocketConsumerServer) RegisterRoute(topic ITaskName, handler RocketTaskHandler) {
	r.routes[topic] = handler
}

func (r *RocketConsumerServer) Boot() (err error) {
	if err = r.makeClient(); err != nil {
		return
	}

	if r.started {
		return
	}

	if err = r.client.Start(); err != nil {
		return
	}

	r.Listen(r.ctx)
	return
}

func (r *RocketConsumerServer) Shutdown() error {
	return nil
}

func (r *RocketConsumerServer) Listen(ctx Context) {
	for i := 0; i < r.consumer.TakeNum(); i++ {
		go r.Receive(ctx, r.client)
	}
}

func (r *RocketConsumerServer) Receive(ctx Context, consumer mq.SimpleConsumer) {
	// 创建用于批量处理的工作池
	workerPool := NewWorkerPool(MaxWorkers)
	workerPool.Start()

	// 创建消息视图对象池
	messageViewPool := sync.Pool{
		New: func() interface{} {
			return make([]*mq.MessageView, 0, r.consumer.TakeMaxMessageNum())
		},
	}

	for {
		// 从对象池获取预分配的切片
		msgViewsSlice := messageViewPool.Get().([]*mq.MessageView)
		// 确保切片是空的
		msgViewsSlice = msgViewsSlice[:0]

		// 接收消息
		mvs, err := consumer.Receive(ctx, r.consumer.TakeMaxMessageNum(), r.consumer.TakeInvisibleDuration())

		if err != nil {
			var status *mq.ErrRpcStatus
			if errors.As(err, &status) {
				if status.GetCode() == 40401 {
					time.Sleep(2 * time.Second)
					continue
				}
			}

			time.Sleep(2 * time.Second)
			continue
		}

		// 先收集相同主题的消息，以便批量处理
		messagesByTopic := make(map[string][]*mq.MessageView)

		for _, view := range mvs {
			topic := view.GetTopic()
			messagesByTopic[topic] = append(messagesByTopic[topic], view)
		}

		// 按主题批量处理消息
		for topic, messages := range messagesByTopic {
			topicName := Topic(topic)
			handler, ok := r.routes[topicName]
			if !ok || handler == nil {
				pr.Yellow("Consumer no handler for topic: %s, group: %s, message count: %d",
					topic, r.consumer.TakeGroup(), len(messages))

				// 对于没有处理器的消息，我们只需要确认它们
				for _, view := range messages {
					_ = consumer.Ack(ctx, view)
				}
				continue
			}

			// 使用工作池处理此主题的所有消息
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

		// 将切片归还到对象池
		messageViewPool.Put(msgViewsSlice)
	}
}

// 工作者池相关常量和类型
const (
	MaxWorkers = 10 // 最大工作者数量
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
