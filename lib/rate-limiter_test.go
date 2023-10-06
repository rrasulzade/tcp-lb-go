package lib

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTokenBucket(t *testing.T) {
	require := require.New(t)

	defaultCapacity := uint64(10)
	defaulRefillRate := uint64(2)

	t.Run("NewTokenBucket", func(t *testing.T) {
		tb := newTokenBucket(defaultCapacity, defaulRefillRate)
		require.Equal(defaultCapacity, tb.capacity)
		require.Equal(defaultCapacity, tb.tokens)
		require.Equal(defaulRefillRate, tb.refillRate)
	})

	t.Run("Take token successfully", func(t *testing.T) {
		tb := newTokenBucket(defaultCapacity, defaulRefillRate)
		require.True(tb.takeToken())
		require.Equal(uint64(9), tb.tokens)
	})

	t.Run("Fail to take token", func(t *testing.T) {
		tb := newTokenBucket(0, uint64(1))
		require.False(tb.takeToken())
	})

	t.Run("Refill tokens correctly", func(t *testing.T) {
		tb := newTokenBucket(defaultCapacity, defaulRefillRate)
		tb.tokens = 0
		time.Sleep(2 * time.Second)
		tb.refillTokens()
		require.Equal(uint64(4), tb.tokens)
	})

	t.Run("Do not exceed capacity", func(t *testing.T) {
		tb := newTokenBucket(defaultCapacity, defaulRefillRate)
		time.Sleep(2 * time.Second)
		tb.refillTokens()
		require.Equal(defaultCapacity, tb.tokens)
	})
}

func TestRateLimiter(t *testing.T) {
	require := require.New(t)

	defaultCapacity := uint64(5)
	defaulRefillRate := uint64(1)
	rl := newRateLimiter(defaultCapacity, defaulRefillRate)

	t.Run("Allow on first connection", func(t *testing.T) {
		clientID := "client1"
		require.True(rl.allowConnection(clientID))
	})

	t.Run("Deny after exhausting tokens", func(t *testing.T) {
		clientID := "client1"
		for i := 0; i < 10; i++ {
			rl.allowConnection(clientID)
		}
		require.False(rl.allowConnection(clientID))
	})

	t.Run("Allow after tokens refill", func(t *testing.T) {
		clientID := "client1"
		time.Sleep(2 * time.Second)
		require.True(rl.allowConnection(clientID))
	})

	t.Run("New client added", func(t *testing.T) {
		clientID := "client2"
		_, exists := rl.clientBuckets[clientID]
		require.False(exists)
		rl.allowConnection(clientID)
		_, exists = rl.clientBuckets[clientID]
		require.True(exists)
	})

	t.Run("Concurrent access", func(t *testing.T) {
		clientID := "client3"
		done := make(chan bool)

		go func() {
			defer func() {
				done <- true
				close(done)
			}()

			for i := 0; i < 100; i++ {
				require.NotPanics(func() {
					rl.allowConnection(clientID)
				}, "Panic occurred during concurrent access.")
			}
		}()

		for i := 0; i < 100; i++ {
			require.NotPanics(func() {
				rl.allowConnection(clientID)
			}, "Panic occurred during concurrent access.")
		}
		<-done
	})

	t.Run("Zero values", func(t *testing.T) {
		clientID := "client1"
		rl1 := newRateLimiter(0, defaulRefillRate)
		require.False(rl1.allowConnection(clientID))

		rl2 := newRateLimiter(defaultCapacity, 0)
		require.True(rl2.allowConnection(clientID))
	})

	t.Run("MultipleClients", func(t *testing.T) {
		rl := newRateLimiter(defaultCapacity, defaulRefillRate)
		numClients := 10

		for i := 0; i < numClients; i++ {
			clientID := fmt.Sprintf("client%d", i)
			rl.allowConnection(clientID)
		}

		require.Equal(numClients, len(rl.clientBuckets))
	})
}
