package sylph

import (
	"hash/fnv"
	"sync"
)

// eventHandlerMap 单个事件处理器映射
type eventHandlerMap struct {
	sync.RWMutex
	handlers map[string][]EventHandler
}

// newEventHandlerMap 创建新的事件处理器映射
func newEventHandlerMap() *eventHandlerMap {
	return &eventHandlerMap{
		handlers: make(map[string][]EventHandler, 8),
	}
}

// event 独立事件系统
// 基于发布/订阅模式实现的事件处理系统，支持同步和异步事件触发
type event struct {
	// 使用32个分片减少锁竞争
	shards [32]*eventHandlerMap
}

// fnv32 计算字符串的32位哈希值
func fnv32(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// getShard 根据事件名称获取对应的分片
func (es *event) getShard(eventName string) *eventHandlerMap {
	return es.shards[fnv32(eventName)%32]
}

// EventHandler 定义事件处理函数类型
// 接收上下文对象和事件载荷作为参数
type EventHandler func(ctx Context, payload interface{})

// newEvent 创建新的事件系统
// 返回初始化的事件系统实例
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
// 参数:
//   - eventName: 事件名称
//   - handlers: 一个或多个事件处理函数
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
// 参数:
//   - eventName: 要取消订阅的事件名称
func (es *event) Off(eventName string) {
	shard := es.getShard(eventName)
	shard.Lock()
	defer shard.Unlock()

	delete(shard.handlers, eventName)
}

// getHandlers 获取指定事件的所有处理函数
// 返回处理函数的副本以避免并发修改问题
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
// 参数:
//   - ctx: 上下文对象
//   - eventName: 事件名称
//   - payload: 事件载荷
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
// 参数:
//   - ctx: 上下文对象
//   - eventName: 事件名称
//   - payload: 事件载荷
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
// 参数:
//   - ctx: 上下文对象
//   - eventName: 事件名称
//   - payload: 事件载荷
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
