package sylph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHeader(t *testing.T) {
	endpoint := Endpoint("test-endpoint")
	header := NewHeader(endpoint)

	assert.NotNil(t, header)
	assert.Equal(t, endpoint, header.Endpoint)
	assert.NotEmpty(t, header.TraceId())
}

func TestHeaderMethods(t *testing.T) {
	header := NewHeader(Endpoint("test-endpoint"))
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

}

type mockStringer struct {
	value string
}

func (m *mockStringer) String() string {
	return m.value
}
