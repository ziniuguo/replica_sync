package main

import (
	"sync"
)

// ConcurrentBoolean is a thread-safe boolean type.
type ConcurrentBoolean struct {
	value bool
	mu    sync.Mutex
}

// Set updates the boolean value.
func (cb *ConcurrentBoolean) Set(value bool) {
	cb.value = value
}

// Get retrieves the current boolean value.
func (cb *ConcurrentBoolean) Get() bool {
	return cb.value
}
