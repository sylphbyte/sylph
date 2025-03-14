package sylph

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"runtime"
)

const (
	loggerMessageKey = "message"
)

// XLoggerFormatter 是一个自定义的 JSON 格式化器，替代 logrus.JSONFormatter
type XLoggerFormatter struct {
	TimestampFormat string
	PrettyPrint     bool
}

func (f *XLoggerFormatter) takeMessage(entry *logrus.Entry) *LoggerMessage {
	value, ok := entry.Data[loggerMessageKey]
	if !ok {
		return nil
	}

	return value.(*LoggerMessage)
}

func (f *XLoggerFormatter) extData(entry *logrus.Entry, data logrus.Fields) {
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

func (f *XLoggerFormatter) makeMessageData(message *LoggerMessage) *LoggerFormatMessage {
	if message == nil {
		return &LoggerFormatMessage{}
	}

	return message.MakeLoggerFormatMessage()
}

func (f *XLoggerFormatter) makeJsonContent(data *LoggerFormatMessage) (serialized []byte, err error) {
	if !f.PrettyPrint {
		serialized, err = _json.Marshal(data)
		return
	}

	serialized, err = _json.MarshalIndent(data, "", "  ")
	return
}

// Format 实现 logrus.Formatter 接口
func (f *XLoggerFormatter) Format(entry *logrus.Entry) ([]byte, error) {
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

	data := make(logrus.Fields)
	message := f.takeMessage(entry)

	logMessage := f.makeMessageData(message)
	f.extData(entry, data)

	logMessage.Extra = data

	// 序列化为 JSON
	serialized, err := f.makeJsonContent(logMessage)
	if err != nil {
		return nil, err
	}

	ts := entry.Time.Format(f.TimestampFormat)
	buf := bytes.NewBuffer([]byte(ts))
	buf.WriteByte(' ')
	buf.WriteString(fmt.Sprintf("[%s-%s]", message.Header.Endpoint.String(), entry.Level.String()))
	buf.WriteByte(' ')
	buf.WriteString(fmt.Sprintf("<%s>", message.Location))
	buf.WriteByte(' ')
	buf.WriteString(message.Message)
	buf.WriteByte(' ')
	buf.Write(serialized)
	buf.WriteByte('\n')

	return buf.Bytes(), nil
}
