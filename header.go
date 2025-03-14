package sylph

import (
	"github.com/google/uuid"
)

// Endpoint 表示服务端点名称的类型
type Endpoint string

// IHeader 头部信息接口
// 定义了处理请求头部信息的方法集合
type IHeader interface {
	Ref() string                // 获取来源引用
	StoreRef(ref string)        // 存储来源引用
	Path() string               // 获取路径
	StorePath(path string)      // 存储路径
	TraceId() string            // 获取跟踪ID
	WithTraceId(traceId string) // 设置跟踪ID
	GenerateTraceId()           // 生成新的跟踪ID
	ResetTraceId()              // 重置跟踪ID为新值
	Mark() string               // 获取标记
	WithMark(mark string)       // 设置标记
	StoreIP(ip string)          // 存储IP地址
	IP() string                 // 获取IP地址
}

// Header 实现IHeader接口的头部信息结构体
type Header struct {
	Endpoint   Endpoint `json:"endpoint"`           // 服务端点名称
	MarkVal    string   `json:"x_mark,omitempty"`   // 自定义标记
	RefVal     string   `json:"x_ref,omitempty"`    // 来源引用，用于追踪请求来源
	PathVal    string   `json:"x_path,omitempty"`   // 当前路径，表示请求的路径
	TraceIdVal string   `json:"trace_id,omitempty"` // 跟踪ID，用于请求追踪

	IPVal string `json:"ip,omitempty"` // 客户端IP地址
}

// NewHeader 创建新的头部信息实例
// 参数:
//   - endpoint: 服务端点名称
//
// 返回:
//   - *Header: 头部信息实例，已初始化跟踪ID
func NewHeader(endpoint Endpoint) *Header {
	return &Header{
		Endpoint:   endpoint,
		TraceIdVal: generateTraceId(),
	}
}

// Ref 获取来源引用
func (h *Header) Ref() string {
	return h.RefVal
}

// StoreRef 存储来源引用
// 参数:
//   - ref: 来源引用
func (h *Header) StoreRef(ref string) {
	h.RefVal = ref
}

// Path 获取路径
func (h *Header) Path() string {
	return h.PathVal
}

// StorePath 存储路径
// 参数:
//   - path: 路径
func (h *Header) StorePath(path string) {
	h.PathVal = path
}

// TraceId 获取跟踪ID
func (h *Header) TraceId() string {
	return h.TraceIdVal
}

// WithTraceId 设置跟踪ID
// 参数:
//   - traceId: 跟踪ID
func (h *Header) WithTraceId(traceId string) {
	h.TraceIdVal = traceId
}

// GenerateTraceId 生成新的跟踪ID
// 如果已有跟踪ID则不进行操作
func (h *Header) GenerateTraceId() {
	if h.TraceIdVal != "" {
		return
	}

	h.TraceIdVal = generateTraceId()
}

// ResetTraceId 强制重置跟踪ID为新值
func (h *Header) ResetTraceId() {
	h.TraceIdVal = generateTraceId()
}

// Mark 获取标记
func (h *Header) Mark() string {
	return h.MarkVal
}

// WithMark 设置标记
// 参数:
//   - mark: 标记值
func (h *Header) WithMark(mark string) {
	h.MarkVal = mark
}

// StoreIP 存储IP地址
// 参数:
//   - ip: IP地址
func (h *Header) StoreIP(ip string) {
	h.IPVal = ip
}

// IP 获取IP地址
func (h *Header) IP() string {
	return h.IPVal
}

// Clone 克隆头部信息
// 返回一个新的Header实例，复制当前实例的端点并生成新的跟踪ID
func (h *Header) Clone() (header *Header) {
	header = &Header{
		Endpoint:   h.Endpoint,
		TraceIdVal: generateTraceId(),
	}

	return
}

// generateTraceId 生成唯一的跟踪ID
// 使用UUID作为跟踪ID
func generateTraceId() string {
	return uuid.NewString()
}
