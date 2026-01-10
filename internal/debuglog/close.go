package debuglog

import (
	"io"
	"log"
)

// RunAndLog executes fn and logs label-prefixed error if it fails.
func RunAndLog(label string, fn func() error) {
	if err := fn(); err != nil {
		log.Printf("%s: %v", label, err)
	}
}

// CloseWithLog closes the provided io.Closer and logs an error with context if closing fails.
// Safe to call with a nil closer.
func CloseWithLog(name string, c io.Closer) {
	if c == nil {
		return
	}
	RunAndLog(name, c.Close)
}
