package sylph

/*
# github.com/json-iterator/go 库文档

## 概述
json-iterator/go 是由滴滴开源的高性能 JSON 解析库，完全兼容标准库 encoding/json，提供了更高的性能和更丰富的功能。
该库可以作为 Go 标准库 encoding/json 的直接替代品，提供完全一致的 API 接口，同时性能有显著提升。

## 主要特点
1. 高性能：在大多数情况下比标准库快 6-10 倍
2. 内存友好：内存分配次数和大小远低于标准库
3. API 完全兼容：可以直接替换 encoding/json，无需修改代码逻辑
4. 功能丰富：提供了标准库不具备的扩展功能
5. 迭代器模式：提供流式 API 进行低层次控制

## 基本用法示例
```go
// 序列化
data := struct{ Name string }{"张三"}
bytes, err := jsoniter.Marshal(data)

// 反序列化
var result struct{ Name string }
err := jsoniter.Unmarshal(bytes, &result)
```

## 配置选项
json-iterator 提供了多种配置选项，在当前项目中使用的配置如下：
- EscapeHTML: true - HTML 字符会被转义，如 < 会变成 \u003c
- SortMapKeys: true - 序列化 map 时会对键进行排序，确保输出一致性
- ValidateJsonRawMessage: true - 验证 json.RawMessage 格式的正确性
- UseNumber: true - 对数字使用 json.Number 类型，避免精度丢失

## 性能优势
- 单遍扫描：仅需一次遍历完成解析
- 最少内存分配：减少内存分配次数
- 流式处理：从流中拉取数据而非一次性加载
- 字符串优化：特别优化了字符串处理
- 跳过优化：高效地跳过不需要的数据

## 高级特性
1. Any API：无需预定义结构就能访问 JSON 数据
   ```go
   value := jsoniter.Get([]byte(`{"user": {"name": "张三"}}`), "user", "name").ToString()
   // 输出: 张三
   ```

2. 迭代器 API：提供流式处理大型 JSON 数据
   ```go
   iter := jsoniter.Parse(jsoniter.ConfigDefault, reader, 4096)
   for field := iter.ReadObject(); field != ""; field = iter.ReadObject() {
       // 处理每个字段
   }
   ```

3. 自定义扩展：通过 Extension 接口自定义编解码行为

4. 字段映射：支持自定义字段名映射

5. 模糊模式：允许类型转换，如数字到字符串的自动转换

## 与标准库对比
相比标准库 encoding/json，json-iterator 在处理中等大小的负载时：
- 解码速度快约 6 倍 (5623 ns/op vs 35510 ns/op)
- 解码内存分配减少约 92% (160 B/op vs 1960 B/op)
- 编码速度快约 2.6 倍 (837 ns/op vs 2213 ns/op)
- 编码内存分配减少约 46% (384 B/op vs 712 B/op)

## 使用场景
1. 高性能 JSON 处理需求
2. API 服务器处理大量 JSON 请求/响应
3. 处理大型 JSON 文件
4. 需要在不修改现有代码的情况下提升 JSON 处理性能

## 兼容性说明
json-iterator 完全实现了 encoding/json 的所有功能，包括：
- Marshal/Unmarshal 接口
- Encoder/Decoder 接口
- MarshalJSON/UnmarshalJSON 自定义方法
- MarshalText/UnmarshalText 自定义方法
- 所有标签选项 (omitempty, string 等)
*/

import jsoniter "github.com/json-iterator/go"

var (
	_json = jsoniter.Config{
		EscapeHTML:             true,
		SortMapKeys:            true,
		ValidateJsonRawMessage: true,
		UseNumber:              true,
	}.Froze()
)
