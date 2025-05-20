// Package redisbarrier implements a distributed barrier synchronization primitive using Redis.
// A barrier is a synchronization primitive that enables multiple processes to wait for each other
// at a certain point before proceeding.
package redisbarrier

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	// ErrBrokenBarrier error used when a barrier is in a broken state
	ErrBrokenBarrier = errors.New("broken redis barrier")
)

// Barrier is a synchronizer that allows a set of goroutines to wait for each other
// to reach a common execution point, also called a barrier.
// Barriers are useful in programs involving a fixed sized party of goroutines
// that must occasionally wait for each other.
type Barrier interface {
	// Await waits until all parties have invoked await on this barrier.
	// If the barrier is reset while any goroutine is waiting, or if the barrier is broken when await is invoked,
	// or while any goroutine is waiting, then ErrBrokenBarrier is returned.
	// If any goroutine is interrupted by ctx.Done() while waiting, then all other waiting goroutines
	// will return ErrBrokenBarrier and the barrier is placed in the broken state.
	// If the current goroutine is the last goroutine to arrive, and a non-nil barrier action was supplied in the constructor,
	// then the current goroutine runs the action before allowing the other goroutines to continue.
	// If an error occurs during the barrier action then that error will be returned and the barrier is placed in the broken state.
	Await(ctx context.Context) error

	// Reset resets the barrier to its initial state.
	// If any parties are currently waiting at the barrier, they will return with a ErrBrokenBarrier.
	Reset()

	// GetNumberWaiting returns the number of parties currently waiting at the barrier.
	GetNumberWaiting() int

	// GetParties returns the number of parties required to trip this barrier.
	GetParties() int

	// IsBroken queries if this barrier is in a broken state.
	// Returns true if one or more parties broke out of this barrier due to interruption by ctx.Done() or the last reset,
	// or a barrier action failed due to an error; false otherwise.
	IsBroken() bool
}

// RedisBarrier represents a distributed barrier synchronization primitive using Redis.
// It allows multiple processes to wait for each other at a certain point before proceeding.
type RedisBarrier struct {
	client     *redis.Client
	barrierKey string
	releaseKey string
	parties    int
	timeout    time.Duration
	action     func() error // optional barrier action
	broken     bool
}

// NewRedisBarrier creates a new RedisBarrier instance.
// Parameters:
//   - client: Redis client instance
//   - barrierKey: Unique key for the barrier
//   - parties: Number of processes that need to wait at the barrier
//   - timeout: Maximum time to wait for all parties to arrive
func NewRedisBarrier(client *redis.Client, barrierKey string, parties int, timeout time.Duration) Barrier {
	if parties <= 0 {
		panic("parties must be positive number")
	}
	return &RedisBarrier{
		client:     client,
		barrierKey: barrierKey,
		releaseKey: barrierKey + ":release",
		parties:    parties,
		timeout:    timeout,
	}
}

// NewRedisBarrierWithAction creates a new RedisBarrier instance with a barrier action.
// The action will be executed by the last party that arrives at the barrier.
func NewRedisBarrierWithAction(client *redis.Client, barrierKey string, parties int, timeout time.Duration, action func() error) Barrier {
	if parties <= 0 {
		panic("parties must be positive number")
	}
	return &RedisBarrier{
		client:     client,
		barrierKey: barrierKey,
		releaseKey: barrierKey + ":release",
		parties:    parties,
		timeout:    timeout,
		action:     action,
	}
}

// Await blocks until all parties have called Await on this barrier.
// Returns an error if the wait times out or if there's a Redis error.
func (b *RedisBarrier) Await(ctx context.Context) error {
	if b.broken {
		return ErrBrokenBarrier
	}

	script := redis.NewScript(`
    local n = redis.call("INCR", KEYS[1])
    if n == 1 then
		local timeout = tonumber(ARGV[1])
		if timeout < 1 then timeout = 1 end
        redis.call("EXPIRE", KEYS[1], timeout)
    end
    return n
	`)

	timeoutSec := int(math.Round(b.timeout.Seconds()))
	n, err := script.Run(ctx, b.client, []string{b.barrierKey}, timeoutSec).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("script execution failed for barrier %s", b.barrierKey)
		}
		return err
	}

	if _, ok := n.(int64); !ok {
		return errors.New("redis result n type is not int64")
	}

	if n == int64(b.parties) {
		// Last party arrived, execute barrier action if exists
		if b.action != nil {
			if err := b.action(); err != nil {
				b.breakBarrier()
				return err
			}
		}
		// Release all waiting parties
		for i := 0; i < b.parties-1; i++ {
			b.client.RPush(ctx, b.releaseKey, "go")
		}
		// Clean up barrier and release keys
		b.client.Del(ctx, b.barrierKey)
		b.client.Del(ctx, b.releaseKey)
	} else {
		// Wait for release signal
		_, err = b.client.BLPop(ctx, b.timeout, b.releaseKey).Result()
		if err != nil {
			if err == redis.Nil {
				return fmt.Errorf("barrier %s wait timeout", b.barrierKey)
			}
			return err
		}
	}

	return nil
}

// Reset resets the barrier to its initial state.
// If any parties are currently waiting at the barrier, they will return with ErrBrokenBarrier.
func (b *RedisBarrier) Reset() {
	b.breakBarrier()
	b.client.Del(context.Background(), b.barrierKey)
	b.client.Del(context.Background(), b.releaseKey)
}

// GetNumberWaiting returns the number of parties currently waiting at the barrier.
func (b *RedisBarrier) GetNumberWaiting() int {
	val, err := b.client.Get(context.Background(), b.barrierKey).Int()
	if err != nil {
		return 0
	}
	return val
}

// GetParties returns the number of parties required to trip this barrier.
func (b *RedisBarrier) GetParties() int {
	return b.parties
}

// IsBroken queries if this barrier is in a broken state.
func (b *RedisBarrier) IsBroken() bool {
	return b.broken
}

// breakBarrier marks the barrier as broken.
func (b *RedisBarrier) breakBarrier() {
	b.broken = true
}
