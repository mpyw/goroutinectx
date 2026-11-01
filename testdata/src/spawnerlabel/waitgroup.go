package spawnerlabel

import (
	"fmt"
	"sync"
)

// [BAD]: Missing label - calls sync.WaitGroup.Go with func arg
func missingLabelWaitgroup() { // want `function "missingLabelWaitgroup" should have //goroutinectx:spawner directive \(calls sync\.WaitGroup\.Go with func argument\)`
	var wg sync.WaitGroup
	wg.Go(func() {
		fmt.Println("work")
	})
	wg.Wait()
}

// [GOOD]: Traditional WaitGroup pattern (Add/Done) - not spawn method
func goodTraditionalWaitgroup() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("work")
	}()
	wg.Wait()
}

// [BAD]: Multiple spawn types including WaitGroup.Go
func multipleSpawnTypesWithWaitgroup() { // want `function "multipleSpawnTypesWithWaitgroup" should have //goroutinectx:spawner directive \(calls sync\.WaitGroup\.Go with func argument\)`
	var wg sync.WaitGroup
	wg.Go(func() {
		fmt.Println("work")
	})
	wg.Wait()
}

// [BAD]: Spawning closure called through a Phi (#306)
//
// Each value the Phi joins is the closure, so the call runs the spawn.
func badWaitgroupClosurePhiCall(flag bool) { // want `function "badWaitgroupClosurePhiCall" should have //goroutinectx:spawner directive \(calls sync\.WaitGroup\.Go with func argument\)`
	f := func() {
		var wg sync.WaitGroup
		wg.Go(func() { fmt.Println("work") })
		wg.Wait()
	}
	h := f
	if flag {
		h = f
	}
	h()
}

// [GOOD]: Non-spawning closure called through a Phi (#306)
//
// The closure the call runs spawns nothing.
func goodWaitgroupPlainClosurePhiCall(flag bool) {
	var wg sync.WaitGroup
	f := func() { fmt.Println("work") }
	h := f
	if flag {
		h = f
	}
	h()
	wg.Wait()
}
