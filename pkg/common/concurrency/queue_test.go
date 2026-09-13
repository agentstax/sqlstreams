package concurrency

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

// behavior: WaitForRoom on a full queue returns at the debounce timeout with
// the room that exists, and returns early when a dequeue frees a full batch.
func TestWaitForRoomReturnsAtTheDebounceOrOnADequeue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// setup
		queue, err := NewPressureQueue[int](2)
		if err != nil {
			t.Fatalf("NewPressureQueue(2) err = %v, want nil", err)
		}
		for _, value := range []int{1, 2} {
			if err := queue.EnQueue(context.Background(), &value); err != nil {
				t.Fatalf("EnQueue(%d) err = %v, want nil", value, err)
			}
		}

		// test
		started := time.Now()
		timedOutRoom, timedOutErr := queue.WaitForRoom(context.Background(), time.Second, 2)
		waited := time.Since(started)
		go func() {
			time.Sleep(100 * time.Millisecond)
			_, _ = queue.DeQueue(context.Background())
			_, _ = queue.DeQueue(context.Background())
		}()
		freedRoom, freedErr := queue.WaitForRoom(context.Background(), time.Minute, 2)
		synctest.Wait()

		// verify
		if timedOutErr != nil || timedOutRoom != 0 {
			t.Errorf("WaitForRoom(full, 1s) = %d, %v, want 0, nil", timedOutRoom, timedOutErr)
		}
		if waited != time.Second {
			t.Errorf("WaitForRoom(full, 1s) waited %v, want exactly the debounce", waited)
		}
		if freedErr != nil || freedRoom != 2 {
			t.Errorf("WaitForRoom after two dequeues = %d, %v, want 2, nil", freedRoom, freedErr)
		}
	})
}
