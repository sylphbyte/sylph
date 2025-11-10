package sylph

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// 基础创建和获取测试
// =============================================================================

// TestHeaderNewHeader 测试 NewHeader 创建
func TestHeaderNewHeader(t *testing.T) {
	endpoint := Endpoint("api")
	header := NewHeader(endpoint)

	assert.NotNil(t, header)
	assert.Equal(t, endpoint, header.EndpointVal)
	assert.NotEmpty(t, header.TraceIdVal)
}

// TestHeaderEndpoint 测试 Endpoint 获取
func TestHeaderEndpoint(t *testing.T) {
	endpoint := Endpoint("web")
	header := NewHeader(endpoint)

	result := header.Endpoint()
	assert.Equal(t, endpoint, result)
}

// TestHeaderPath 测试 Path 获取和设置
func TestHeaderPath(t *testing.T) {
	header := NewHeader(Endpoint("api"))

	// 初始应该为空
	assert.Empty(t, header.Path())

	// 设置路径
	path := "/api/users/123"
	header.StorePath(path)
	assert.Equal(t, path, header.Path())
}

// TestHeaderStorePath 测试 StorePath 设置
func TestHeaderStorePath(t *testing.T) {
	header := NewHeader(Endpoint("api"))

	// 测试普通路径
	header.StorePath("/api/test")
	assert.Equal(t, "/api/test", header.PathVal)

	// 测试空路径
	header.StorePath("")
	assert.Empty(t, header.PathVal)

	// 测试复杂路径
	complexPath := "/api/v1/users/123/posts/456?filter=active"
	header.StorePath(complexPath)
	assert.Equal(t, complexPath, header.PathVal)
}

// TestHeaderRef 测试 Ref 获取和设置
func TestHeaderRef(t *testing.T) {
	header := NewHeader(Endpoint("api"))

	// 初始应该为空
	assert.Empty(t, header.Ref())

	// 设置引用
	ref := "user-service"
	header.StoreRef(ref)
	assert.Equal(t, ref, header.Ref())
}

// TestHeaderStoreRef 测试 StoreRef 设置
func TestHeaderStoreRef(t *testing.T) {
	header := NewHeader(Endpoint("api"))

	// 测试普通引用
	header.StoreRef("order-service")
	assert.Equal(t, "order-service", header.RefVal)

	// 测试空引用
	header.StoreRef("")
	assert.Empty(t, header.RefVal)
}

// TestHeaderMark 测试 Mark 获取和设置
func TestHeaderMark(t *testing.T) {
	header := NewHeader(Endpoint("api"))

	// 初始应该为空
	assert.Empty(t, header.Mark())

	// 设置标记
	mark := "high-priority"
	header.WithMark(mark)
	assert.Equal(t, mark, header.Mark())
}

// TestHeaderWithMark 测试 WithMark 设置
func TestHeaderWithMark(t *testing.T) {
	header := NewHeader(Endpoint("api"))

	// 测试普通标记
	header.WithMark("urgent")
	assert.Equal(t, "urgent", header.MarkVal)

	// 测试空标记
	header.WithMark("")
	assert.Empty(t, header.MarkVal)

	// 测试覆盖标记
	header.WithMark("normal")
	assert.Equal(t, "normal", header.MarkVal)
}

// TestHeaderIP 测试 IP 获取和设置
func TestHeaderIP(t *testing.T) {
	header := NewHeader(Endpoint("api"))

	// 初始应该为空
	assert.Empty(t, header.IP())

	// 设置IP
	ip := "192.168.1.100"
	header.StoreIP(ip)
	assert.Equal(t, ip, header.IP())
}

// TestHeaderStoreIP 测试 StoreIP 设置
func TestHeaderStoreIP(t *testing.T) {
	header := NewHeader(Endpoint("api"))

	// 测试 IPv4
	header.StoreIP("192.168.1.1")
	assert.Equal(t, "192.168.1.1", header.IPVal)

	// 测试 IPv6
	header.StoreIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334")
	assert.Equal(t, "2001:0db8:85a3:0000:0000:8a2e:0370:7334", header.IPVal)

	// 测试空IP
	header.StoreIP("")
	assert.Empty(t, header.IPVal)
}

// =============================================================================
// TraceId 管理测试
// =============================================================================

// TestHeaderTraceId 测试 TraceId 获取
func TestHeaderTraceId(t *testing.T) {
	header := NewHeader(Endpoint("api"))

	traceId := header.TraceId()
	assert.NotEmpty(t, traceId)

	// 验证 UUID 格式 (36个字符，包含4个破折号)
	assert.Len(t, traceId, 36)
	assert.Equal(t, 4, strings.Count(traceId, "-"))
}

// TestHeaderWithTraceId 测试 WithTraceId 设置
func TestHeaderWithTraceId(t *testing.T) {
	header := NewHeader(Endpoint("api"))

	// 设置自定义 TraceId
	customId := "custom-trace-id-12345"
	header.WithTraceId(customId)
	assert.Equal(t, customId, header.TraceIdVal)

	// 设置空 TraceId
	header.WithTraceId("")
	assert.Empty(t, header.TraceIdVal)
}

// TestHeaderGenerateTraceId 测试 GenerateTraceId 生成
func TestHeaderGenerateTraceId(t *testing.T) {
	header := NewHeader(Endpoint("api"))
	initialId := header.TraceId()

	// 当已有 TraceId 时，不应该重新生成
	header.GenerateTraceId()
	assert.Equal(t, initialId, header.TraceId())

	// 当没有 TraceId 时，应该生成新的
	header.TraceIdVal = ""
	header.GenerateTraceId()
	newId := header.TraceId()
	assert.NotEmpty(t, newId)
	assert.NotEqual(t, initialId, newId)
}

// TestHeaderResetTraceId 测试 ResetTraceId 重置
func TestHeaderResetTraceId(t *testing.T) {
	header := NewHeader(Endpoint("api"))
	initialId := header.TraceId()

	// 重置应该生成新的 TraceId
	header.ResetTraceId()
	newId := header.TraceId()

	assert.NotEmpty(t, newId)
	assert.NotEqual(t, initialId, newId)
	assert.Len(t, newId, 36)
}

// TestHeaderTraceIdFormat 测试 TraceId 格式
func TestHeaderTraceIdFormat(t *testing.T) {
	// 测试多次生成，验证格式一致性
	for i := 0; i < 10; i++ {
		header := NewHeader(Endpoint("api"))
		traceId := header.TraceId()

		// UUID v4 格式验证
		assert.Len(t, traceId, 36)
		assert.Equal(t, 4, strings.Count(traceId, "-"))

		// 位置验证：8-4-4-4-12
		parts := strings.Split(traceId, "-")
		assert.Len(t, parts, 5)
		assert.Len(t, parts[0], 8)
		assert.Len(t, parts[1], 4)
		assert.Len(t, parts[2], 4)
		assert.Len(t, parts[3], 4)
		assert.Len(t, parts[4], 12)
	}
}

// =============================================================================
// Clone 测试
// =============================================================================

// TestHeaderClone 测试 Clone 基本功能
func TestHeaderClone(t *testing.T) {
	// 创建原始 Header 并设置所有字段
	original := NewHeader(Endpoint("api"))
	original.StorePath("/api/users")
	original.StoreRef("test-service")
	original.WithMark("priority")
	original.StoreIP("192.168.1.1")

	// 克隆
	cloned := original.Clone()

	// 验证不是同一个对象
	assert.NotSame(t, original, cloned)

	// Clone 只复制 Endpoint，并生成新的 TraceId
	assert.Equal(t, original.EndpointVal, cloned.EndpointVal)

	// TraceId 应该是新生成的（不同）
	assert.NotEmpty(t, cloned.TraceIdVal)
	assert.NotEqual(t, original.TraceIdVal, cloned.TraceIdVal)

	// 其他字段不会被复制（保持为空）
	assert.Empty(t, cloned.PathVal)
	assert.Empty(t, cloned.RefVal)
	assert.Empty(t, cloned.MarkVal)
	assert.Empty(t, cloned.IPVal)
}

// TestHeaderCloneIndependence 测试 Clone 独立性
func TestHeaderCloneIndependence(t *testing.T) {
	original := NewHeader(Endpoint("api"))
	original.StorePath("/original")

	cloned := original.Clone()

	// 克隆是独立的，修改克隆不会影响原始
	cloned.StorePath("/cloned")
	cloned.StoreRef("cloned-ref")
	cloned.WithMark("cloned-mark")
	cloned.StoreIP("1.1.1.1")

	// 原始对象保持不变
	assert.Equal(t, "/original", original.PathVal)
	assert.Empty(t, original.RefVal)
	assert.Empty(t, original.MarkVal)
	assert.Empty(t, original.IPVal)

	// 克隆对象已修改
	assert.Equal(t, "/cloned", cloned.PathVal)
	assert.Equal(t, "cloned-ref", cloned.RefVal)
	assert.Equal(t, "cloned-mark", cloned.MarkVal)
	assert.Equal(t, "1.1.1.1", cloned.IPVal)
}

// TestHeaderCloneTraceId 测试 Clone 生成新的 TraceId
func TestHeaderCloneTraceId(t *testing.T) {
	original := NewHeader(Endpoint("api"))
	originalTraceId := original.TraceId()

	cloned := original.Clone()

	// Clone 会生成新的 TraceId（不同于原始）
	assert.NotEmpty(t, cloned.TraceId())
	assert.NotEqual(t, originalTraceId, cloned.TraceId())

	// 修改克隆的 TraceId 不应该影响原始
	clonedTraceId := cloned.TraceId()
	cloned.ResetTraceId()
	assert.NotEqual(t, clonedTraceId, cloned.TraceId())
	assert.Equal(t, originalTraceId, original.TraceId())
}

// =============================================================================
// 边界情况和综合测试
// =============================================================================

// TestHeaderEmptyValues 测试空值处理
func TestHeaderEmptyValues(t *testing.T) {
	header := NewHeader(Endpoint(""))

	// 空端点
	assert.Empty(t, header.Endpoint())

	// 未设置的字段应该返回空
	assert.Empty(t, header.Path())
	assert.Empty(t, header.Ref())
	assert.Empty(t, header.Mark())
	assert.Empty(t, header.IP())

	// TraceId 应该自动生成，即使端点为空
	assert.NotEmpty(t, header.TraceId())
}

// TestHeaderSpecialCharacters 测试特殊字符
func TestHeaderSpecialCharacters(t *testing.T) {
	header := NewHeader(Endpoint("api"))

	// 路径中的特殊字符
	specialPath := "/api/用户/测试?key=值&中文=参数"
	header.StorePath(specialPath)
	assert.Equal(t, specialPath, header.Path())

	// Ref 中的特殊字符
	specialRef := "服务-名称_123"
	header.StoreRef(specialRef)
	assert.Equal(t, specialRef, header.Ref())

	// Mark 中的特殊字符
	specialMark := "高优先级-urgent"
	header.WithMark(specialMark)
	assert.Equal(t, specialMark, header.Mark())
}

// TestHeaderAllFields 测试所有字段综合
func TestHeaderAllFields(t *testing.T) {
	header := NewHeader(Endpoint("rpc"))

	// 设置所有字段
	header.StorePath("/rpc/calculate")
	header.StoreRef("math-service")
	header.WithMark("compute-intensive")
	header.StoreIP("10.0.0.5")
	customTraceId := "test-trace-123"
	header.WithTraceId(customTraceId)

	// 验证所有字段
	assert.Equal(t, Endpoint("rpc"), header.Endpoint())
	assert.Equal(t, "/rpc/calculate", header.Path())
	assert.Equal(t, "math-service", header.Ref())
	assert.Equal(t, "compute-intensive", header.Mark())
	assert.Equal(t, "10.0.0.5", header.IP())
	assert.Equal(t, customTraceId, header.TraceId())
}

// TestHeaderMultipleUpdates 测试多次更新
func TestHeaderMultipleUpdates(t *testing.T) {
	header := NewHeader(Endpoint("api"))

	// Path 多次更新
	header.StorePath("/v1/users")
	assert.Equal(t, "/v1/users", header.Path())
	header.StorePath("/v2/users")
	assert.Equal(t, "/v2/users", header.Path())

	// Ref 多次更新
	header.StoreRef("service-v1")
	assert.Equal(t, "service-v1", header.Ref())
	header.StoreRef("service-v2")
	assert.Equal(t, "service-v2", header.Ref())

	// Mark 多次更新
	header.WithMark("low")
	assert.Equal(t, "low", header.Mark())
	header.WithMark("high")
	assert.Equal(t, "high", header.Mark())
}

// TestHeaderImmutableEndpoint 测试 Endpoint 不可变
func TestHeaderImmutableEndpoint(t *testing.T) {
	endpoint := Endpoint("api")
	header := NewHeader(endpoint)

	// Endpoint 在创建后不能通过接口方法修改
	assert.Equal(t, endpoint, header.Endpoint())

	// 修改其他字段不应该影响 Endpoint
	header.StorePath("/test")
	header.StoreRef("ref")
	assert.Equal(t, endpoint, header.Endpoint())
}

// =============================================================================
// 性能基准测试
// =============================================================================

// BenchmarkNewHeader Header 创建性能
func BenchmarkNewHeader(b *testing.B) {
	endpoint := Endpoint("api")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewHeader(endpoint)
	}
}

// BenchmarkHeaderClone Clone 操作性能
func BenchmarkHeaderClone(b *testing.B) {
	header := NewHeader(Endpoint("api"))
	header.StorePath("/api/test")
	header.StoreRef("service")
	header.WithMark("mark")
	header.StoreIP("1.2.3.4")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = header.Clone()
	}
}

// BenchmarkHeaderSetters Setter 操作性能
func BenchmarkHeaderSetters(b *testing.B) {
	header := NewHeader(Endpoint("api"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		header.StorePath("/api/test")
		header.StoreRef("service")
		header.WithMark("mark")
		header.StoreIP("1.2.3.4")
	}
}

// BenchmarkGenerateTraceId TraceId 生成性能
func BenchmarkGenerateTraceId(b *testing.B) {
	for i := 0; i < b.N; i++ {
		header := &Header{}
		header.GenerateTraceId()
	}
}

// BenchmarkResetTraceId TraceId 重置性能
func BenchmarkResetTraceId(b *testing.B) {
	header := NewHeader(Endpoint("api"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		header.ResetTraceId()
	}
}
