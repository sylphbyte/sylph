package sylph

import (
	"context"
	"github.com/pkg/errors"
)

var (
	producerRegistry = producerRegistryMapping{
		TopicKindNormal:      NewNormalProducer,
		TopicKindFifo:        NewFifoProducer,
		TopicKindDelay:       NewDelayProducer,
		TopicKindTransaction: NewTransactionProducer,
	}

	ErrTransactionNoHandle = errors.New("transaction no handle")
)

type producerRegistryMapping map[TopicKind]producerHandler

type producerHandler func(topic RocketTopic, instance RocketInstance) IProducer

// IProducer RocketMQ生产者接口，增加批量发送方法
type IProducer interface {
	Send(ctx Context, message *SendMessage) *SendRet
	SendBatch(ctx Context, messages []*SendMessage) []*SendRet
	Boot() error
}

func newBaseProducer(topic RocketTopic, instance RocketInstance) *baseProducerRocket {
	return &baseProducerRocket{topic: topic, instance: instance}
}

func NewRocketProducer(topic RocketTopic, instance RocketInstance) IProducer {
	handler := producerRegistry[topic.Kind]
	return handler(topic, instance)
}

func NewNormalProducer(topic RocketTopic, instance RocketInstance) IProducer {
	return &NormalProducer{
		baseProducerRocket: newBaseProducer(topic, instance),
	}
}

type NormalProducer struct {
	*baseProducerRocket
}

func (n *NormalProducer) Send(ctx Context, message *SendMessage) *SendRet {
	msg := message.TakeMqMessage(n.topic.Topic)

	return NewSendRet(n.client.Send(context.Background(), msg))
}

// SendBatch 批量发送消息
func (n *NormalProducer) SendBatch(ctx Context, messages []*SendMessage) []*SendRet {
	if len(messages) == 0 {
		return []*SendRet{}
	}

	// 预分配结果数组
	results := make([]*SendRet, len(messages))

	// 逐个发送（RocketMQ Go SDK可能不支持批量发送）
	for i, message := range messages {
		results[i] = n.Send(ctx, message)
	}

	return results
}

func NewDelayProducer(topic RocketTopic, instance RocketInstance) IProducer {
	return &DelayProducer{
		baseProducerRocket: newBaseProducer(topic, instance),
	}
}

type DelayProducer struct {
	*baseProducerRocket
}

func (n DelayProducer) Send(ctx Context, message *SendMessage) *SendRet {
	msg := message.TakeMqMessage(n.topic.Topic)
	msg.SetDelayTimestamp(message.TakeDelayTime())
	return NewSendRet(n.client.Send(context.Background(), msg))
}

// SendBatch 批量发送延迟消息
func (n DelayProducer) SendBatch(ctx Context, messages []*SendMessage) []*SendRet {
	if len(messages) == 0 {
		return []*SendRet{}
	}

	// 预分配结果数组
	results := make([]*SendRet, len(messages))

	// 逐个发送
	for i, message := range messages {
		results[i] = n.Send(ctx, message)
	}

	return results
}

func NewFifoProducer(topic RocketTopic, instance RocketInstance) IProducer {
	return &FifoProducer{
		baseProducerRocket: newBaseProducer(topic, instance),
	}
}

type FifoProducer struct {
	*baseProducerRocket
}

func (n FifoProducer) Send(ctx Context, message *SendMessage) *SendRet {
	msg := message.TakeMqMessage(n.topic.Topic)
	msg.SetDelayTimestamp(message.TakeDelayTime())
	return NewSendRet(n.client.Send(context.Background(), msg))
}

// SendBatch 批量发送FIFO消息
func (n FifoProducer) SendBatch(ctx Context, messages []*SendMessage) []*SendRet {
	if len(messages) == 0 {
		return []*SendRet{}
	}

	// 预分配结果数组
	results := make([]*SendRet, len(messages))

	// 逐个发送
	for i, message := range messages {
		results[i] = n.Send(ctx, message)
	}

	return results
}

func NewTransactionProducer(topic RocketTopic, instance RocketInstance) IProducer {
	return &TransactionProducer{
		baseProducerRocket: newBaseProducer(topic, instance),
	}
}

type TransactionProducer struct {
	*baseProducerRocket
}

func (n TransactionProducer) Send(ctx Context, message *SendMessage) *SendRet {
	msg := message.TakeMqMessage(n.topic.Topic)
	msg.SetDelayTimestamp(message.TakeDelayTime())

	transaction := n.client.BeginTransaction()
	resp, err := n.client.SendWithTransaction(context.Background(), msg, transaction)
	if err != nil {
		return NewSendRet(nil, err)
	}

	handler := message.TakeTransactionHandle()
	if handler == nil {
		_ = transaction.RollBack()
		return NewSendRet(nil, ErrTransactionNoHandle)
	}

	err = handler(ctx)

	if err != nil {
		_ = transaction.RollBack()
		return NewSendRet(nil, err)
	}

	_ = transaction.Commit()
	return NewSendRet(resp, nil)
}

// 事务消息不支持批量发送，实现空方法满足接口
func (n TransactionProducer) SendBatch(ctx Context, messages []*SendMessage) []*SendRet {
	// 对于事务消息，不支持批量发送，只能逐个发送
	results := make([]*SendRet, len(messages))
	for i, message := range messages {
		results[i] = n.Send(ctx, message)
	}
	return results
}
