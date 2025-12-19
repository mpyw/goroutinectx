package filefilterskip

import (
	"context"
	"fmt"
)

// badGoroutineInTest is NOT reported when -test=false.
func badGoroutineInTest(ctx context.Context) {
	go func() {
		fmt.Println("no context in test file - but skipped")
	}()
}
