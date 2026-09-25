package main

import (
	"fmt"

	"golang.org/x/sync/errgroup"
)

func main() {
	goodSimple()
	badSimple()
	goodComplex()
	badComplex()
}

// ===== SIMPLE PATTERN =====

// goodSimple: properly labeled spawner function
//
//goroutinectx:spawner
func goodSimple() {
	g := new(errgroup.Group)
	g.Go(func() error {
		fmt.Println("work")
		return nil
	})
	_ = g.Wait()
}

// badSimple: missing spawner label
func badSimple() {
	g := new(errgroup.Group)
	g.Go(func() error {
		fmt.Println("work")
		return nil
	})
	_ = g.Wait()
}

// ===== COMPLEX PATTERN =====

// goodComplex: properly labeled with nested spawn
//
//goroutinectx:spawner
func goodComplex() {
	g := new(errgroup.Group)
	for i := range 3 {
		g.Go(func() error {
			fmt.Printf("work %d\n", i)
			return nil
		})
	}
	g.TryGo(func() error {
		fmt.Println("try work")
		return nil
	})
	_ = g.Wait()
}

// badComplex: missing label with multiple spawn methods
func badComplex() {
	g := new(errgroup.Group)
	for i := range 3 {
		g.Go(func() error {
			fmt.Printf("work %d\n", i)
			return nil
		})
	}
	g.TryGo(func() error {
		fmt.Println("try work")
		return nil
	})
	_ = g.Wait()
}
