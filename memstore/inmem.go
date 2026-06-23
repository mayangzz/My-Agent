package memstore

import (
	"context"
	"sync"

	"github.com/mayangzz/My-Agent/harness"
)

// InMem 把历史放进内存 map,重启即丢。开发期默认,零依赖。
type InMem struct {
	mu   sync.RWMutex
	data map[string][]harness.Message
}

func NewInMem() *InMem {
	return &InMem{data: map[string][]harness.Message{}}
}

func (m *InMem) Append(_ context.Context, session string, msg harness.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[session] = append(m.data[session], msg)
	return nil
}

func (m *InMem) Load(_ context.Context, session string) ([]harness.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.data[session]
	out := make([]harness.Message, len(src)) // 复制一份,别让外部改到内部 slice
	copy(out, src)
	return out, nil
}

func (m *InMem) Reset(_ context.Context, session string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, session)
	return nil
}
