// Stub package for testing
package conc

// WaitGroup is a safer version of sync.WaitGroup.
type WaitGroup struct{}

// Go spawns a new goroutine in the WaitGroup.
func (wg *WaitGroup) Go(f func()) {}

// Wait blocks until all spawned goroutines exit.
func (wg *WaitGroup) Wait() {}

// WaitAndRecover blocks until all spawned goroutines exit.
func (wg *WaitGroup) WaitAndRecover() *Recovered { return nil }

// Recovered holds information about a recovered panic.
type Recovered struct{}
