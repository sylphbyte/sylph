package sylph

import (
	"sylph/pkg/storage"
)

var (
	producerRegistry = producerRegistryMapping{
		TopicKindNormal:      NewNormalProducer,
		TopicKindFifo:        NewFifoProducer,
		TopicKindDelay:       NewDelayProducer,
		TopicKindTransaction: NewTransactionProducer,
	}
)

type producerRegistryMapping map[TopicKind]producerHandler

type producerHandler func(topic RocketTopic, instance RocketInstance) IProducer

type IProducer interface {
	Send(ctcontext.Context.Context, message *SendMessage) *SendRet
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

func (n *NormalProducer) Send(ctcontext.Context.Context, message *SendMessage) *SendRet {
	msg := message.TakeMqMessage(n.topic.Topic)

	return NewSendRet(n.client.Send(ctx, msg))
}

func NewDelayProducer(topic RocketTopic, instance RocketInstance) IProducer {
	return &DelayProducer{
		baseProducerRocket: newBaseProducer(topic, instance),
	}
}

type DelayProducer struct {
	*baseProducerRocket
}

func (n DelayProducer) Send(ctcontext.Context.Context, message *SendMessage) *SendRet {
	msg := message.TakeMqMessage(n.topic.Topic)
	msg.SetDelayTimestamp(message.TakeDelayTime())
	return NewSendRet(n.client.Send(ctx, msg))
}

func NewFifoProducer(topic RocketTopic, instance RocketInstance) IProducer {
	return &FifoProducer{
		baseProducerRocket: newBaseProducer(topic, instance),
	}
}

type FifoProducer struct {
	*baseProducerRocket
}

func (n FifoProducer) Send(ctcontext.Context.Context, message *SendMessage) *SendRet {
	msg := message.TakeMqMessage(n.topic.Topic)
	msg.SetDelayTimestamp(message.TakeDelayTime())
	return NewSendRet(n.client.Send(ctx, msg))
}

func NewTransactionProducer(topic RocketTopic, instance RocketInstance) IProducer {
	return &TransactionProducer{
		baseProducerRocket: newBaseProducer(topic, instance),
	}
}

type TransactionProducer struct {
	*baseProducerRocket
}

func (n TransactionProducer) Send(ctcontext.Context.Context, message *SendMessage) *SendRet {
	msg := message.TakeMqMessage(n.topic.Topic)
	msg.SetDelayTimestamp(message.TakeDelayTime())

	transaction := n.client.BeginTransaction()
	resp, err := n.client.SendWithTransaction(ctx, msg, transaction)
	if err != nil {
		return NewSendRet(nil, err)
	}

	handler := message.TakeTransactionHandle()
	if handler == nil {
		_ = transaction.RollBack()
		return NewSendRet(nil, storage.ErrTransactionNoHandle)
	}

	err = handler(ctx)

	if err != nil {
		_ = transaction.RollBack()
		return NewSendRet(nil, err)
	}

	_ = transaction.Commit()
	return NewSendRet(resp, nil)
}
