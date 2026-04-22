package helper

import "time"

// SystemClock implements the Clock interface using the real system clock.
type SystemClock struct{}

// NewSystemClock constructs a SystemClock.
func NewSystemClock() *SystemClock { return &SystemClock{} }

// Now returns the current UTC time.
func (c *SystemClock) Now() time.Time { return time.Now().UTC() }
