package llm

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// peakTracker counts calls in flight and remembers the most at once.
type peakTracker struct{ live, peak int32 }

func (p *peakTracker) in() {
	n := atomic.AddInt32(&p.live, 1)
	for {
		old := atomic.LoadInt32(&p.peak)
		if n <= old || atomic.CompareAndSwapInt32(&p.peak, old, n) {
			return
		}
	}
}
func (p *peakTracker) out() { atomic.AddInt32(&p.live, -1) }

func always(int) bool { return true }

// Reads (file reads, web_search, fetch_page) side by side overlap, the batch takes
// about one call's time, and results come back in the model's order even when later
// calls finish first.
func TestRunBatchParallelReadsKeepOrder(t *testing.T) {
	names := []string{"file_read", "web_search", "fetch_page", "file_write", "file_read", "file_read"}
	read := func(i int) bool { return names[i] != "file_write" }
	var p peakTracker
	var order []int
	t0 := time.Now()
	ok := RunBatch(context.Background(), len(names), read, always,
		func(_ context.Context, i int) string {
			p.in()
			defer p.out()
			time.Sleep(time.Duration(120-i*15) * time.Millisecond) // later calls finish first
			return names[i]
		},
		func(i int, res string) {
			if res != names[i] {
				t.Fatalf("call %d got %q", i, res)
			}
			order = append(order, i)
		})
	el := time.Since(t0)
	if !ok {
		t.Fatal("batch should finish")
	}
	for i, v := range order {
		if v != i {
			t.Fatalf("finish order %v, want 0..5", order)
		}
	}
	if p.peak < 3 {
		t.Fatalf("the three leading reads should overlap, peak was %d", p.peak)
	}
	// sequential would be 120+105+90+75+60+45 = 495ms; parallel ≈ 120+75+60 = 255ms
	if el > 400*time.Millisecond {
		t.Fatalf("took %v: the reads didn't run at the same time", el)
	}
}

// A long run of reads is capped at MaxParallel in flight, and all of them still run.
func TestRunBatchCapsConcurrency(t *testing.T) {
	const n = MaxParallel*2 + 3
	var p peakTracker
	var ran int32
	ok := RunBatch(context.Background(), n, always, always,
		func(_ context.Context, i int) string {
			p.in()
			defer p.out()
			time.Sleep(30 * time.Millisecond)
			atomic.AddInt32(&ran, 1)
			return ""
		}, func(int, string) {})
	if !ok || ran != n {
		t.Fatalf("ok=%v ran=%d, want all %d", ok, ran, n)
	}
	if p.peak > MaxParallel || p.peak < MaxParallel {
		t.Fatalf("peak %d, want exactly MaxParallel=%d", p.peak, MaxParallel)
	}
}

// Writes, commands and approvals run one at a time, in order, and split the reads:
// no read overlaps a write and the reads after a write wait for it.
func TestRunBatchWritesAndApprovalsStaySequential(t *testing.T) {
	names := []string{"file_read", "file_read", "run_shell", "approve", "file_write", "file_read", "file_read"}
	read := func(i int) bool { return names[i] == "file_read" }
	var mu sync.Mutex
	var live []string
	var log []string
	var p peakTracker
	ok := RunBatch(context.Background(), len(names), read, always,
		func(_ context.Context, i int) string {
			mu.Lock()
			if names[i] != "file_read" && len(live) > 0 {
				t.Errorf("%s (call %d) ran beside %v", names[i], i, live)
			}
			for _, l := range live {
				if l != "file_read" {
					t.Errorf("call %d ran beside %s", i, l)
				}
			}
			live = append(live, names[i])
			log = append(log, "start:"+names[i])
			mu.Unlock()
			if names[i] != "file_read" {
				p.in()
				defer p.out()
			}
			time.Sleep(20 * time.Millisecond)
			mu.Lock()
			for k, l := range live {
				if l == names[i] {
					live = append(live[:k], live[k+1:]...)
					break
				}
			}
			mu.Unlock()
			return names[i]
		}, func(int, string) {})
	if !ok {
		t.Fatal("batch should finish")
	}
	if p.peak != 1 {
		t.Fatalf("non-read calls overlapped (peak %d)", p.peak)
	}
	got := strings.Join(log, " ")
	want := "start:run_shell start:approve start:file_write"
	if !strings.Contains(got, want) {
		t.Fatalf("writes/approvals out of order: %s", got)
	}
}

// Stop (start returning false) ends the batch; calls already started finish.
func TestRunBatchStopHonoured(t *testing.T) {
	ran := 0
	ok := RunBatch(context.Background(), 4, func(int) bool { return false },
		func(i int) bool { return i < 2 },
		func(context.Context, int) string { ran++; return "" },
		func(int, string) {})
	if ok || ran != 2 {
		t.Fatalf("ok=%v ran=%d, want stop after 2 sequential calls", ok, ran)
	}
}

// One read that hangs is cut off at CallTimeout with an error, its context is
// cancelled, and the others in the batch keep their results.
func TestRunBatchPerCallTimeout(t *testing.T) {
	old := CallTimeout
	CallTimeout = 80 * time.Millisecond
	defer func() { CallTimeout = old }()
	var cancelled int32
	res := make([]string, 3)
	t0 := time.Now()
	ok := RunBatch(context.Background(), 3, always, always,
		func(ctx context.Context, i int) string {
			if i == 1 {
				select {
				case <-ctx.Done():
					atomic.StoreInt32(&cancelled, 1)
				case <-time.After(5 * time.Second):
				}
				return "late"
			}
			time.Sleep(10 * time.Millisecond)
			return "ok"
		},
		func(i int, r string) { res[i] = r })
	el := time.Since(t0)
	if !ok {
		t.Fatal("batch should finish")
	}
	if el > time.Second {
		t.Fatalf("batch waited %v for the hung call", el)
	}
	if res[0] != "ok" || res[2] != "ok" {
		t.Fatalf("the healthy calls lost their results: %q", res)
	}
	if !strings.HasPrefix(res[1], "Error:") || !strings.Contains(res[1], "longer than") {
		t.Fatalf("hung call result %q, want a timeout error", res[1])
	}
	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt32(&cancelled) != 1 {
		t.Fatal("the hung call's context was not cancelled")
	}
}

// Writes and commands are never cut off by the read deadline.
func TestRunBatchNoTimeoutOnWrites(t *testing.T) {
	old := CallTimeout
	CallTimeout = 20 * time.Millisecond
	defer func() { CallTimeout = old }()
	var got string
	RunBatch(context.Background(), 1, func(int) bool { return false }, always,
		func(ctx context.Context, _ int) string {
			time.Sleep(60 * time.Millisecond)
			if ctx.Err() != nil {
				return "cancelled"
			}
			return "done"
		},
		func(_ int, r string) { got = r })
	if got != "done" {
		t.Fatalf("write got %q, want it to run to the end", got)
	}
}

// A read that errors or panics only fails itself; a panicking write doesn't take the
// batch (or the app) down either.
func TestRunBatchFailureIsolation(t *testing.T) {
	names := []string{"file_read", "boom", "web_search", "error", "file_write_boom", "fetch_page"}
	read := func(i int) bool { return names[i] != "file_write_boom" }
	res := make([]string, len(names))
	ok := RunBatch(context.Background(), len(names), read, always,
		func(_ context.Context, i int) string {
			switch names[i] {
			case "boom", "file_write_boom":
				panic("kaboom " + names[i])
			case "error":
				return "Error: not found"
			}
			time.Sleep(10 * time.Millisecond)
			return "ok " + names[i]
		},
		func(i int, r string) { res[i] = r })
	if !ok {
		t.Fatal("batch should finish despite failures")
	}
	for _, i := range []int{0, 2, 5} {
		if res[i] != "ok "+names[i] {
			t.Fatalf("call %d (%s) got %q", i, names[i], res[i])
		}
	}
	if !strings.Contains(res[1], "crashed") || !strings.Contains(res[1], "kaboom boom") {
		t.Fatalf("panicking read got %q", res[1])
	}
	if res[3] != "Error: not found" {
		t.Fatalf("erroring read got %q", res[3])
	}
	if !strings.Contains(res[4], "crashed") {
		t.Fatalf("panicking write got %q", res[4])
	}
}

// Stop during a run of reads: in-flight reads see their context end and report it.
func TestRunBatchCancelStopsReads(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	res := make([]string, 2)
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	RunBatch(ctx, 2, always, always,
		func(c context.Context, _ int) string { <-c.Done(); time.Sleep(time.Second); return "late" },
		func(i int, r string) { res[i] = r })
	if res[0] != "Stopped." || res[1] != "Stopped." {
		t.Fatalf("got %q, want Stopped.", res)
	}
}
