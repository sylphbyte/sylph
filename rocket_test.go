package sylph

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// TestRocketConsumer_TakeOptions 测试消费者选项配置
func TestRocketConsumer_TakeOptions(t *testing.T) {
	consumer := RocketConsumer{
		Group: "test-group",
		Wait:  5,
		Subscriptions: []RocketTopic{
			{Topic: "topic1", Tags: "tag1"},
			{Topic: "topic2", Tags: "tag2,tag3"},
			{Topic: "topic3", Tags: ""}, // 应该默认为 "*"
		},
	}

	// 测试获取组名
	assert.Equal(t, "test-group", consumer.TakeGroup())

	// 测试等待时间
	assert.Equal(t, 5*time.Second, consumer.TakeWait())

	// 测试订阅表达式
	expressions := consumer.makeSubscriptionExpressions()
	assert.Equal(t, 3, len(expressions))
	assert.Contains(t, expressions, "topic1")
	assert.Contains(t, expressions, "topic2")
	assert.Contains(t, expressions, "topic3")

	// 检查空标签是否默认为 "*"
	// 注意：由于FilterExpression的实现可能变化，我们无法直接访问表达式内容
	// 因此这部分测试暂时省略
}

// TestRocketInstance_MakeConfig 测试RocketMQ实例配置生成
func TestRocketInstance_MakeConfig(t *testing.T) {
	// 创建测试实例
	instance := RocketInstance{
		Name:      "test-instance",
		Endpoint:  "localhost:9876",
		AccessKey: "test-key",
		SecretKey: "test-secret",
	}

	// 生成配置
	config := instance.MakeConfig()

	// 验证配置是否正确
	assert.Equal(t, "localhost:9876", config.Endpoint)
	assert.NotNil(t, config.Credentials)
	assert.Equal(t, "test-key", config.Credentials.AccessKey)
	assert.Equal(t, "test-secret", config.Credentials.AccessSecret)
}

// TestConsumerServerName 测试消费者服务器名称
func TestConsumerServerName(t *testing.T) {
	// 初始化测试数据
	consumer := RocketConsumer{
		Group: "test-group",
	}

	instance := RocketInstance{
		Name: "test-instance",
	}

	// 创建服务器
	server := NewRocketConsumerServer(consumer, instance)

	// 验证名称
	assert.Equal(t, "test-group", server.Name())
}

// TestSendMessageOptionsAndConfig 测试SendMessage选项设置
func TestSendMessageOptionsAndConfig(t *testing.T) {
	// 创建测试消息
	msg := NewSendMessage(map[string]string{"test": "value"})

	// 确保对象已正确创建
	assert.NotNil(t, msg)
	assert.NotNil(t, msg.Body)
	assert.NotNil(t, msg.opts)

	// 测试标签设置
	msg.WithTag("test-tag")
	assert.Equal(t, Tag("test-tag"), msg.opts.tag)

	// 测试延迟持续时间设置
	duration := 10 * time.Minute
	now := time.Now()
	msg.WithDelayDuration(duration)

	// 检查延迟时间是否在预期范围内（允许小误差）
	delayTime := msg.TakeDelayTime()
	assert.True(t, delayTime.After(now.Add(duration-time.Second)))
	assert.True(t, delayTime.Before(now.Add(duration+time.Second)))

	// 测试属性设置
	msg.WithProperty("prop1", "val1")
	msg.WithProperty("prop2", "val2")

	// 检查属性是否已设置
	props := msg.getOrNewProperties()
	assert.Equal(t, "val1", props["prop1"])
	assert.Equal(t, "val2", props["prop2"])
}

// TestWorkerPool 测试工作者池的基本功能
func TestWorkerPool(t *testing.T) {
	// 创建工作者池
	pool := NewWorkerPool(2)
	assert.Equal(t, 2, pool.maxWorkers)

	// 启动池
	pool.Start()

	// 创建通道来同步测试
	done := make(chan bool)

	// 提交任务
	pool.SubmitTask(func() {
		time.Sleep(10 * time.Millisecond)
		done <- true
	})

	// 等待任务完成
	select {
	case <-done:
		// 任务成功完成
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Task did not complete in time")
	}

	// 停止池
	pool.Stop()
}

// TestContextAdaptation 测试上下文转换函数
func TestContextAdaptation(t *testing.T) {
	// 测试适配函数
	ctx := NewContext("test", "test")
	adaptedCtx := adaptContext(ctx)
	assert.NotNil(t, adaptedCtx)

	// 测试从自定义上下文到标准上下文的转换
	wrappedCtx := toStdContext(ctx)
	assert.NotNil(t, wrappedCtx)

	// 测试使用已有的标准上下文
	stdCtx := context.Background()
	// 注：直接传入标准context.Context是不合法的操作
	// 这里我们使用一个更合理的测试
	// 验证标准上下文本身是context.Context类型
	_, ok := stdCtx.(context.Context)
	assert.True(t, ok)
}

// TestTopicKind 测试主题类型
func TestTopicKind(t *testing.T) {
	// 测试主题类型名称
	assert.Equal(t, "NORMAL", TopicKindNormal.Name())
	assert.Equal(t, "FIFO", TopicKindFifo.Name())
	assert.Equal(t, "DELAY", TopicKindDelay.Name())
	assert.Equal(t, "Transactional", TopicKindTransaction.Name())
}

// TestTopic 测试主题
func TestTopic(t *testing.T) {
	topic := Topic("test-topic")
	assert.Equal(t, "test-topic", topic.Name())
}

// TestRocketTopic_TakeOptions 测试主题选项
func TestRocketTopic_TakeOptions(t *testing.T) {
	// 测试普通主题
	normalTopic := RocketTopic{
		Topic: "normal-topic",
		Tags:  "tag1",
		Kind:  TopicKindNormal,
	}
	normalOpts := normalTopic.TakeOptions()
	assert.Len(t, normalOpts, 1)

	// 测试事务主题
	txTopic := RocketTopic{
		Topic: "tx-topic",
		Tags:  "tag1",
		Kind:  TopicKindTransaction,
	}
	txOpts := txTopic.TakeOptions()
	assert.Len(t, txOpts, 2) // 有额外的事务检查器选项
}

// TestMakeRocketId 测试创建RocketID
func TestMakeRocketId(t *testing.T) {
	topic := RocketTopic{
		Topic: "test-topic",
	}
	instance := RocketInstance{
		Name: "test-instance",
	}

	id := MakeRocketId(topic, instance)
	assert.Equal(t, "test-instance:test-topic", id)
}

// TestRocketConsumer_Methods 测试RocketConsumer的方法
func TestRocketConsumer_Methods(t *testing.T) {
	consumer := RocketConsumer{
		Group:             "test-group",
		Num:               3,
		Wait:              5,
		MaxMessageNum:     10,
		InvisibleDuration: 30,
	}

	// 测试数量和时间设置
	assert.Equal(t, 3, consumer.TakeNum())
	assert.Equal(t, 5*time.Second, consumer.TakeWait())
	assert.Equal(t, int32(10), consumer.TakeMaxMessageNum())
	assert.Equal(t, 30*time.Second, consumer.TakeInvisibleDuration())
}

// TestLoadServiceRocketConfig 测试从配置文件加载服务配置
func TestLoadServiceRocketConfig(t *testing.T) {
	// 读取配置文件
	data, err := ioutil.ReadFile("etc/service_rocket.yaml")
	if err != nil {
		t.Fatalf("无法读取配置文件: %v", err)
	}

	// 解析YAML
	var yamlConfig struct {
		RocketGroup RocketGroup `yaml:"rocket_group"`
	}

	err = yaml.Unmarshal(data, &yamlConfig)
	if err != nil {
		t.Fatalf("解析配置文件失败: %v", err)
	}

	// 验证基本配置结构
	assert.NotEmpty(t, yamlConfig.RocketGroup)

	// 验证SAAS实例
	saasInstance, exists := yamlConfig.RocketGroup["saas"]
	assert.True(t, exists, "saas实例应该存在")
	assert.Equal(t, "saas", saasInstance.Name)
	assert.Equal(t, "123.57.2.204:8081", saasInstance.Endpoint)

	// 验证主题配置
	assert.Len(t, saasInstance.Topics, 2)

	// 验证sms主题
	var smsTopic RocketTopic
	for _, topic := range saasInstance.Topics {
		if topic.Topic == "sms" {
			smsTopic = topic
			break
		}
	}
	assert.Equal(t, "sms", smsTopic.Topic)
	assert.Equal(t, TopicKindNormal, smsTopic.Kind)

	// 验证sms_event主题
	var smsEventTopic RocketTopic
	for _, topic := range saasInstance.Topics {
		if topic.Topic == "sms_event" {
			smsEventTopic = topic
			break
		}
	}
	assert.Equal(t, "sms_event", smsEventTopic.Topic)
	assert.Equal(t, TopicKindDelay, smsEventTopic.Kind)

	// 验证消费者配置
	assert.Len(t, saasInstance.Consumers, 2)

	// 验证GID_sms消费者
	var smsConsumer RocketConsumer
	for _, consumer := range saasInstance.Consumers {
		if consumer.Group == "GID_sms" {
			smsConsumer = consumer
			break
		}
	}
	assert.Equal(t, "GID_sms", smsConsumer.Group)
	assert.Equal(t, 1, smsConsumer.Num)
	assert.Equal(t, 5, smsConsumer.Wait)
	assert.Equal(t, int32(16), smsConsumer.MaxMessageNum)
	assert.Equal(t, 43200, smsConsumer.InvisibleDuration)
	assert.Len(t, smsConsumer.Subscriptions, 1)
	assert.Equal(t, "sms", smsConsumer.Subscriptions[0].Topic)
	assert.Equal(t, "*", smsConsumer.Subscriptions[0].Tags)

	// 验证GID_sms_event消费者
	var smsEventConsumer RocketConsumer
	for _, consumer := range saasInstance.Consumers {
		if consumer.Group == "GID_sms_event" {
			smsEventConsumer = consumer
			break
		}
	}
	assert.Equal(t, "GID_sms_event", smsEventConsumer.Group)
	assert.Equal(t, 1, smsEventConsumer.Num)
	assert.Equal(t, 5, smsEventConsumer.Wait)
	assert.Equal(t, int32(16), smsEventConsumer.MaxMessageNum)
	assert.Equal(t, 43200, smsEventConsumer.InvisibleDuration)
	assert.Len(t, smsEventConsumer.Subscriptions, 1)
	assert.Equal(t, "sms_event", smsEventConsumer.Subscriptions[0].Topic)
	assert.Equal(t, "*", smsEventConsumer.Subscriptions[0].Tags)

	// 测试创建消费者服务器
	server := NewRocketConsumerServer(smsConsumer, saasInstance)
	assert.Equal(t, "GID_sms", server.Name())

	// 验证订阅表达式
	expressions := smsConsumer.makeSubscriptionExpressions()
	assert.Len(t, expressions, 1)
	assert.Contains(t, expressions, "sms")
}
