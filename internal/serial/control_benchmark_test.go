package serial_test

import (
	"context"
	"sync"
	"testing"

	"github.com/emersion/go-imap/v2/internal/serial"
)

func BenchmarkLock(b *testing.B) {
	lock := &sync.Mutex{}
	wg := sync.WaitGroup{}
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			lock.Lock()
			// no work load
			lock.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkSerialControl(b *testing.B) {
	benchmarkSerialControlWithOptions(b)
}

func BenchmarkSerialControlCancelFormer(b *testing.B) {
	benchmarkSerialControlWithOptions(b, serial.WithCancelFormer())
}

func BenchmarkSerialControlCancelLater(b *testing.B) {
	benchmarkSerialControlWithOptions(b, serial.WithCancelLater())
}

func BenchmarkSerialControlOrdered(b *testing.B) {
	benchmarkSerialControlWithOptions(b, serial.WithOrdered(100))
}

func benchmarkSerialControlWithOptions(b *testing.B, options ...serial.ControlOption) {
	ctl := serial.NewControl(options...)
	defer ctl.Close()
	wg := sync.WaitGroup{}
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			_, cancel := ctl.New(ctx)
			// no work load
			cancel()
			wg.Done()
		}()
	}
	wg.Wait()
}
