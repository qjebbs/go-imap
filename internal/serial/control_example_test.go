package serial_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2/internal/serial"
)

func ExampleControl() {
	start := time.Now()
	taskTime := 100 * time.Millisecond
	wg := sync.WaitGroup{}
	// with cancel former, only the last spawned task will be done,
	// all other tasks will be canceled.
	// The total time is the time of the last task.
	ctl := serial.NewControl(serial.WithCancelFormer())
	defer ctl.Close()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := ctl.New(context.Background())
			defer cancel()
			timer := time.NewTimer(taskTime)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				log.Printf("#%d canceled\n", i)
				return
			case <-timer.C:
				log.Printf("#%d something done\n", i)
			}
		}(i)
	}
	wg.Wait()
	dur := time.Since(start).Round(taskTime)
	fmt.Println(dur)
	// Output:
	// 100ms
}
