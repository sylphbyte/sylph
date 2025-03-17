package sylph

import (
	"fmt"
	"sync"

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

	// 从连接池获取或创建客户端
	p.client, err = globalRocketPool.getOrCreateProducer(p.instance, p.topic)
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

	// 从连接池获取或创建客户端
	p.client, err = globalRocketPool.getOrCreateConsumer(p.instance, p.consumer)
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

// 客户端连接池实现
type rocketClientPool struct {
	sync.RWMutex
	producers map[string]mq.Producer
	consumers map[string]mq.SimpleConsumer
}

// 全局客户端连接池
var globalRocketPool = &rocketClientPool{
	producers: make(map[string]mq.Producer),
	consumers: make(map[string]mq.SimpleConsumer),
}

// getOrCreateProducer 获取或创建生产者客户端
func (p *rocketClientPool) getOrCreateProducer(instance RocketInstance, topic RocketTopic) (mq.Producer, error) {
	key := MakeRocketId(topic, instance)

	// 先尝试从池中获取
	p.RLock()
	if client, ok := p.producers[key]; ok {
		p.RUnlock()
		return client, nil
	}
	p.RUnlock()

	// 不存在则创建新客户端（需要写锁）
	p.Lock()
	defer p.Unlock()

	// 二次检查（双重检查锁定模式）
	if client, ok := p.producers[key]; ok {
		return client, nil
	}

	// 创建新客户端
	client, err := mq.NewProducer(instance.MakeConfig(), topic.TakeOptions()...)
	if err != nil {
		return nil, err
	}

	// 保存到池中
	p.producers[key] = client
	return client, nil
}

// getOrCreateConsumer 获取或创建消费者客户端
func (p *rocketClientPool) getOrCreateConsumer(instance RocketInstance, consumer RocketConsumer) (mq.SimpleConsumer, error) {
	key := fmt.Sprintf("%s:%s", instance.Name, consumer.Group)

	// 先尝试从池中获取
	p.RLock()
	if client, ok := p.consumers[key]; ok {
		p.RUnlock()
		return client, nil
	}
	p.RUnlock()

	// 不存在则创建新客户端（需要写锁）
	p.Lock()
	defer p.Unlock()

	// 二次检查
	if client, ok := p.consumers[key]; ok {
		return client, nil
	}

	// 创建新客户端
	conf := instance.MakeConfig()
	conf.ConsumerGroup = consumer.Group

	client, err := mq.NewSimpleConsumer(conf, consumer.TakeOptions()...)
	if err != nil {
		return nil, err
	}

	// 保存到池中
	p.consumers[key] = client
	return client, nil
}

// closeAll 关闭所有客户端连接
func (p *rocketClientPool) closeAll() {
	p.Lock()
	defer p.Unlock()

	// 简单清空映射，让GC处理客户端资源
	// 实际应用中如果有明确的关闭方法，应该在此调用
	p.producers = make(map[string]mq.Producer)
	p.consumers = make(map[string]mq.SimpleConsumer)
}
