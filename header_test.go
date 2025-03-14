package sylph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHeader(t *testing.T) {
	endpoint := &mockStringer{value: "test-endpoint"}
	header := NewHeader(endpoint)

	assert.NotNil(t, header)
	assert.Equal(t, endpoint, header.Endpoint)
	assert.NotEmpty(t, header.TraceId())
}

func TestHeaderMethods(t *testing.T) {
	header := NewHeader(&mockStringer{value: "test-endpoint"})
	initialTraceId := header.TraceId()

	t.Run("Ref and Path", func(t *testing.T) {
		header.RefVal = "test-ref"
		header.PathVal = "test-path"

		assert.Equal(t, "test-ref", header.Ref())
		assert.Equal(t, "test-path", header.Path())
	})

	t.Run("GenerateTraceId", func(t *testing.T) {
		header.GenerateTraceId()
		assert.Equal(t, initialTraceId, header.TraceId())
	})

	t.Run("StoreUserId", func(t *testing.T) {
		header.StoreUserId("123")
		assert.Equal(t, 123, header.UserId)
	})

	t.Run("StorePhone", func(t *testing.T) {
		header.StorePhone("123456789")
		assert.Equal(t, "123456789", header.Phone)
	})

	t.Run("StoreChannel", func(t *testing.T) {
		header.StoreChannelMark("test-channel")
		header.StoreChannelId("456")

		assert.Equal(t, "test-channel", header.ChannelMark)
		assert.Equal(t, 456, header.ChannelId)
	})
}

type mockStringer struct {
	value string
}

func (m *mockStringer) String() string {
	return m.value
}
