package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moonrhythm/session"
)

func TestMemory(t *testing.T) {
	t.Parallel()

	s := new(Memory).GCEvery(10 * time.Millisecond)

	opt := session.StoreOption{TTL: time.Millisecond}

	data := make(session.Data)
	data["test"] = "123"

	err := s.Set("a", data, opt)
	assert.NoError(t, err)

	time.Sleep(5 * time.Millisecond)
	b, err := s.Get("a", opt)
	assert.Nil(t, b)
	assert.Error(t, err)

	s.Set("a", data, opt)
	time.Sleep(20 * time.Millisecond)
	_, err = s.Get("a", opt)
	assert.Error(t, err, "expected expired key return error")

	s.Set("a", data, session.StoreOption{TTL: time.Second})
	b, err = s.Get("a", opt)
	assert.NoError(t, err)
	assert.Equal(t, data, b)

	_, err = s.Get("a", session.StoreOption{Rolling: true, TTL: time.Minute})
	assert.NoError(t, err)
	time.Sleep(time.Second)
	_, err = s.Get("a", opt)
	assert.NoError(t, err)

	s.Del("a", opt)
	_, err = s.Get("a", opt)
	assert.Error(t, err)
}

func TestMemoryWithoutTTL(t *testing.T) {
	t.Parallel()

	s := new(Memory).GCEvery(10 * time.Millisecond)

	opt := session.StoreOption{}

	data := make(session.Data)
	data["test"] = "123"

	err := s.Set("a", data, opt)
	assert.NoError(t, err)

	b, err := s.Get("a", opt)
	assert.NoError(t, err)
	assert.Equal(t, data, b)
}