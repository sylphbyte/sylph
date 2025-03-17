package sylph

import (
	mq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/sylphbyte/pr"
	"time"
)

type TransactionHandle func(ctx Context) error

type Tag string // 消息标签

func NewSendMessage(val any) *SendMessage {
	body, _ := _json.Marshal(val)
	return &SendMessage{
		Body: body,
		opts: &SendMessageOption{},
	}
}

type SendMessageOption struct {
	transactionHandle TransactionHandle
	delayTime         time.Time

	tag        Tag
	keys       []string
	properties map[string]string
}

type SendMessage struct {
	Body []byte `json:"body"`
	opts *SendMessageOption
}

func (s *SendMessage) WithTag(tag Tag) {
	s.opts.tag = tag
}

func (s *SendMessage) WithTransactionHandle(handle TransactionHandle) {
	s.opts.transactionHandle = handle
}

func (s *SendMessage) TakeTransactionHandle() TransactionHandle {
	return s.opts.transactionHandle
}

func (s *SendMessage) WithDelayTime(delayTime time.Time) {
	s.opts.delayTime = delayTime
}

func (s *SendMessage) TakeDelayTime() time.Time {
	if s.opts == nil || s.opts.delayTime.IsZero() {
		return time.Now()
	}
	return s.opts.delayTime
}

func (s *SendMessage) WithDelayDuration(duration time.Duration) {
	s.opts.delayTime = time.Now().Add(duration)
}

func (s *SendMessage) WithKeys(keys ...string) {
	s.opts.keys = keys
}

func (s *SendMessage) WithProperty(key, value string) {
	s.getOrNewProperties()[key] = value
}

func (s *SendMessage) getOrNewProperties() map[string]string {
	if s.opts.properties == nil {
		s.opts.properties = make(map[string]string)
	}
	return s.opts.properties
}

func (s *SendMessage) TakeMqMessage(topic string) *mq.Message {
	msg := &mq.Message{
		Topic: topic,
		Body:  s.Body,
	}

	if s.opts != nil {
		if s.opts.tag != "" {
			msg.SetTag(string(s.opts.tag))
		}

		if len(s.opts.keys) > 0 {
			msg.SetKeys(s.opts.keys...)
		}

		if len(s.opts.properties) > 0 {
			for k, v := range s.opts.properties {
				msg.AddProperty(k, v)
			}
		}
	}

	return msg
}

func NewSendRet(raw []*mq.SendReceipt, err error) *SendRet {
	return &SendRet{
		raw: raw,
		err: err,
	}
}

type SendRet struct {
	raw []*mq.SendReceipt
	err error
}

func (s *SendRet) TakeError() error {
	return s.err
}

func (s *SendRet) TakeReceipts() []*mq.SendReceipt {
	return s.raw
}

func (s *SendRet) PrintTakeReceipts() {
	pr.Json(s.raw)
}
