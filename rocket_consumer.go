package sylph

import (
	"fmt"
	mq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/pkg/errors"
	"os"
	"strings"
	"time"
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

func NewMessage(ctx context.Context, view *mq.MessageView) *Message {
	return &Message{
		ctx:  ctx,
		view: view,
	}
}

type Message struct {
	ctx  context.Context
	view *mq.MessageView
}

func (m *Message) Context() context.Context {
	return m.ctx
}

func (m *Message) View() *mq.MessageView {
	return m.view
}

type RocketTaskHandler func(ctx context.Context, view *mq.MessageView) (err error)

type RocketTaskRoutes map[x.ITaskName]RocketTaskHandler

type RocketConsumerServer struct {
	ctx context.Context
	baseConsumerRocket
	routes RocketTaskRoutes
}

func NewRocketConsumerServer(ctx context.Context, consumer RocketConsumer, instance RocketInstance) *RocketConsumerServer {
	return &RocketConsumerServer{
		ctx: ctx,
		baseConsumerRocket: baseConsumerRocket{
			consumer: consumer,
			instance: instance,
		},
		routes: make(RocketTaskRoutes),
	}
}

func (r *RocketConsumerServer) RegisterRoute(topic x.ITaskName, handler RocketTaskHandler) {
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

func (r *RocketConsumerServer) Listen(ctx context.Context) {
	for i := 0; i < r.consumer.TakeNum(); i++ {
		go r.Receive(ctx, r.client)
	}
}

func (r *RocketConsumerServer) Receive(ctx context.Context, consumer mq.SimpleConsumer) {
	for {
		mvs, err := consumer.Receive(ctx, r.consumer.TakeMaxMessageNum(), r.consumer.TakeInvisibleDuration())

		//pr.Red("receive error: %v", err)
		//pr.Red("receive mvs: %v", mvs)
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

		for _, view := range mvs {
			topic := Topic(view.GetTopic())
			handler, ok := r.routes[topic]
			if !ok || handler == nil {
				ctx.Warn("server.RocketConsumerServer.Receive", "consumer no handler", x.H{
					"msg_id": view.GetMessageId(),
					"topic":  view.GetTopic(),
					"group":  r.consumer.TakeGroup(),
				})

				continue
			}

			go r.handleMsg(ctx, consumer, view, handler)
		}
	}
}

func (r *RocketConsumerServer) handleMsg(ctx context.Context, consumer mq.SimpleConsumer, view *mq.MessageView, handler RocketTaskHandler) {
	topic := view.GetTopic()
	defer func() {
		if err := recover(); err != nil {
			newError := errors.New(fmt.Sprintf("%v", err))
			ctx.Error("server.RocketConsumerServer.handleMsg", "consume handle panic", newError, x.H{
				"topic":  topic,
				"msg_id": view.GetMessageId(),
			})
		}
	}()

	// 不管怎么样都要ack
	defer func() {
		_ = consumer.Ack(ctx, view)
	}()

	cloneCtx := ctx.Clone()
	cloneCtx.TakeHeader().StorePath(topic)

	// 怎么ack? 传递下去？
	err := handler(ctx, view)
	if err != nil {
		ctx.Error("server.RocketConsumerServer.handleMsg", "topic handle error", err, x.H{
			"topic":  topic,
			"msg_id": view.GetMessageId(),
		})

	}

}
