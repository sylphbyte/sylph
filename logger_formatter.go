package sylph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/sirupsen/logrus"
)

const (
	loggerMessageKey = "message"
)

// LoggerFormatter 是一个自定义的 JSON 格式化器，替代 logrus.JSONFormatter
type LoggerFormatter struct {
	TimestampFormat string
	PrettyPrint     bool
	// 添加对象池以减少内存分配
	bufferPool sync.Pool
}

// 初始化对象池
func (f *LoggerFormatter) getBuffer() *bytes.Buffer {
	if f.bufferPool.New == nil {
		f.bufferPool.New = func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 1024)) // 预分配合理大小的缓冲区
		}
	}

	buf := f.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// 返回buffer到对象池
func (f *LoggerFormatter) putBuffer(buf *bytes.Buffer) {
	f.bufferPool.Put(buf)
}

func (f *LoggerFormatter) takeMessage(entry *logrus.Entry) *LoggerMessage {
	value, ok := entry.Data[loggerMessageKey]
	if !ok {
		return nil
	}

	return value.(*LoggerMessage)
}

func (f *LoggerFormatter) extData(entry *logrus.Entry, data logrus.Fields) {
	// 添加其他字段
	for k, v := range entry.Data {
		if k == loggerMessageKey {
			continue
		}

		if _, ok := data[k]; ok {
			continue
		}

		data[k] = v
	}
}

func (f *LoggerFormatter) makeMessageData(message *LoggerMessage) *LoggerFormatMessage {
	if message == nil {
		return &LoggerFormatMessage{}
	}

	return message.MakeLoggerFormatMessage()
}

func (f *LoggerFormatter) makeJsonContent(data *LoggerFormatMessage) ([]byte, error) {
	if !f.PrettyPrint {
		return _json.Marshal(data) // _json 已经是线程安全的 Froze() 实例
	}

	return _json.MarshalIndent(data, "", "  ")
}

// Format 实现 logrus.Formatter 接口
func (f *LoggerFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	defer func() {
		if r := recover(); r != nil {
			// 打印错误信息到标准输出
			fmt.Printf("json format catch error: %v\n", r)

			// 捕获并打印堆栈信息
			bs := make([]byte, 2048)
			length := runtime.Stack(bs, false)
			fmt.Printf("%s\n", string(bs[:length]))

			// 使用结构化日志记录错误信息和堆栈
			entry.WithFields(logrus.Fields{
				"error": r,
				"stack": string(bs[:length]),
				"data":  entry.Data,
			}).Error("Recovered from panic")

			// 优雅退出程序
			os.Exit(1)
		}
	}()

	// 预分配合理大小的 map，减少扩容操作
	data := make(logrus.Fields, len(entry.Data))
	message := f.takeMessage(entry)

	logMessage := f.makeMessageData(message)
	f.extData(entry, data)

	logMessage.Extra = data

	// 序列化为 JSON
	serialized, err := f.makeJsonContent(logMessage)
	if err != nil {
		return nil, err
	}

	// 使用对象池中的buffer
	buf := f.getBuffer()
	defer f.putBuffer(buf)

	// 预估字符串长度，减少内存重分配
	buf.WriteString(entry.Time.Format(f.TimestampFormat))
	buf.WriteByte(' ')

	// 使用 WriteString 代替 fmt.Sprintf 减少内存分配
	buf.WriteByte('[')
	if message != nil && message.Header != nil {
		buf.WriteString(string(message.Header.EndpointVal))
	}
	buf.WriteByte('-')
	buf.WriteString(entry.Level.String())
	buf.WriteByte(']')

	buf.WriteByte(' ')
	buf.WriteByte('<')
	if message != nil {
		buf.WriteString(message.Location)
	}
	buf.WriteByte('>')
	buf.WriteByte(' ')

	if message != nil && message.Marks != nil && len(message.Marks) > 0 {
		buf.WriteString("marks: ")
		markJson, _ := json.Marshal(message.Marks)
		buf.Write(markJson)
		buf.WriteByte(' ')
	}

	if message != nil {
		buf.WriteByte('(')
		buf.WriteString(message.Message)
		buf.WriteByte(')')

		buf.WriteByte(' ')
		buf.WriteByte('|')
		buf.WriteByte('|')
		buf.WriteByte(' ')
	}

	buf.WriteByte(' ')
	buf.Write(serialized)
	buf.WriteByte('\n')

	// 创建一个新的字节切片，避免buffer被回收后数据被覆盖
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	return result, nil
}
