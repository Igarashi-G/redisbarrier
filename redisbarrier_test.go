package redisbarrier

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
)

func setupTestRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
}

func TestNewRedisBarrier(t *testing.T) {
	client := setupTestRedis()
	barrier := NewRedisBarrier(client, "test_barrier", 3, 30*time.Second)

	assert.NotNil(t, barrier)
	assert.Equal(t, 3, barrier.GetParties())
}

func TestNewRedisBarrierWithAction(t *testing.T) {
	client := setupTestRedis()
	actionCalled := false
	action := func() error {
		actionCalled = true
		return nil
	}

	barrier := NewRedisBarrierWithAction(client, "test_barrier_action", 1, 30*time.Second, action)
	assert.NotNil(t, barrier)
	assert.Equal(t, 1, barrier.GetParties())

	// Test action execution
	err := barrier.Await(context.Background())
	assert.NoError(t, err)
	assert.True(t, actionCalled)
}

func TestBarrierAwait(t *testing.T) {
	client := setupTestRedis()
	ctx := context.Background()

	// Clean up any existing keys
	client.Del(ctx, "test_barrier")
	client.Del(ctx, "test_barrier:release")

	barrier := NewRedisBarrier(client, "test_barrier", 2, 5*time.Second)

	// Test timeout
	err := barrier.Await(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "wait timeout")

	// Test successful barrier
	barrier2 := NewRedisBarrier(client, "test_barrier2", 1, 5*time.Second)
	err = barrier2.Await(ctx)
	assert.NoError(t, err)
}

func TestBarrierConcurrent(t *testing.T) {
	client := setupTestRedis()
	ctx := context.Background()

	// Clean up any existing keys
	client.Del(ctx, "concurrent_barrier")
	client.Del(ctx, "concurrent_barrier:release")

	barrier := NewRedisBarrier(client, "concurrent_barrier", 2, 5*time.Second)

	// Create a channel to signal completion
	done := make(chan bool)

	// Start first goroutine
	go func() {
		err := barrier.Await(ctx)
		assert.NoError(t, err)
		done <- true
	}()

	// Start second goroutine
	go func() {
		err := barrier.Await(ctx)
		assert.NoError(t, err)
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done
}

func TestBarrierReset(t *testing.T) {
	client := setupTestRedis()
	ctx := context.Background()

	barrier := NewRedisBarrier(client, "test_barrier_reset", 2, 5*time.Second)

	// Start a goroutine that will be interrupted by reset
	done := make(chan bool)
	go func() {
		err := barrier.Await(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrBrokenBarrier, err)
		done <- true
	}()

	// Wait a bit to ensure the goroutine has started waiting
	time.Sleep(100 * time.Millisecond)

	// Reset the barrier
	barrier.Reset()

	// Wait for the goroutine to finish
	<-done

	// Verify barrier is broken
	assert.True(t, barrier.IsBroken())
}

func TestBarrierWithActionError(t *testing.T) {
	client := setupTestRedis()
	ctx := context.Background()

	expectedErr := errors.New("action error")
	action := func() error {
		return expectedErr
	}

	barrier := NewRedisBarrierWithAction(client, "test_barrier_action_error", 1, 5*time.Second, action)

	// Test action error
	err := barrier.Await(ctx)
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.True(t, barrier.IsBroken())
}

func TestBarrierGetParties(t *testing.T) {
	client := setupTestRedis()
	barrier := NewRedisBarrier(client, "test_barrier_parties", 5, 30*time.Second)
	assert.Equal(t, 5, barrier.GetParties())
}

func TestBarrierGetNumberWaiting(t *testing.T) {
	client := setupTestRedis()
	ctx := context.Background()
	barrier := NewRedisBarrier(client, "test_barrier_waiting", 2, 5*time.Second)

	// Start a goroutine that will wait at the barrier
	done := make(chan bool)
	go func() {
		barrier.Await(ctx)
		done <- true
	}()

	// Wait a bit to ensure the goroutine has started waiting
	time.Sleep(100 * time.Millisecond)

	// Check number of waiting parties
	assert.Equal(t, 1, barrier.GetNumberWaiting())

	// Clean up
	barrier.Reset()
	<-done
}
