package serial_test

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2/internal/serial"
)

func TestSerialControlClose(t *testing.T) {
	ctl := serial.NewControl()
	defer ctl.Close()
	s := newStatistics()
	for i := 0; i < 10; i++ {
		i := i
		go worker(ctl, i, s)
	}
	time.Sleep(100 * time.Millisecond) // wait for a worker to be spawned
	ctl.Cancel()                       // cancel the first worker
	ctl.Cancel()                       // very likely cancel nothing
	ctl.Close()                        // Close for ctl.Done()
	ctl.Close()                        // should do no harm if called multiple times
	select {
	case <-ctl.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("ctl.Done() timeout")
	}
	time.Sleep(100 * time.Millisecond) // wait for all workers to be canceled
	assertStatistics(t, s, 0, 10)
}

func TestSerialControlCancelOnNew(t *testing.T) {
	ctl := serial.NewControl(serial.WithCancelFormer())
	defer ctl.Close()
	s := newStatistics()
	for i := 0; i < 10; i++ {
		i := i
		go worker(ctl, i, s)
	}
	// in 300ms:
	// 1. all tasks are canceled by subsequent tasks, except the last one.
	// 2. the last worker is spawned and done.
	time.Sleep(300 * time.Millisecond)
	assertStatistics(t, s, 1, 9)
}

func TestSerialControlCloseNoCancelOnNew(t *testing.T) {
	ctl := serial.NewControl()
	s := newStatistics()
	for i := 0; i < 10; i++ {
		i := i
		go worker(ctl, i, s)
	}
	time.Sleep(100 * time.Millisecond) // sleep for a worker spawned
	ctl.Cancel()                       // cancel the first worker
	time.Sleep(100 * time.Millisecond) // sleep for a worker spawned
	ctl.Cancel()                       // cancel the second worker
	time.Sleep(500 * time.Millisecond) // wait for 2 workers spawned and done
	ctl.Close()                        // close the control, cancel all remaining workers
	time.Sleep(100 * time.Millisecond) // wait for all workers to be canceled
	assertStatistics(t, s, 2, 8)
}

func TestSerialControlOrdered(t *testing.T) {
	ctl := serial.NewControl(serial.WithOrdered(100))
	defer ctl.Close()
	s := newStatistics()
	wg := sync.WaitGroup{}
	want := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	for i := 0; i < 10; i++ {
		i := i
		wg.Add(1)
		go func() {
			worker(ctl, i, s)
			wg.Done()
		}()
		time.Sleep(10 * time.Millisecond) // wait for the task accepted by queue
	}
	wg.Wait() // wait for all workers to be canceled
	if !reflect.DeepEqual(s.timeline, want) {
		t.Fatalf("got %v, want %v", s.timeline, want)
	}
}

func TestSerialControlClosed(t *testing.T) {
	ctl := serial.NewControl()
	ctl.Close()
	ctx, cancel := ctl.New(context.Background())
	defer cancel()
	if ctx.Err() != context.Canceled {
		t.Fatalf("got %v, want %v", ctx.Err(), context.Canceled)
	}
}

func TestSerialControlQueueClosed(t *testing.T) {
	ctl := serial.NewControl(serial.WithOrdered(100))
	ctl.Close()
	ctx, cancel := ctl.New(context.Background())
	defer cancel()
	if ctx.Err() != context.Canceled {
		t.Fatalf("got %v, want %v", ctx.Err(), context.Canceled)
	}
}

// worker simulates a worker that takes 200ms to finish.
func worker(ctl *serial.Control, i int, statistics *statistics) {
	ctx, cancel := ctl.New(context.Background())
	defer cancel()
	if ctx.Err() != nil {
		statistics.addCanceled(i)
		fmt.Println("worker", i, "canceled before start")
		return
	}
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		statistics.addCanceled(i)
		fmt.Println("worker", i, "canceled")
		return
	case <-timer.C:
		statistics.addDone(i)
		fmt.Println("worker", i, "done")
	}
}

type statistics struct {
	sync.Mutex
	timeline []int
	done     map[int]struct{}
	canceled map[int]struct{}
}

func newStatistics() *statistics {
	return &statistics{
		timeline: make([]int, 0),
		done:     make(map[int]struct{}),
		canceled: make(map[int]struct{}),
	}
}

func (s *statistics) addDone(i int) {
	s.Lock()
	s.done[i] = struct{}{}
	s.timeline = append(s.timeline, i)
	s.Unlock()
}

func (s *statistics) addCanceled(i int) {
	s.Lock()
	s.canceled[i] = struct{}{}
	s.timeline = append(s.timeline, i)
	s.Unlock()
}

func assertStatistics(t *testing.T, s *statistics, done, canceled int) {
	s.Lock()
	defer s.Unlock()
	gotDone := len(s.done)
	gotCanceled := len(s.canceled)
	if gotDone != done || gotCanceled != canceled {
		t.Fatalf("(done, canceled) = (%d, %d), wanted (done, canceled) = (%d, %d)", gotDone, gotCanceled, done, canceled)
	}
}
