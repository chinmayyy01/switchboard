package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/coder/websocket"
)

func frame(v int16) []byte {
	b := make([]byte, 320*2)
	for i := 0; i < 320; i++ {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	return b
}

func oneCall(ctx context.Context, url string) (int64, error) {
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return 0, err
	}
	defer c.CloseNow()
	c.SetReadLimit(1 << 20)

	if err := c.Write(ctx, websocket.MessageText, []byte(`{"type":"call.start","lang":"en"}`)); err != nil {
		return 0, err
	}
	loud, silent := frame(6000), frame(0)
	for i := 0; i < 25; i++ {
		c.Write(ctx, websocket.MessageBinary, loud)
	}
	for i := 0; i < 60; i++ {
		c.Write(ctx, websocket.MessageBinary, silent)
	}

	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			return 0, err
		}
		if typ != websocket.MessageText {
			continue
		}
		var m struct {
			Type   string `json:"type"`
			TurnMs int64  `json:"turn_ms"`
		}
		if json.Unmarshal(data, &m) == nil && m.Type == "latency" {
			return m.TurnMs, nil
		}
	}
}

func pct(xs []int64, p float64) int64 {
	if len(xs) == 0 {
		return 0
	}
	i := int(float64(len(xs)) * p / 100)
	if i >= len(xs) {
		i = len(xs) - 1
	}
	return xs[i]
}

func main() {
	n := flag.Int("n", 25, "concurrent callers")
	url := flag.String("url", "ws://localhost:8090/ws", "caller ws url")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	lats := make([]int64, *n)
	errs := make([]error, *n)
	start := time.Now()
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			lats[i], errs[i] = oneCall(ctx, *url)
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	ok := make([]int64, 0, *n)
	fails := 0
	for i := 0; i < *n; i++ {
		if errs[i] != nil {
			fails++
			fmt.Fprintf(os.Stderr, "caller %d: %v\n", i, errs[i])
			continue
		}
		ok = append(ok, lats[i])
	}
	sort.Slice(ok, func(i, j int) bool { return ok[i] < ok[j] })

	var sum int64
	for _, v := range ok {
		sum += v
	}
	var avg int64
	if len(ok) > 0 {
		avg = sum / int64(len(ok))
	}

	fmt.Printf("callers=%d ok=%d failed=%d wall=%s\n", *n, len(ok), fails, elapsed.Round(time.Millisecond))
	fmt.Printf("turn_ms  avg=%d p50=%d p95=%d max=%d\n", avg, pct(ok, 50), pct(ok, 95), pct(ok, 100))
}
