package auth

import (
	"sync"
	"time"
)

type CallLimiter struct {
	mu     sync.Mutex
	active map[string]int
}

func NewCallLimiter() *CallLimiter {
	return &CallLimiter{active: map[string]int{}}
}

func (l *CallLimiter) Acquire(accountID string, isOwner bool) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !isOwner && l.active[accountID] > 0 {
		return nil, false
	}
	l.active[accountID]++
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.active[accountID]--
			if l.active[accountID] <= 0 {
				delete(l.active, accountID)
			}
		})
	}, true
}

func (l *CallLimiter) Active(accountID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active[accountID] > 0
}

type AttemptLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	now    func() time.Time
	byKey  map[string]*attempt
}

type attempt struct {
	count int
	reset time.Time
}

func NewAttemptLimiter(max int, window time.Duration) *AttemptLimiter {
	if max <= 0 {
		max = 5
	}
	if window <= 0 {
		window = 15 * time.Minute
	}
	return &AttemptLimiter{
		max:    max,
		window: window,
		now:    time.Now,
		byKey:  map[string]*attempt{},
	}
}

func (l *AttemptLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.byKey[key]
	if a == nil || l.now().After(a.reset) {
		return true
	}
	return a.count < l.max
}

func (l *AttemptLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	a := l.byKey[key]
	if a == nil || now.After(a.reset) {
		l.byKey[key] = &attempt{count: 1, reset: now.Add(l.window)}
		return
	}
	a.count++
	if len(l.byKey) > 10000 {
		for k, v := range l.byKey {
			if now.After(v.reset) {
				delete(l.byKey, k)
			}
		}
	}
}

func (l *AttemptLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.byKey, key)
}
