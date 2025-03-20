package sylph

import (
	"hash/fnv"
	"sync"
)

// eventHandlerMap 单个事件处理器映射
// 存储特定事件的处理器函数列表，并提供并发安全访问
//
// 功能说明:
//   - 管理特定事件的订阅处理器
//   - 提供线程安全的读写操作
//   - 通过读写锁实现高并发下的性能优化
//
// 字段说明:
//   - RWMutex: 读写锁，保护处理器映射的并发访问
//   - handlers: 事件名称到处理器列表的映射
type eventHandlerMap struct {
	sync.RWMutex
	handlers map[string][]EventHandler
}

// newEventHandlerMap 创建新的事件处理器映射
// 初始化一个具有合适初始容量的事件处理器映射
//
// 返回:
//   - *eventHandlerMap: 初始化的事件处理器映射实例
func newEventHandlerMap() *eventHandlerMap {
	return &eventHandlerMap{
		handlers: make(map[string][]EventHandler, 8),
	}
}

// event 独立事件系统
// 基于发布/订阅模式实现的事件处理系统，支持同步和异步事件触发
//
// 功能说明:
//   - 提供事件订阅和发布机制
//   - 支持同步和异步事件处理
//   - 使用分片技术减少锁竞争，提高并发性能
//   - 支持等待和非等待两种异步触发模式
//
// 实现原理:
//   - 使用32个分片存储事件处理器，根据事件名称哈希值分配
//   - 每个分片有独立的读写锁，避免全局锁竞争
//   - 事件触发时复制处理器列表，避免并发修改问题
//
// 使用示例:
//
//	// 创建事件系统
//	eventSystem := newEvent()
//
//	// 订阅事件
//	eventSystem.On("user.created", func(ctx Context, payload interface{}) {
//	    user := payload.(User)
//	    ctx.Info("event.handler", "新用户创建", map[string]interface{}{
//	        "user_id": user.ID,
//	        "username": user.Name,
//	    })
//	})
//
//	// 触发事件
//	user := User{ID: "123", Name: "张三"}
//	eventSystem.Emit(ctx, "user.created", user)
//
//	// 异步触发事件
//	eventSystem.AsyncEmitNoWait(ctx, "user.login", user)
type event struct {
	// 使用32个分片减少锁竞争
	shards [32]*eventHandlerMap
}

// fnv32 计算字符串的32位哈希值
// 基于FNV-1a哈希算法，用于事件分片索引计算
//
// 参数:
//   - s: 要计算哈希的字符串
//
// 返回:
//   - uint32: 32位哈希值
//
// 逻辑说明:
//   - 使用标准库的fnv哈希实现
//   - 对输入字符串的每个字节进行哈希计算
//   - 返回固定长度的32位无符号整数结果
func fnv32(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// getShard 根据事件名称获取对应的分片
// 通过哈希函数将事件名映射到特定分片
//
// 参数:
//   - eventName: 事件名称
//
// 返回:
//   - *eventHandlerMap: 对应的事件处理器映射分片
//
// 逻辑说明:
//   - 计算事件名称的哈希值
//   - 对分片数量取模，确定分片索引
//   - 返回对应的分片引用
func (es *event) getShard(eventName string) *eventHandlerMap {
	return es.shards[fnv32(eventName)%32]
}

// EventHandler类型已在define.go中定义

// newEvent 创建新的事件系统
// 初始化一个具有32个分片的事件系统实例
//
// 返回:
//   - *event: 初始化完成的事件系统
//
// 使用示例:
//
//	eventSystem := newEvent()
//	// 之后可以使用eventSystem订阅和发布事件
func newEvent() *event {
	e := &event{}
	// 初始化所有分片
	for i := 0; i < 32; i++ {
		e.shards[i] = newEventHandlerMap()
	}
	return e
}

// On 订阅事件
// 将处理函数添加到指定事件的订阅列表中
//
// 参数:
//   - eventName: 事件名称
//   - handlers: 一个或多个事件处理函数
//
// 使用示例:
//
//	eventSystem.On("order.created", func(ctx Context, payload interface{}) {
//	    order := payload.(Order)
//	    // 处理订单创建事件
//	    sendOrderNotification(order)
//	})
//
//	// 多个处理器
//	eventSystem.On("user.login",
//	    func(ctx Context, payload interface{}) { updateLastLogin(payload) },
//	    func(ctx Context, payload interface{}) { recordLoginAttempt(payload) },
//	)
//
// 注意事项:
//   - 如果提供了空的处理器列表，函数将直接返回
//   - 处理器会追加到现有列表，而不是替换
//   - 同一个事件可以多次调用On添加多个处理器
func (es *event) On(eventName string, handlers ...EventHandler) {
	if len(handlers) == 0 {
		return
	}

	shard := es.getShard(eventName)
	shard.Lock()
	defer shard.Unlock()

	if existingHandlers, ok := shard.handlers[eventName]; ok {
		shard.handlers[eventName] = append(existingHandlers, handlers...)
	} else {
		shard.handlers[eventName] = handlers
	}
}

// Off 取消订阅特定事件
// 从订阅映射中移除指定事件的所有处理函数
//
// 参数:
//   - eventName: 要取消订阅的事件名称
//
// 使用示例:
//
//	// 取消订阅特定事件的所有处理器
//	eventSystem.Off("user.login")
//
// 注意事项:
//   - 此操作会移除指定事件的所有处理器
//   - 无法选择性地移除单个处理器
//   - 如果事件不存在，操作不会产生错误
func (es *event) Off(eventName string) {
	shard := es.getShard(eventName)
	shard.Lock()
	defer shard.Unlock()

	delete(shard.handlers, eventName)
}

// getHandlers 获取指定事件的所有处理函数
// 返回处理函数的副本以避免并发修改问题
//
// 参数:
//   - eventName: 事件名称
//
// 返回:
//   - []EventHandler: 处理函数列表的副本，如果事件不存在则返回nil
//
// 逻辑说明:
//   - 获取事件对应的分片
//   - 用读锁保护获取处理器列表
//   - 创建处理器列表的副本并返回
//   - 如果事件不存在，返回nil
func (es *event) getHandlers(eventName string) []EventHandler {
	shard := es.getShard(eventName)
	shard.RLock()
	defer shard.RUnlock()

	handlers, ok := shard.handlers[eventName]
	if !ok {
		return nil
	}

	// 创建副本避免并发修改问题
	result := make([]EventHandler, len(handlers))
	copy(result, handlers)
	return result
}

// Emit 同步触发事件
// 同步调用所有订阅了指定事件的处理函数
//
// 参数:
//   - ctx: 上下文对象
//   - eventName: 事件名称
//   - payload: 事件载荷
//
// 使用示例:
//
//	// 同步触发事件
//	data := map[string]interface{}{
//	    "item_id": "12345",
//	    "quantity": 2,
//	}
//	eventSystem.Emit(ctx, "cart.item.added", data)
//
// 注意事项:
//   - 同步执行会阻塞直到所有处理器完成
//   - 处理器按添加顺序依次执行
//   - 如果某个处理器发生panic，会中断后续处理器的执行
//   - 如果没有处理器订阅事件，函数会立即返回
func (es *event) Emit(ctx Context, eventName string, payload interface{}) {
	handlers := es.getHandlers(eventName)
	if len(handlers) == 0 {
		return
	}

	for _, h := range handlers {
		h(ctx, payload)
	}
}

// AsyncEmit 异步触发事件并等待所有处理完成
// 异步调用所有订阅了指定事件的处理函数，并使用WaitGroup等待全部完成
//
// 参数:
//   - ctx: 上下文对象
//   - eventName: 事件名称
//   - payload: 事件载荷
//
// 使用示例:
//
//	// 异步触发事件并等待所有处理器完成
//	order := Order{ID: "ORD-123", Total: 99.99}
//	eventSystem.AsyncEmit(ctx, "order.processed", order)
//	// 此处代码会等待所有事件处理完成后执行
//	fmt.Println("所有订单处理完成")
//
// 注意事项:
//   - 函数会等待所有处理器完成后才返回
//   - 处理器并发执行，无法保证执行顺序
//   - 使用SafeGo保证单个处理器的panic不会影响其他处理器
//   - 如果没有处理器订阅事件，函数会立即返回
func (es *event) AsyncEmit(ctx Context, eventName string, payload interface{}) {
	handlers := es.getHandlers(eventName)
	if len(handlers) == 0 {
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(handlers))

	for _, h := range handlers {
		// 创建副本避免闭包问题
		handler := h
		SafeGo(ctx, "event.AsyncEmit", func() {
			defer wg.Done()
			handler(ctx, payload)
		})
	}

	// 等待所有事件处理完成
	wg.Wait()
}

// AsyncEmitNoWait 异步触发事件但不等待完成
// 异步调用所有订阅了指定事件的处理函数，但不等待它们完成
//
// 参数:
//   - ctx: 上下文对象
//   - eventName: 事件名称
//   - payload: 事件载荷
//
// 使用示例:
//
//	// 异步触发事件但不等待完成
//	logInfo := map[string]string{
//	    "action": "user_login",
//	    "user_id": "12345",
//	    "timestamp": time.Now().String(),
//	}
//	eventSystem.AsyncEmitNoWait(ctx, "system.log", logInfo)
//	// 此处代码会立即执行，不等待事件处理完成
//
// 注意事项:
//   - 函数不会等待处理器完成，立即返回
//   - 适用于不需要等待结果的后台任务
//   - 处理器并发执行，无法保证执行顺序
//   - 使用SafeGo保证单个处理器的panic不会影响系统稳定性
//   - 如果没有处理器订阅事件，函数会立即返回
func (es *event) AsyncEmitNoWait(ctx Context, eventName string, payload interface{}) {
	handlers := es.getHandlers(eventName)
	if len(handlers) == 0 {
		return
	}

	for _, h := range handlers {
		// 创建副本避免闭包问题
		handler := h
		SafeGo(ctx, "event.AsyncEmitNoWait", func() {
			handler(ctx, payload)
		})
	}
}
