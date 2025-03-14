package sylph

import (
	"fmt"
	mq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
)

const (
	// TopicKindNormal 正常队列
	TopicKindNormal TopicKind = iota
	// TopicKindFifo 顺序队列
	TopicKindFifo
	// TopicKindDelay 延迟队列
	TopicKindDelay
	// TopicKindTransaction 事务队列
	TopicKindTransaction
)

var (
	topicKindNames = TopicKindNames{
		TopicKindNormal:      "NORMAL",
		TopicKindFifo:        "FIFO",
		TopicKindDelay:       "DELAY",
		TopicKindTransaction: "Transactional",
	}
)

type TopicKindNames map[TopicKind]string

// TopicKind 定义通道类型
type TopicKind int

func (t TopicKind) Name() string {
	return topicKindNames[t]
}

// Topic 定义Topic 就是任务名？
type Topic string

func (t Topic) Name() string {
	return string(t)
}

// RocketYaml rocketYaml 配置
type RocketYaml struct {
	RocketGroup RocketGroup `yaml:"rocket_group" mapstructure:"rocket_group"`
}

type RocketGroup map[string]RocketInstance

// RocketInstance Rocket实例
type RocketInstance struct {
	Name      string          `json:"name" mapstructure:"name"`
	Endpoint  string          `yaml:"end_point" mapstructure:"end_point"`
	AccessKey string          `yaml:"access_key" mapstructure:"access_key"`
	SecretKey string          `yaml:"secret_key" mapstructure:"secret_key"`
	Topics    RocketTopics    `yaml:"topics" mapstructure:"topics"`
	Consumers RocketConsumers `yaml:"consumers" mapstructure:"consumers"`
}

func (m RocketInstance) MakeConfig() *mq.Config {
	return &mq.Config{
		Endpoint: m.Endpoint,
		Credentials: &credentials.SessionCredentials{
			AccessKey:    m.AccessKey,
			AccessSecret: m.SecretKey,
		},
	}
}

type RocketTopics []RocketTopic
type RocketTopic struct {
	Topic string    `yaml:"topic" mapstructure:"topic"`
	Tags  string    `yaml:"tags"  mapstructure:"tags"`
	Kind  TopicKind `yaml:"kind" mapstructure:"kind"`
}

func (p *RocketTopic) TakeOptions() []mq.ProducerOption {
	opts := []mq.ProducerOption{
		mq.WithTopics(p.Topic),
	}

	if p.Kind == TopicKindTransaction {
		opts = append(opts, mq.WithTransactionChecker(&mq.TransactionChecker{
			Check: func(msg *mq.MessageView) mq.TransactionResolution {
				return mq.COMMIT
			},
		}))
	}

	return opts
}

func MakeRocketId(topic RocketTopic, instance RocketInstance) string {
	return fmt.Sprintf("%s:%s", instance.Name, topic.Topic)
}

type baseProducerRocket struct {
	topic    RocketTopic
	instance RocketInstance
	client   mq.Producer
	started  bool
}

func (p *baseProducerRocket) makeClient() (err error) {
	if p.client != nil {
		return
	}

	p.client, err = mq.NewProducer(p.instance.MakeConfig(), p.topic.TakeOptions()...)
	return
}

func (p *baseProducerRocket) Boot() (err error) {
	if err = p.makeClient(); err != nil {
		return
	}

	if p.started {
		return
	}

	p.started = true
	return p.client.Start()
}

type baseConsumerRocket struct {
	consumer RocketConsumer
	instance RocketInstance
	client   mq.SimpleConsumer
	started  bool
}

func (p *baseConsumerRocket) makeClient() (err error) {
	if p.client != nil {
		return
	}

	conf := p.instance.MakeConfig()
	conf.ConsumerGroup = p.consumer.Group

	p.client, err = mq.NewSimpleConsumer(conf, p.consumer.TakeOptions()...)
	return
}

func (p *baseConsumerRocket) Boot() (err error) {
	if err = p.makeClient(); err != nil {
		return
	}

	if p.started {
		return
	}

	p.started = true
	return p.client.Start()
}
