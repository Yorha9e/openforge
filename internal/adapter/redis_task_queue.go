package adapter

import (
	"context"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"openforge/internal/shared/kernel"
)

const queueCap = 100

// RedisTaskQueue implements kernel.TaskQueue.
// Attempts a Redis connection; on failure uses an in-memory channel-based queue.
type RedisTaskQueue struct {
	addr     string
	password string
	redisOK  bool
	logger   *log.Logger

	mu     sync.Mutex
	queues map[string]chan kernel.Message
}

// NewRedisTaskQueue creates a RedisTaskQueue. If addr is empty, it reads
// the REDIS_URL env var (default "localhost:6379"). If Redis is unreachable,
// it falls back to an in-memory queue and logs a warning.
func NewRedisTaskQueue(addr, password string) *RedisTaskQueue {
	if addr == "" {
		addr = os.Getenv("REDIS_URL")
		if addr == "" {
			addr = "localhost:6379"
		}
	}
	// Strip redis:// scheme if present
	if strings.HasPrefix(addr, "redis://") {
		if u, err := url.Parse(addr); err == nil {
			if h := u.Host; h != "" {
				addr = h
			}
			if p, ok := u.User.Password(); ok && password == "" {
				password = p
			}
		}
	}

	q := &RedisTaskQueue{
		addr:     addr,
		password: password,
		logger:   log.New(log.Writer(), "[redis-task-queue] ", log.LstdFlags|log.Lmsgprefix),
		queues:   make(map[string]chan kernel.Message),
	}

	if q.tryRedis() {
		q.redisOK = true
		q.logger.Printf("connected to Redis at %s", addr)
	} else {
		q.logger.Printf("Redis at %s unreachable; using in-memory fallback", addr)
	}

	return q
}

func (q *RedisTaskQueue) tryRedis() bool {
	conn, err := net.DialTimeout("tcp", q.addr, 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("*1\r\n$4\r\nPING\r\n")); err != nil {
		return false
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 32)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}
	return strings.HasPrefix(string(buf[:n]), "+PONG")
}

func (q *RedisTaskQueue) getOrCreateChan(topic string) chan kernel.Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	ch, ok := q.queues[topic]
	if !ok {
		ch = make(chan kernel.Message, queueCap)
		q.queues[topic] = ch
	}
	return ch
}

// Enqueue adds a message to the queue. The priority is stored in msg.Priority
// for future use by the Redis-backed implementation.
func (q *RedisTaskQueue) Enqueue(ctx context.Context, topic string, msg kernel.Message, priority int) error {
	if q.redisOK {
		// TODO(phase-8): use go-redis XAdd with priority in message fields.
	}
	msg.Priority = priority
	ch := q.getOrCreateChan(topic)
	select {
	case ch <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Dequeue blocks until a message is available for the given topic.
func (q *RedisTaskQueue) Dequeue(ctx context.Context, topic string) (kernel.Message, error) {
	if q.redisOK {
		// TODO(phase-8): use go-redis XRead when go-redis is integrated.
	}
	ch := q.getOrCreateChan(topic)
	select {
	case msg := <-ch:
		return msg, nil
	case <-ctx.Done():
		return kernel.Message{}, ctx.Err()
	}
}

// Ack is a no-op; messages are removed from the queue on Dequeue.
func (q *RedisTaskQueue) Ack(_ context.Context, _ string, _ string) error {
	return nil
}

// compile-time interface check
var _ kernel.TaskQueue = (*RedisTaskQueue)(nil)
