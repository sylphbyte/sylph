package sylph

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

var _ logrus.Hook = (*LoggerBufferHook)(nil)

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
}

// Levels 实现 logrus.Hook 接口，返回支持的日志级别
func (h LoggerBufferHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire 实现 logrus.Hook 接口，处理日志条目
func (h LoggerBufferHook) Fire(entry *logrus.Entry) error {
	select {
	case h.logPipe <- entry:
	default:
		fmt.Println("Log channel buffer is full, dropping log entry")
	}
	return nil
}

// watchLogs 监控日志管道并批量处理日志
func (h LoggerBufferHook) watchLogs() {
	ticker := time.NewTicker(h.opt.TakeFlushDuration()) // 定期写入日志
	defer ticker.Stop()

	size := int(h.opt.BufferSize)
	batch := make([]*logrus.Entry, 0, size)

	for {
		select {
		case entry := <-h.logPipe:
			batch = append(batch, entry)
			if len(batch) >= size {
				h.flushLogBatch(batch)
				batch = make([]*logrus.Entry, 0, size)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				h.flushLogBatch(batch)
				batch = make([]*logrus.Entry, 0, size)
			}
		case <-h.quitPipe:
			h.flushLogBatch(batch)
			return
		}
	}
}

// flushLogBatch 批量处理日志
func (h LoggerBufferHook) flushLogBatch(batch []*logrus.Entry) {
	if len(batch) == 0 {
		return
	}

	// 按级别分组
	levelMap := make(map[logrus.Level][]*logrus.Entry)
	for _, entry := range batch {
		levelMap[entry.Level] = append(levelMap[entry.Level], entry)
	}

	for level, entries := range levelMap {
		// 当路径为空时，跳过文件写入
		if h.opt.Path == "" {
			continue
		}

		// 获取对应的writer
		writer := _logWriterManager.GetWriter(h.opt.Path, h.name, level)
		if writer == nil {
			continue
		}

		for _, entry := range entries {
			msg, err := entry.Logger.Formatter.Format(entry)
			if err != nil && h.opt.PrintError {
				fmt.Fprintf(os.Stderr, "Failed to format log entry: %v\n", err)
				continue
			}

			_, err = writer.Write(msg)
			if err != nil && h.opt.PrintError {
				fmt.Fprintf(os.Stderr, "Failed to write log entry: %v\n", err)
			}
		}
	}
}
