package sylph

import (
	"github.com/google/uuid"
)

// Header 实现IHeader接口的头部信息结构体
// 用于在请求处理过程中传递和存储与请求相关的元数据
//
// 功能说明:
//   - 存储和管理请求的核心元数据，如服务端点、路径、跟踪ID等
//   - 提供标准化的请求追踪能力，生成和维护唯一的请求ID
//   - 支持自定义标记，用于请求分类和过滤
//
// 字段说明:
//   - EndpointVal: 服务端点名称，标识请求来源的服务类型
//   - MarkVal: 自定义标记，用于请求分类和特殊处理
//   - RefVal: 来源引用，用于追踪请求在系统间的流转路径
//   - PathVal: 当前路径，表示请求的具体目标
//   - TraceIdVal: 跟踪ID，用于分布式系统中的请求链路追踪
//   - IPVal: 客户端IP地址，用于安全分析和日志记录
//
// 使用示例:
//
//	// 创建新的头部信息
//	header := sylph.NewHeader(sylph.EndpointWeb)
//	header.WithMark("high-priority")
//	header.StorePath("/api/users")
//
//	// 在上下文中使用
//	ctx.StoreHeader(header)
//
//	// 请求追踪
//	traceId := ctx.TakeHeader().TraceId()
//	logger.WithField("trace_id", traceId).Info("处理请求")
type Header struct {
	EndpointVal Endpoint `json:"endpoint"`           // 服务端点名称
	MarkVal     string   `json:"x_mark,omitempty"`   // 自定义标记
	RefVal      string   `json:"x_ref,omitempty"`    // 来源引用，用于追踪请求来源
	PathVal     string   `json:"x_path,omitempty"`   // 当前路径，表示请求的路径
	TraceIdVal  string   `json:"trace_id,omitempty"` // 跟踪ID，用于请求追踪

	IPVal string `json:"ip,omitempty"` // 客户端IP地址
}

// NewHeader 创建新的头部信息实例
// 自动生成唯一的跟踪ID，便于请求追踪
//
// 参数:
//   - endpoint: 服务端点名称，标识请求来源的服务类型
//
// 返回:
//   - *Header: 头部信息实例，已初始化跟踪ID
//
// 使用示例:
//
//	header := sylph.NewHeader(sylph.EndpointWeb)
//	ctx.StoreHeader(header)
func NewHeader(endpoint Endpoint) *Header {
	return &Header{
		EndpointVal: endpoint,
		TraceIdVal:  generateTraceId(),
	}
}

// Endpoint 获取请求来源的服务端点
// 实现IHeader接口的方法
//
// 返回:
//   - Endpoint: 服务端点类型
func (h *Header) Endpoint() Endpoint {
	return h.EndpointVal
}

// Ref 获取来源引用
// 用于了解请求的原始来源信息
//
// 返回:
//   - string: 来源引用标识符
func (h *Header) Ref() string {
	return h.RefVal
}

// StoreRef 存储来源引用
// 设置请求的引用信息，通常包含来源服务或模块的标识
//
// 参数:
//   - ref: 来源引用标识符
//
// 使用示例:
//
//	header.StoreRef("user-service")
func (h *Header) StoreRef(ref string) {
	h.RefVal = ref
}

// Path 获取路径
// 获取请求的目标路径，通常是API路径
//
// 返回:
//   - string: 请求路径
func (h *Header) Path() string {
	return h.PathVal
}

// StorePath 存储路径
// 设置请求的目标路径
//
// 参数:
//   - path: 请求路径，通常是API端点路径
//
// 使用示例:
//
//	header.StorePath("/api/users/123")
func (h *Header) StorePath(path string) {
	h.PathVal = path
}

// TraceId 获取跟踪ID
// 用于在分布式系统中追踪请求流转
//
// 返回:
//   - string: 唯一的跟踪标识符
func (h *Header) TraceId() string {
	return h.TraceIdVal
}

// WithTraceId 设置跟踪ID
// 通常用于保持请求跟踪链路的连续性，如从外部系统接收请求时
//
// 参数:
//   - traceId: 跟踪ID，通常是UUID格式
//
// 使用示例:
//
//	// 从外部系统接收的请求
//	incomingTraceId := externalRequest.GetHeader("X-Trace-ID")
//	if incomingTraceId != "" {
//	    header.WithTraceId(incomingTraceId)
//	}
func (h *Header) WithTraceId(traceId string) {
	h.TraceIdVal = traceId
}

// GenerateTraceId 生成新的跟踪ID
// 如果当前没有跟踪ID才会生成，避免覆盖已有的ID
//
// 逻辑说明:
//   - 检查当前是否已有跟踪ID
//   - 如果没有，则生成新的UUID作为跟踪ID
//
// 注意事项:
//   - 此方法不会覆盖已有的跟踪ID，安全用于确保跟踪ID存在
func (h *Header) GenerateTraceId() {
	if h.TraceIdVal != "" {
		return
	}

	h.TraceIdVal = generateTraceId()
}

// ResetTraceId 强制重置跟踪ID为新值
// 无论是否已有跟踪ID，都会生成新的UUID替换
//
// 使用示例:
//
//	// 在需要开始新的跟踪链路时
//	header.ResetTraceId()
//
// 注意事项:
//   - 谨慎使用此方法，因为它会中断请求的跟踪链路
func (h *Header) ResetTraceId() {
	h.TraceIdVal = generateTraceId()
}

// Mark 获取标记
// 标记通常用于请求分类、优先级标识或特殊处理标志
//
// 返回:
//   - string: 标记值
func (h *Header) Mark() string {
	return h.MarkVal
}

// WithMark 设置标记
// 标记请求的特殊属性，如优先级、来源类型等
//
// 参数:
//   - mark: 标记值
//
// 使用示例:
//
//	header.WithMark("high-priority")
//	header.WithMark("batch-process")
func (h *Header) WithMark(mark string) {
	h.MarkVal = mark
}

// StoreIP 存储IP地址
// 记录客户端IP地址，用于日志审计和安全分析
//
// 参数:
//   - ip: IP地址字符串
//
// 使用示例:
//
//	clientIP := request.RemoteAddr
//	header.StoreIP(clientIP)
func (h *Header) StoreIP(ip string) {
	h.IPVal = ip
}

// IP 获取IP地址
// 获取之前存储的客户端IP地址
//
// 返回:
//   - string: 客户端IP地址
func (h *Header) IP() string {
	return h.IPVal
}

// Clone 克隆头部信息
// 创建一个新的Header实例，复制当前实例的端点并生成新的跟踪ID
//
// 返回:
//   - *Header: 新的Header实例
//
// 使用示例:
//
//	// 为子请求创建新的Header
//	subRequestHeader := originalHeader.Clone()
//
// 注意事项:
//   - 克隆操作会保留原始端点，但生成新的跟踪ID
//   - 其他字段如标记、路径等不会被复制，需要根据需要单独设置
func (h *Header) Clone() *Header {
	header := &Header{
		EndpointVal: h.EndpointVal,
		TraceIdVal:  generateTraceId(),
	}

	return header
}

// generateTraceId 生成唯一的跟踪ID
// 使用UUID作为跟踪ID，保证全局唯一性
//
// 返回:
//   - string: UUID格式的跟踪ID
//
// 注意事项:
//   - 此函数使用google/uuid库生成高质量的随机UUID
//   - 生成的UUID符合RFC 4122标准
func generateTraceId() string {
	return uuid.NewString()
}
