// Package workerpool provides a simple worker pool for testing external spawner.
package workerpool

// Pool is a worker pool.
type Pool struct{}

// Submit submits a task to the pool.
// This function spawns goroutines with the given func.
func (p *Pool) Submit(fn func()) {
	go fn()
}

// Run runs tasks with the pool.
// This function spawns goroutines with the given funcs.
func Run(fn func()) {
	go fn()
}
