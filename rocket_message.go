package sylph

import (
	"time"

	mq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/sylphbyte/pr"
)

// TransactionHandle 事务消息处理函数类型
// 用于处理事务消息的回调函数，决定事务消息的提交或回滚
//
// 参数:
//   - ctx: 事务上下文，包含事务相关信息
//
// 返回:
//   - error: 如果返回nil则提交事务，否则回滚事务
type TransactionHandle func(ctx Context) error

// Tag 消息标签类型
// 用于消息过滤和分类
type Tag string

// NewSendMessage 创建一个新的发送消息对象
// 将任意类型的值转换为可发送的消息对象
//
// 参数:
//   - val: 任意类型的消息内容，将被序列化为JSON
//
// 返回:
//   - *SendMessage: 新创建的消息对象
//
// 使用示例:
//
//	msg := sylph.NewSendMessage(map[string]string{"key": "value"})
//	msg.WithTag("order")
func NewSendMessage(val any) *SendMessage {
	body, _ := _json.Marshal(val)
	return &SendMessage{
		Body: body,
		opts: &SendMessageOption{},
	}
}

// SendMessageOption 发送消息选项
// 包含消息发送的各种配置选项
type SendMessageOption struct {
	transactionHandle TransactionHandle // 事务处理函数
	delayTime         time.Time         // 延迟发送时间

	tag        Tag               // 消息标签
	keys       []string          // 消息键，用于查询和统计
	properties map[string]string // 消息自定义属性
}

// SendMessage 发送消息结构体
// 封装了待发送的消息内容和选项
type SendMessage struct {
	Body []byte             `json:"body"` // 消息体，序列化后的内容
	opts *SendMessageOption // 消息选项
}

// WithTag 设置消息标签
// 标签用于消息过滤和分类
//
// 参数:
//   - tag: 消息标签
func (s *SendMessage) WithTag(tag Tag) {
	s.opts.tag = tag
}

// WithTransactionHandle 设置事务处理函数
// 用于处理事务消息的提交或回滚
//
// 参数:
//   - handle: 事务处理函数
//
// 注意事项:
//   - 只有事务消息需要设置此项
func (s *SendMessage) WithTransactionHandle(handle TransactionHandle) {
	s.opts.transactionHandle = handle
}

// TakeTransactionHandle 获取事务处理函数
//
// 返回:
//   - TransactionHandle: 当前设置的事务处理函数
func (s *SendMessage) TakeTransactionHandle() TransactionHandle {
	return s.opts.transactionHandle
}

// WithDelayTime 设置消息延迟发送时间
// 消息将在指定时间点发送
//
// 参数:
//   - delayTime: 期望发送消息的时间点
func (s *SendMessage) WithDelayTime(delayTime time.Time) {
	s.opts.delayTime = delayTime
}

// TakeDelayTime 获取消息延迟发送时间
// 如果未设置延迟时间，则返回当前时间
//
// 返回:
//   - time.Time: 延迟发送的时间点
func (s *SendMessage) TakeDelayTime() time.Time {
	if s.opts == nil || s.opts.delayTime.IsZero() {
		return time.Now()
	}
	return s.opts.delayTime
}

// WithDelayDuration 设置消息延迟发送持续时间
// 消息将在当前时间基础上推迟指定的持续时间后发送
//
// 参数:
//   - duration: 延迟的持续时间
//
// 使用示例:
//
//	msg.WithDelayDuration(time.Minute * 5) // 5分钟后发送
func (s *SendMessage) WithDelayDuration(duration time.Duration) {
	s.opts.delayTime = time.Now().Add(duration)
}

// WithKeys 设置消息键
// 消息键用于消息查询和统计
//
// 参数:
//   - keys: 一个或多个消息键
func (s *SendMessage) WithKeys(keys ...string) {
	s.opts.keys = keys
}

// WithProperty 设置消息自定义属性
// 用于添加消息的元数据
//
// 参数:
//   - key: 属性键
//   - value: 属性值
func (s *SendMessage) WithProperty(key, value string) {
	s.getOrNewProperties()[key] = value
}

// getOrNewProperties 获取或创建属性映射
// 如果属性映射不存在则创建一个新的
//
// 返回:
//   - map[string]string: 属性映射
func (s *SendMessage) getOrNewProperties() map[string]string {
	if s.opts.properties == nil {
		s.opts.properties = make(map[string]string)
	}
	return s.opts.properties
}

// TakeMqMessage 转换为RocketMQ客户端消息
// 将SendMessage转换为RocketMQ客户端可用的消息格式
//
// 参数:
//   - topic: 消息主题
//
// 返回:
//   - *mq.Message: RocketMQ客户端消息
//
// 注意事项:
//   - 此方法主要供内部使用，将内部消息格式转换为RocketMQ客户端格式
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

// NewSendRet 创建一个新的发送结果对象
// 封装RocketMQ客户端的发送结果
//
// 参数:
//   - raw: RocketMQ客户端原始发送回执
//   - err: 发送过程中的错误
//
// 返回:
//   - *SendRet: 发送结果对象
func NewSendRet(raw []*mq.SendReceipt, err error) *SendRet {
	return &SendRet{
		raw: raw,
		err: err,
	}
}

// SendRet 发送结果结构体
// 封装了消息发送的结果信息
type SendRet struct {
	raw []*mq.SendReceipt // 原始发送回执
	err error             // 发送错误
}

// TakeError 获取发送错误
//
// 返回:
//   - error: 发送过程中的错误，如果为nil表示发送成功
func (s *SendRet) TakeError() error {
	return s.err
}

// TakeReceipts 获取发送回执
//
// 返回:
//   - []*mq.SendReceipt: 原始发送回执列表
func (s *SendRet) TakeReceipts() []*mq.SendReceipt {
	return s.raw
}

// PrintTakeReceipts 打印发送回执
// 以JSON格式打印发送回执信息，用于调试
func (s *SendRet) PrintTakeReceipts() {
	pr.Json(s.raw)
}
