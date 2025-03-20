package sylph

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockServer struct {
	mock.Mock
}

func (m *mockServer) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockServer) Boot() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockServer) Shutdown() error {
	args := m.Called()
	return args.Error(0)
}

func TestProject(t *testing.T) {
	t.Run("Mounts", func(t *testing.T) {
		p := NewProject()
		srv := &mockServer{}
		srv.On("Name").Return("test-server")

		p.Mounts(srv)

		assert.Equal(t, 1, len(p.servers))
		assert.Equal(t, srv, p.servers[0])
	})

	t.Run("BootsSuccess", func(t *testing.T) {
		p := NewProject()
		srv := &mockServer{}
		srv.On("Name").Return("test-server")
		srv.On("Boot").Return(nil)

		p.Mounts(srv)
		err := p.Boots()

		assert.NoError(t, err)
		srv.AssertExpectations(t)
	})

	t.Run("BootsFailure", func(t *testing.T) {
		p := NewProject()
		srv1 := &mockServer{}
		srv2 := &mockServer{}
		expectedErr := errors.New("boot failed")

		srv1.On("Name").Return("test-server-1")
		srv2.On("Name").Return("test-server-2")
		srv1.On("Boot").Return(nil)
		srv2.On("Boot").Return(expectedErr)
		srv1.On("Shutdown").Return(nil)

		p.Mounts(srv1, srv2)
		err := p.Boots()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), expectedErr.Error())
		srv1.AssertExpectations(t)
		srv2.AssertExpectations(t)
	})

	t.Run("ShutdownsSuccess", func(t *testing.T) {
		p := NewProject()
		srv := &mockServer{}
		srv.On("Name").Return("test-server")
		srv.On("Boot").Return(nil)
		srv.On("Shutdown").Return(nil)

		p.Mounts(srv)
		_ = p.Boots()
		err := p.Shutdowns()

		assert.NoError(t, err)
		srv.AssertExpectations(t)
	})

	t.Run("ShutdownsFailure", func(t *testing.T) {
		p := NewProject()
		srv := &mockServer{}
		expectedErr := errors.New("shutdown failed")

		srv.On("Name").Return("test-server")
		srv.On("Boot").Return(nil)
		srv.On("Shutdown").Return(expectedErr)

		p.Mounts(srv)
		_ = p.Boots()
		err := p.Shutdowns()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), expectedErr.Error())
		srv.AssertExpectations(t)
	})
}
