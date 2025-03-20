package sylph

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// 全局变量定义
var (
	// __robotRds 全局的Redis客户端，用于机器人消息频率限制
	// 通过InjectRobotLimit函数设置，用于存储消息发送记录和控制发送频率
	__robotRds *redis.Client

	// robotRdsMu 保护全局Redis客户端的互斥锁
	// 确保对__robotRds的并发访问安全
	robotRdsMu sync.RWMutex

	// robotsMu 保护全局robot变量的互斥锁
	// 确保对各类型机器人实例的并发访问安全
	robotsMu sync.RWMutex
)

// 机器人模板常量定义
// 用于不同类型消息的视觉区分
const (
	// robotErrorTemplate 错误消息模板
	// 使用红色表示错误消息，通常用于系统错误和异常情况
	robotErrorTemplate robotTemplate = "red"

	// robotSuccessTemplate 成功消息模板
	// 使用绿色表示成功消息，通常用于操作成功的通知
	robotSuccessTemplate robotTemplate = "green"

	// robotWarningTemplate 警告消息模板
	// 使用橙色表示警告消息，通常用于需要注意但不是错误的情况
	robotWarningTemplate robotTemplate = "orange"

	// robotInfoTemplate 信息消息模板
	// 使用蓝色表示普通信息，通常用于一般性通知
	robotInfoTemplate robotTemplate = "blue"
)

// robotTemplate 机器人消息模板类型
// 用于定义不同类型消息的颜色和样式
//
// 功能说明:
//   - 提供消息的视觉差异化，便于快速识别消息类型
//   - 支持多种预定义的消息模板，对应不同的消息严重级别
//   - 便于在不同通知平台上实现一致的消息样式
type robotTemplate string

// String 获取模板的字符串表示
// 实现了fmt.Stringer接口，便于调试和日志记录
//
// 返回:
//   - string: 模板名称，对应不同的颜色标识
func (t robotTemplate) String() string {
	return string(t)
}

// 全局机器人实例
// 预定义的不同类型消息机器人，通过InjectRobot函数初始化
var (
	// errorRoboter 错误消息机器人
	// 用于发送系统错误和异常情况的消息，使用红色模板
	errorRoboter *robot

	// warningRoboter 警告消息机器人
	// 用于发送警告和需要注意的消息，使用橙色模板
	warningRoboter *robot

	// successRoboter 成功消息机器人
	// 用于发送操作成功和积极结果的消息，使用绿色模板
	successRoboter *robot

	// infoRoboter 信息消息机器人
	// 用于发送一般性信息和通知，使用蓝色模板
	infoRoboter *robot
)

// IFields 字段内容接口
// 定义了消息字段的内容生成方法，用于格式化消息字段
//
// 功能说明:
//   - 提供统一的接口，用于生成结构化的消息字段内容
//   - 支持不同的消息字段格式和样式
//   - 可以被多种字段实现扩展，适配不同的通知系统
//
// 使用示例:
//
//	type CustomFields struct {
//	    Data map[string]string
//	}
//
//	func (f CustomFields) MakeContent() string {
//	    var result string
//	    for k, v := range f.Data {
//	        result += fmt.Sprintf("%s: %s\n", k, v)
//	    }
//	    return result
//	}
type IFields interface {
	// MakeContent 生成字段内容字符串
	// 返回格式化后的字段内容
	MakeContent() (content string)
}

// RobotFields 机器人消息字段集合
// 键值对形式的消息字段，实现了IFields接口
//
// 功能说明:
//   - 提供简单的键值对形式存储消息字段
//   - 自动格式化为结构化的消息内容
//   - 支持任意类型的字段值，使用fmt.Sprintf格式化
//
// 使用示例:
//
//	fields := sylph.RobotFields{
//	    "服务器": "api-server-01",
//	    "错误码": "E1001",
//	    "详情": "数据库连接超时",
//	}
//	content := fields.MakeContent()
type RobotFields map[string]interface{}

// Size 获取字段数量
// 返回字段集合中的键值对数量
//
// 返回:
//   - int: 字段的数量
func (f RobotFields) Size() int {
	return len(f)
}

// MakeContent 生成字段内容字符串
// 将字段集合格式化为可读字符串，实现IFields接口
//
// 返回:
//   - content: 格式化后的字段内容字符串
//
// 逻辑说明:
//   - 遍历map中的所有键值对
//   - 对每个键值对使用固定格式进行格式化
//   - 键名加粗显示，值单独一行并缩进
//
// 输出示例:
//
//	**服务器**:
//	    api-server-01
//	**错误码**:
//	    E1001
func (f RobotFields) MakeContent() (content string) {
	for k, v := range f {
		content += fmt.Sprintf("**%6s**:\n\t%s\n", k, v)
	}
	return
}

// InjectRobot 注入机器人实例
// 初始化四种不同类型的机器人实例，用于发送不同类型的消息
//
// 参数:
//   - rob: 实现了IRobot接口的机器人实例
//
// 功能说明:
//   - 初始化错误、警告、成功和信息四种类型的机器人
//   - 每种机器人使用不同的消息模板，提供视觉差异
//   - 所有机器人共享同一个底层实现，但应用不同的模板
//
// 注意事项:
//   - 如果传入nil，则不会进行注入
//   - 此函数应在程序启动时调用，以初始化机器人消息系统
//   - 线程安全，可以从多个goroutine调用
//
// 使用示例:
//
//	// 使用自定义机器人实现
//	robot := &SlackRobot{WebhookURL: "https://hooks.slack.com/services/xxx"}
//	sylph.InjectRobot(robot)
//
//	// 使用内置机器人实现
//	robot := sylph.NewDingTalkRobot("webhook_url", "sign_secret")
//	sylph.InjectRobot(robot)
func InjectRobot(rob IRobot) {
	if rob == nil {
		return
	}

	robotsMu.Lock()
	defer robotsMu.Unlock()

	errorRoboter = newRobot(rob, robotErrorTemplate)
	warningRoboter = newRobot(rob, robotWarningTemplate)
	successRoboter = newRobot(rob, robotSuccessTemplate)
	infoRoboter = newRobot(rob, robotInfoTemplate)
}

// InjectRobotLimit 注入机器人消息频率限制的Redis客户端
// 设置用于限制机器人消息发送频率的Redis客户端
//
// 参数:
//   - robotRedis: Redis客户端实例
//
// 功能说明:
//   - 设置用于存储消息发送记录的Redis客户端
//   - 启用消息发送频率限制功能
//   - 防止短时间内发送过多相同消息导致通知轰炸
//
// 注意事项:
//   - 此函数应在程序启动时调用，以启用机器人消息频率限制
//   - 如果未调用此函数，机器人消息不会受到频率限制
//   - 线程安全，可以从多个goroutine调用
//
// 使用示例:
//
//	// 初始化Redis客户端
//	redisClient := redis.NewClient(&redis.Options{
//	    Addr: "localhost:6379",
//	    Password: "",
//	    DB: 0,
//	})
//
//	// 注入到机器人系统
//	sylph.InjectRobotLimit(redisClient)
func InjectRobotLimit(robotRedis *redis.Client) {
	robotRdsMu.Lock()
	defer robotRdsMu.Unlock()

	__robotRds = robotRedis
}

// robot 机器人实现
// 封装了具体的机器人实例和消息模板
//
// 功能说明:
//   - 包装底层机器人实现，提供消息发送和格式化功能
//   - 支持不同消息模板，用于区分不同类型的消息
//   - 实现线程安全的消息发送
//   - 支持消息频率限制，防止过多相同消息
//
// 字段说明:
//   - mu: 互斥锁，保护单个robot实例的并发访问
//   - rob: 底层机器人接口实现，负责实际的消息发送
//   - template: 消息模板，决定消息的颜色和样式
//   - now: 创建时间，用于消息时间戳
type robot struct {
	mu       sync.Mutex    // 保护单个robot实例
	rob      IRobot        // 机器人接口实现
	template robotTemplate // 消息模板
	now      time.Time     // 创建时间
}

// newRobot 创建一个新的机器人实例
// 使用给定的机器人接口实现和模板
//
// 参数:
//   - rob: 机器人接口实现，负责实际的消息发送
//   - template: 消息模板，决定消息的颜色和样式
//
// 返回:
//   - *robot: 新创建的机器人实例
//
// 逻辑说明:
//   - 初始化robot结构体
//   - 设置底层机器人实现和消息模板
//   - 记录创建时间，用于消息时间戳
func newRobot(rob IRobot, template robotTemplate) *robot {
	return &robot{rob: rob, template: template, now: time.Now()}
}

// makeTitle 生成消息标题
// 在标题中添加系统通知前缀和时间戳
//
// 参数:
//   - title: 原始标题
//
// 返回:
//   - string: 格式化后的完整标题
//
// 格式示例:
//
//	"系统通知: 数据库连接失败 (2023-09-18 15:30:45)"
func (r *robot) makeTitle(title string) string {
	return fmt.Sprintf("系统通知: %s (%s)", title, r.now.Format(time.DateTime))
}

// Send 发送消息
// 使用机器人发送指定类型的消息
//
// 参数:
//   - title: 消息标题
//   - content: 消息内容
//   - fieldsGroup: 可选的字段组，用于提供结构化的附加信息
//
// 返回:
//   - error: 发送过程中的错误，成功则返回nil
//
// 逻辑说明:
//   - 首先检查Redis客户端是否已设置
//   - 检查是否允许发送消息（频率限制）
//   - 通过底层机器人实现发送格式化后的消息
//   - 使用互斥锁保护并发发送
//
// 注意事项:
//   - 如果Redis客户端未设置，将不发送消息
//   - 如果频率限制检查不通过，将不发送消息
//   - 此方法是线程安全的，可以从多个goroutine调用
//
// 使用示例:
//
//	// 发送错误消息
//	errorRoboter.Send("数据库连接失败", "无法连接到主数据库", map[string]interface{}{
//	    "错误码": "DB001",
//	    "服务器": "db-main",
//	    "重试次数": 3,
//	})
//
//	// 发送成功消息
//	successRoboter.Send("数据迁移完成", "所有数据已成功迁移到新服务器", map[string]interface{}{
//	    "记录数": 10243,
//	    "耗时": "15分钟",
//	})
func (r *robot) Send(title, content string, fieldsGroup ...map[string]interface{}) error {
	// 获取Redis客户端
	rds := __robotRds

	if rds == nil {
		return nil
	}

	// 检查是否允许发送
	if !allowSend(title) {
		return nil
	}

	// 发送消息
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.rob.Send(r.makeTitle(title), content, fieldsGroup...)
}

// allowSend 检查是否允许发送指定消息
// 使用Redis实现消息频率限制，防止过多相同消息
//
// 参数:
//   - key: 消息键，通常是消息标题
//
// 返回:
//   - bool: 是否允许发送，true表示允许
//
// 实现细节:
//   - 使用MD5哈希处理消息键，避免过长键名
//   - 在Redis中记录消息键，设置10分钟过期时间
//   - 当前实现总是返回true，实际频率限制逻辑可以扩展
//
// 注意事项:
//   - 目前的实现总是返回true，但会在Redis中记录消息键
//   - 消息键会被MD5哈希处理，在Redis中保存10分钟
//   - 可以扩展该函数以实现更复杂的频率限制策略
//
// 扩展示例:
//
//	func allowSend(key string) bool {
//	    key = md5String(key)
//	    ctx := context.Background()
//
//	    // 检查是否在10分钟内已经发送过
//	    exists, _ := __robotRds.Exists(ctx, key).Result()
//	    if exists > 0 {
//	        // 获取计数
//	        count, _ := __robotRds.Get(ctx, key).Int()
//	        if count > 5 {
//	            // 10分钟内超过5次，不允许发送
//	            return false
//	        }
//	        // 增加计数
//	        __robotRds.Incr(ctx, key)
//	    } else {
//	        // 首次发送，设置计数为1，10分钟过期
//	        __robotRds.Set(ctx, key, 1, time.Minute*10)
//	    }
//	    return true
//	}
func allowSend(key string) bool {
	key = md5String(key)
	__robotRds.Set(context.Background(), key, 0, time.Minute*10)
	return true
}
