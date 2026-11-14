package auth

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const limiterPrefix = "switchboard:limit:"

type CallLimiter struct {
	mu     sync.Mutex
	active map[string]int
	redis  *redis.Client
	ttl    time.Duration
}

func NewCallLimiter() *CallLimiter {
	return &CallLimiter{active: map[string]int{}}
}

// NewRedisCallLimiter shares one-call-per-account state across Go processes.
// The TTL is a safety net if a release is missed.
func NewRedisCallLimiter(client *redis.Client, ttl time.Duration) *CallLimiter {
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	return &CallLimiter{redis: client, ttl: ttl}
}

var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

func (l *CallLimiter) Acquire(accountID string, isOwner bool) (func(), bool) {
	if isOwner {
		return func() {}, true
	}
	key := limiterPrefix + "call:" + accountID
	if l.redis != nil {
		token, err := randomToken()
		if err != nil {
			return nil, false
		}
		ok, err := l.redis.SetNX(context.Background(), key, token, l.ttl).Result()
		if err != nil || !ok {
			return nil, false
		}
		var once sync.Once
		return func() {
			once.Do(func() {
				releaseScript.Run(context.Background(), l.redis, []string{key}, token)
			})
		}, true
	}
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
	key := limiterPrefix + "call:" + accountID
	if l.redis != nil {
		n, err := l.redis.Exists(context.Background(), key).Result()
		return err == nil && n > 0
	}
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
	redis  *redis.Client
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

func NewRedisAttemptLimiter(client *redis.Client, max int, window time.Duration) *AttemptLimiter {
	l := NewAttemptLimiter(max, window)
	l.redis = client
	return l
}

func (l *AttemptLimiter) Allow(key string) bool {
	if l.redis != nil {
		n, err := l.redis.Get(context.Background(), limiterPrefix+"login:"+key).Int()
		if err == redis.Nil {
			return true
		}
		if err != nil {
			// Redis is a coordination layer, not an excuse to lock everyone out.
			return true
		}
		return n < l.max
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.byKey[key]
	if a == nil || l.now().After(a.reset) {
		return true
	}
	return a.count < l.max
}

func (l *AttemptLimiter) Fail(key string) {
	if l.redis != nil {
		k := limiterPrefix + "login:" + key
		n, err := l.redis.Incr(context.Background(), k).Result()
		if err == nil && n == 1 {
			l.redis.Expire(context.Background(), k, l.window)
		}
		return
	}
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
	if l.redis != nil {
		l.redis.Del(context.Background(), limiterPrefix+"login:"+key)
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.byKey, key)
}
