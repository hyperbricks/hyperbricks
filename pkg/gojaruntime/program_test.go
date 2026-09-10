package gojaruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProgramConcurrentIsolation(t *testing.T) {
	p, err := Compile("isolation.js", `
var count = 0;
function main(input) {
  count++;
  if (Object.prototype.requestID !== undefined || Array.prototype.requestID !== undefined) {
    throw new Error("prototype leaked");
  }
  Object.prototype.requestID = input.query.rid;
  Array.prototype.requestID = input.query.rid;
  input.values.nested.items.push(input.query.rid);
  return {rid: input.query.rid, count: count, items: input.values.nested.items};
}`, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]interface{}{"nested": map[string]interface{}{"items": []string{"original"}}}
	var wg sync.WaitGroup
	for worker := 0; worker < 32; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for request := 0; request < 10; request++ {
				rid := fmt.Sprintf("%d-%d", worker, request)
				input, err := json.Marshal(map[string]interface{}{"query": map[string]string{"rid": rid}, "values": values})
				if err != nil {
					t.Error(err)
					return
				}
				data, err := p.Run(context.Background(), input)
				if err != nil {
					t.Error(err)
					return
				}
				items := data["items"].([]interface{})
				if data["rid"] != rid || data["count"] != float64(1) || len(items) != 2 || items[0] != "original" || items[1] != rid {
					t.Errorf("request %s leaked state: %#v", rid, data)
				}
				items[0] = "caller mutation"
			}
		}(worker)
	}
	wg.Wait()
	if got := values["nested"].(map[string]interface{})["items"].([]string); len(got) != 1 || got[0] != "original" {
		t.Fatalf("input changed: %#v", values)
	}
}

func TestProgramErrorAndTimeoutDoNotPoisonNextRequest(t *testing.T) {
	p, err := Compile("failures.js", `var seen = 0;
function main(input) {
  seen++;
  Object.prototype.poison = input.mode;
  if (input.mode === "throw") throw new Error("expected failure");
  if (input.mode === "loop") while (true) {}
  return {seen: seen, mode: Object.prototype.poison};
}`, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"throw", "loop"} {
		_, err := p.Run(context.Background(), []byte(`{"mode":"`+mode+`"}`))
		if err == nil {
			t.Fatalf("%s did not fail", mode)
		}
		if mode == "loop" && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("loop error: %v", err)
		}
		data, err := p.Run(context.Background(), []byte(`{"mode":"ok"}`))
		if err != nil || data["seen"] != float64(1) || data["mode"] != "ok" {
			t.Fatalf("after %s: data=%v err=%v", mode, data, err)
		}
	}
}

func TestProgramRequestCancellation(t *testing.T) {
	p, err := Compile("cancel.js", `function main(input) { if (input.loop) while (true) {} return {}; }`, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Run(ctx, []byte(`{}`)); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancel: %v", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(10*time.Millisecond, cancel)
	defer timer.Stop()
	if _, err := p.Run(ctx, []byte(`{"loop":true}`)); !errors.Is(err, context.Canceled) {
		t.Fatalf("in-flight cancel: %v", err)
	}
	if _, err := p.Run(context.Background(), []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
}

func TestProgramHasNoHostCapabilities(t *testing.T) {
	p, err := Compile("capabilities.js", `function main() {
  return {require: typeof require, process: typeof process, fetch: typeof fetch,
    filesystem: typeof fs, http: typeof http, timer: typeof setTimeout};
}`, DefaultTimeout)
	if err != nil {
		t.Fatal(err)
	}
	data, err := p.Run(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range data {
		if value != "undefined" {
			t.Errorf("unexpected capability %s: %v", key, value)
		}
	}
}

func TestProgramRejectsInvalidSourceAndResults(t *testing.T) {
	for _, source := range []string{
		"", "function main(", "var nothing = 1;", "while (true) {}",
		`Object.defineProperty(globalThis, "main", {get: function() { throw new Error("getter"); }});`,
		`Object.defineProperty(globalThis, "main", {get: function() { while (true) {} }});`,
	} {
		if _, err := Compile("invalid.js", source, 20*time.Millisecond); err == nil {
			t.Errorf("accepted source %q", source)
		}
	}
	for _, result := range []string{"undefined", "null", "[]", "42", "function() {}", "Promise.resolve({})", "{bad: Promise.resolve({})}", "{bad: new Map()}", "{bad: function() {}}", "{bad: undefined}", "{bad: NaN}"} {
		p, err := Compile("result.js", "function main() { return "+result+"; }", DefaultTimeout)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.Run(context.Background(), []byte(`{}`)); err == nil {
			t.Errorf("accepted result %q", result)
		}
	}
	if _, err := Compile("timeout.js", "function main() { return {}; }", 0); err == nil {
		t.Fatal("accepted zero timeout")
	}
}

func TestProgramSerializationIsInsideDeadline(t *testing.T) {
	p, err := Compile("getter.js", `function main() { return { get hang() { while (true) {} } }; }`, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Run(context.Background(), []byte(`{}`)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("serialization deadline: %v", err)
	}
}

func TestProgramReportsSourceAndRejectsBadInput(t *testing.T) {
	p, err := Compile("resource-script.js", `function main() { throw new Error("example"); }`, DefaultTimeout)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Run(context.Background(), []byte(`{}`)); err == nil || !strings.Contains(err.Error(), "resource-script.js") {
		t.Fatalf("missing source location: %v", err)
	}
	if _, err := p.Run(context.Background(), []byte(`not JSON`)); err == nil {
		t.Fatal("accepted non-JSON input")
	}
}
