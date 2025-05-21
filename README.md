# redisbarrier
[![Go Report Card](https://goreportcard.com/badge/github.com/Igarashi-G/redisbarrier)](https://goreportcard.com/report/github.com/Igarashi-G/redisbarrier)
[![Coverage Status](https://coveralls.io/repos/github/Igarashi-G/redisbarrier/badge.svg?branch=master)](https://coveralls.io/github/Igarashi-G/redisbarrier?branch=master)
[![GoDoc](https://godoc.org/github.com/Igarashi-G/redisbarrier?status.svg)](https://godoc.org/github.com/Igarashi-G/redisbarrier)
[![License](https://img.shields.io/github/license/mashape/apistatus.svg?maxAge=2592000)](LICENSE)

**A distributed barrier synchronizer for Go​**

**RedisBarrier** A distributed synchronization barrier implementation using Redis. This library provides a way to synchronize multiple processes or goroutines across different machines using Redis as the coordination mechanism (the barrier).

Inspired by cyclicbarrier https://github.com/marusama/cyclicbarrier

## Installation

```bash
go get github.com/yourusername/redisbarrier
```

## Usage

### Basic Usage

```go
import (
    "context"
    "time"
    "github.com/go-redis/redis/v8"
    "github.com/yourusername/redisbarrier"
)

// Create Redis client
rdb := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "", // no password set
    DB:       0,  // use default DB
})

// Create a barrier that requires 3 parties
barrier := redisbarrier.NewRedisBarrier(rdb, "my_barrier", 3, 30*time.Second)

// Wait at the barrier
ctx := context.Background()
err := barrier.Await(ctx)
if err != nil {
    // Handle error
}
```

### Using Barrier with Action

```go
// Create a barrier with an action
barrier := redisbarrier.NewRedisBarrierWithAction(
    rdb,
    "action_barrier",
    2,
    30*time.Second,
    func() error {
        // This will be executed by the last party to arrive
        return nil
    },
)

// Wait at the barrier
err := barrier.Await(ctx)
```

### Checking Barrier State

```go
// Get the number of parties currently waiting
waiting := barrier.GetNumberWaiting()

// Get the total number of parties required
parties := barrier.GetParties()

// Check if the barrier is broken
isBroken := barrier.IsBroken()
```

### Resetting the Barrier

```go
// Reset the barrier state
barrier.Reset()
```

## Running Tests

1. Make sure Redis server is running on localhost:6379
2. Run the tests:

```bash
go test -v ./...
```

## Examples

Check the `examples` directory for complete usage examples:

```bash
cd examples
go run main.go
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

