// You can edit this code!
// Click here and start typing.
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func main(
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				fmt.Println("context done")
				return
			default:
				fmt.Println("working...")
				cancel()
				time.Sleep(time.Second * 1)
			}
		}
	}()

	wg.Wait()
	cancel()
	fmt.Println("Hello, 世界")
}
