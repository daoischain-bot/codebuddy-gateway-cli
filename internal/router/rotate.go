package router

import (
	"sync"
	"sync/atomic"
	"time"
)

type KeyStatus struct {
	Key      string
	Index    int
	LastUsed time.Time
	LastErr  string
	Failed   bool
	Cooldown time.Time
	Requests int64
	Errors   int64
}

type KeyPool struct {
	keys    []*KeyStatus
	counter uint64
	mu      sync.RWMutex
}

func NewKeyPool(keys []string) *KeyPool {
	pool := &KeyPool{
		keys: make([]*KeyStatus, len(keys)),
	}
	for i, k := range keys {
		pool.keys[i] = &KeyStatus{Key: k, Index: i}
	}
	return pool
}

func (p *KeyPool) Next() *KeyStatus {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(p.keys)
	if n == 0 {
		return nil
	}

	start := int(atomic.AddUint64(&p.counter, 1)) % n
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		key := p.keys[idx]
		if !key.Failed || time.Now().After(key.Cooldown) {
			key.Failed = false
			key.LastUsed = time.Now()
			key.Requests++
			return key
		}
	}

	idx := start % n
	key := p.keys[idx]
	key.Failed = false
	key.LastUsed = time.Now()
	key.Requests++
	return key
}

func (p *KeyPool) MarkFailed(key *KeyStatus, err string, retryable bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key.LastErr = err
	key.Errors++
	if retryable {
		key.Failed = true
		key.Cooldown = time.Now().Add(30 * time.Second)
	} else {
		key.Failed = true
		key.Cooldown = time.Now().Add(5 * time.Minute)
	}
}

func (p *KeyPool) Keys() map[string]*KeyStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	m := make(map[string]*KeyStatus, len(p.keys))
	for _, ks := range p.keys {
		m[ks.Key] = ks
	}
	return m
}

func (p *KeyPool) All() []*KeyStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*KeyStatus, len(p.keys))
	copy(out, p.keys)
	return out
}

func IsRetryableStatus(code int) bool {
	return code == 401 || code == 403 || code == 429 || code >= 500
}
