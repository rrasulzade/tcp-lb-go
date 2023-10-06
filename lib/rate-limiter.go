package lib

import (
	"sync"
	"time"
)

// tokenBucket represents a token bucket rate limiter.
type tokenBucket struct {
	// capacity is the maximum number of tokens the bucket can hold.
	capacity uint64

	// tokens is the number of tokens currently present in the bucket.
	tokens uint64

	// refillRate is the number of tokens added to the bucket every second.
	refillRate uint64

	// fractionalTokens keeps track of accumulated fractional tokens.
	fractionalTokens float64

	// lastRefillTime is a timestamp of the last time tokens refilled.
	lastRefillTime time.Time
}

// newTokenBucket initializes and returns a new tokenBucket.
func newTokenBucket(capacity, refillRate uint64) *tokenBucket {
	return &tokenBucket{
		capacity:       capacity,
		tokens:         capacity,
		refillRate:     refillRate,
		lastRefillTime: time.Now(),
	}
}

// refillTokens refills the bucket based on the elapsed
// time since the last refill.
func (tb *tokenBucket) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime).Seconds()
	refillAmount := elapsed * float64(tb.refillRate)

	// Split the refillAmount into whole and fractional parts
	wholeTokens := uint64(refillAmount)
	tb.fractionalTokens += refillAmount - float64(wholeTokens)

	// If fractionalTokens accumulates to a whole token, add it to tokens
	if tb.fractionalTokens >= 1 {
		wholeTokens++
		tb.fractionalTokens--
	}

	if refillAmount > 0 {
		tb.tokens = min(tb.capacity, tb.tokens+wholeTokens)
		tb.lastRefillTime = now
	}
}

// takeToken attempts to take a token from the bucket.
func (tb *tokenBucket) takeToken() bool {
	// refresh the bucket
	tb.refillTokens()

	if tb.tokens == 0 {
		return false
	}

	tb.tokens--
	return true
}

// rateLimiter represents rate limiting capabilities
// for multiple clients using the token bucket algorithm.
type rateLimiter struct {
	// mu ensures concurrent access to the clientBuckets map.
	mu sync.Mutex

	// bucketCapacity is a default capacity for a new client bucket.
	bucketCapacity uint64

	// bucketRefillRate is a default refill rate for a new client bucket.
	bucketRefillRate uint64

	// clientBuckets is map from clientID to a tokenBucket.
	clientBuckets map[string]*tokenBucket
}

// newRateLimiter initializes and returns a new rateLimiter
// with the specified default bucket parameters.
func newRateLimiter(bucketCapacity, bucketRefillRate uint64) *rateLimiter {
	return &rateLimiter{
		clientBuckets:    make(map[string]*tokenBucket),
		bucketCapacity:   bucketCapacity,
		bucketRefillRate: bucketRefillRate,
	}
}

// AllowConnection checks if a client is allowed
// to make a connection based on their rate limits.
// If the client doesn't have an associated tokenBucket, one is created.
// TODO leverage 'funtional option pattern' to make token bucket params
// configurable per client if necessary
func (rl *rateLimiter) allowConnection(clientID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.clientBuckets[clientID]
	if !exists {
		bucket = newTokenBucket(rl.bucketCapacity, rl.bucketRefillRate)
		rl.clientBuckets[clientID] = bucket
	}

	return bucket.takeToken()
}
