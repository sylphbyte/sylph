package sylph

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var _ logrus.Hook = (*LoggerBufferHook)(nil)

// batchPool 用于重用batch切片，减少GC压力
var batchPool = sync.Pool{
	New: func() interface{} {
		return make([]*logrus.Entry, 0, 128) // 预分配合理大小
	},
}

// getBatch 从对象池获取一个batch切片
func getBatch(capacity int) []*logrus.Entry {
	batch := batchPool.Get().([]*logrus.Entry)
	// 确保容量足够
	if cap(batch) < capacity {
		// 如果容量不够，创建新的
		return make([]*logrus.Entry, 0, capacity)
	}
	// 重置长度
	return batch[:0]
}

// putBatch 将batch切片归还到对象池
func putBatch(batch []*logrus.Entry) {
	// 避免持有对条目的引用，帮助GC
	for i := range batch {
		batch[i] = nil
	}
	batchPool.Put(batch)
}

// NewLoggerBufferHook 创建新的日志缓冲钩子
func NewLoggerBufferHook(name string, opt *LoggerConfig) (hook *LoggerBufferHook) {
	hook = &LoggerBufferHook{
		name:     name,
		opt:      opt,
		logPipe:  make(chan *logrus.Entry, opt.BufferCap),
		quitPipe: make(chan struct{}),
	}

	go hook.watchLogs()
	return
}

// LoggerBufferHook 日志缓冲钩子，用于批量处理日志
type LoggerBufferHook struct {
	name     string
	opt      *LoggerConfig
	logPipe  chan *logrus.Entry // 日志写入管道
	quitPipe chan struct{}      // 退出管道
	// 添加缓存实例以提高性能
	levelMapPool sync.Pool
}

// Levels 实现 logrus.Hook 接口，返回支持的日志级别
func (h *LoggerBufferHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire 实现 logrus.Hook 接口，处理日志条目
func (h *LoggerBufferHook) Fire(entry *logrus.Entry) error {
	// 创建一个副本，因为entry在Fire方法返回后可能会被修改
	clone := *entry

	// 克隆Data字段，避免并发修改问题
	clone.Data = make(logrus.Fields, len(entry.Data))
	for k, v := range entry.Data {
		clone.Data[k] = v
	}

	// 尝试写入channel
	select {
	case h.logPipe <- &clone:
		// 成功写入
	default:
		// 如果PrintError为true，则打印警告
		if h.opt.PrintError {
			fmt.Fprintln(os.Stderr, "Log channel buffer is full, dropping log entry level:", entry.Level, "msg:", entry.Message)
		}
	}
	return nil
}

// watchLogs 监控日志管道并批量处理日志
func (h *LoggerBufferHook) watchLogs() {
	ticker := time.NewTicker(h.opt.TakeFlushDuration()) // 定期写入日志
	defer ticker.Stop()

	size := int(h.opt.BufferSize)
	batch := getBatch(size)

	for {
		select {
		case entry := <-h.logPipe:
			batch = append(batch, entry)
			if len(batch) >= size {
				h.flushLogBatch(batch)
				// 重新获取batch
				batch = getBatch(size)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				h.flushLogBatch(batch)
				// 重新获取batch
				batch = getBatch(size)
			}
		case <-h.quitPipe:
			if len(batch) > 0 {
				h.flushLogBatch(batch)
				putBatch(batch) // 归还最后一个batch
			}
			return
		}
	}
}

// getLevelMap 获取级别Map
func (h *LoggerBufferHook) getLevelMap() map[logrus.Level][]*logrus.Entry {
	if h.levelMapPool.New == nil {
		h.levelMapPool.New = func() interface{} {
			return make(map[logrus.Level][]*logrus.Entry)
		}
	}

	m := h.levelMapPool.Get().(map[logrus.Level][]*logrus.Entry)
	// 清空map但保留容量
	for k := range m {
		delete(m, k)
	}
	return m
}

// flushLogBatch 批量处理日志
func (h *LoggerBufferHook) flushLogBatch(batch []*logrus.Entry) {
	if len(batch) == 0 {
		return
	}

	// 如果路径为空，直接返回
	if h.opt.Path == "" {
		putBatch(batch) // 归还batch
		return
	}

	// 按级别分组
	levelMap := h.getLevelMap()

	for _, entry := range batch {
		levelMap[entry.Level] = append(levelMap[entry.Level], entry)
	}

	// 缓存格式化后的消息，避免重复格式化
	formattedMsgs := make(map[*logrus.Entry][]byte, len(batch))

	for level, entries := range levelMap {
		// 获取对应的writer
		writer := _logWriterManager.GetWriter(h.opt.Path, h.name, level)
		if writer == nil {
			continue
		}

		// 先批量格式化所有条目
		for _, entry := range entries {
			msg, err := entry.Logger.Formatter.Format(entry)
			if err != nil {
				if h.opt.PrintError {
					_, _ = fmt.Fprintf(os.Stderr, "Failed to format log entry: %v\n", err)
				}
				continue
			}
			formattedMsgs[entry] = msg
		}

		// 对于多条日志，考虑合并写入以减少IO操作
		if len(entries) > 1 && len(formattedMsgs) > 0 {
			// 估算总大小以预分配buffer
			totalSize := 0
			for _, msg := range formattedMsgs {
				totalSize += len(msg)
			}

			// 创建合并buffer
			mergedBuf := bytes.NewBuffer(make([]byte, 0, totalSize))

			// 合并所有消息
			for _, entry := range entries {
				if msg, ok := formattedMsgs[entry]; ok {
					mergedBuf.Write(msg)
				}
			}

			// 一次性写入所有消息
			_, err := writer.Write(mergedBuf.Bytes())
			if err != nil && h.opt.PrintError {
				fmt.Fprintf(os.Stderr, "Failed to write batch log entries: %v\n", err)
			}
		} else {
			// 对于单条日志，直接写入
			for _, entry := range entries {
				if msg, ok := formattedMsgs[entry]; ok {
					_, err := writer.Write(msg)
					if err != nil && h.opt.PrintError {
						fmt.Fprintf(os.Stderr, "Failed to write log entry: %v\n", err)
					}
				}
			}
		}
	}

	// 清空并归还levelMap
	for k := range levelMap {
		levelMap[k] = nil
	}

	h.levelMapPool.Put(levelMap)

	// 归还batch
	putBatch(batch)
}
