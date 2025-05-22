package main

import (
	"context"
	"fmt"
	"time"

	"redisbarrier"

	"github.com/go-redis/redis/v8"
)

func main() {
	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	// Example 1: Basic barrier usage
	fmt.Println("Example 1: Basic barrier usage")
	barrierKey := "my_barrier"
	barrier := redisbarrier.NewRedisBarrier(rdb, barrierKey, 3, 30*time.Second)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	fmt.Println("Waiting at barrier...")
	err := barrier.Await(ctx)
	if err != nil {
		fmt.Printf("Error waiting at barrier: %v\n", err)
		return
	}
	fmt.Println("All parties have arrived! Proceeding...")

	// Example 2: Barrier with action
	fmt.Println("\nExample 2: Barrier with action")
	actionBarrier := redisbarrier.NewRedisBarrierWithAction(
		rdb,
		"action_barrier",
		2,
		30*time.Second,
		func() error {
			fmt.Println("Barrier action executed!")
			return nil
		},
	)

	// Start two goroutines to demonstrate the barrier
	done := make(chan bool)
	for i := 0; i < 2; i++ {
		go func(id int) {
			fmt.Printf("Goroutine %d waiting at barrier...\n", id)
			err := actionBarrier.Await(ctx)
			if err != nil {
				fmt.Printf("Goroutine %d error: %v\n", id, err)
			} else {
				fmt.Printf("Goroutine %d proceeding...\n", id)
			}
			done <- true
		}(i)
	}

	// Wait for both goroutines to complete
	<-done
	<-done

	// Example 3: Barrier reset and state checking
	fmt.Println("\nExample 3: Barrier reset and state checking")
	resetBarrier := redisbarrier.NewRedisBarrier(rdb, "reset_barrier", 2, 5*time.Second)

	// Start a goroutine that will be interrupted by reset
	go func() {
		fmt.Println("Goroutine waiting at barrier...")
		err := resetBarrier.Await(ctx)
		if err == redisbarrier.ErrBrokenBarrier {
			fmt.Println("Barrier was reset!")
		}
		done <- true
	}()

	// Wait a bit and then reset the barrier
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("Number of waiting parties: %d\n", resetBarrier.GetNumberWaiting())
	fmt.Printf("Total parties required: %d\n", resetBarrier.GetParties())
	fmt.Println("Resetting barrier...")
	resetBarrier.Reset()
	fmt.Printf("Is barrier broken? %v\n", resetBarrier.IsBroken())

	// Wait for the goroutine to finish
	<-done
}
