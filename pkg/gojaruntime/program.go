// Package gojaruntime executes trusted project scripts with request-local state.
// It is not a security or memory sandbox for untrusted code.
package gojaruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
)

const (
	DefaultTimeout = 100 * time.Millisecond
	MaxTimeout     = 5 * time.Second
	MaxDataBytes   = 1 << 20
)

// Program shares code, never a VM, function closure, or JavaScript object.
type Program struct {
	code    *goja.Program
	timeout time.Duration
}

// Capture the JSON helpers before evaluating project code. Values cross the
// Go/JS boundary as JSON, never as wrappers around shared Go maps or objects.
var bridge = goja.MustCompile("hyperbricks:goja-data", `(function () {
  "use strict";
  const parse = JSON.parse;
  const stringify = JSON.stringify;
  const prototype = Object.prototype;
  const getPrototypeOf = Object.getPrototypeOf;
  const array = Array.isArray;
  const finite = Number.isFinite;
  return {
    parse: function (text) { return parse(text); },
    stringify: function (value) {
      if (value === null || typeof value !== "object" ||
          (getPrototypeOf(value) !== prototype && getPrototypeOf(value) !== null)) {
        throw new TypeError("main(input) must return a plain data object");
      }
      return stringify(value, function (key, item) {
        const type = typeof item;
        if (type === "undefined" || type === "function" || type === "symbol" || type === "bigint" ||
            (type === "number" && !finite(item))) {
          throw new TypeError("script result must contain only JSON-compatible data");
        }
        if (item !== null && type === "object" && !array(item) &&
            getPrototypeOf(item) !== prototype && getPrototypeOf(item) !== null) {
          throw new TypeError("script result must contain only plain data objects and arrays");
        }
        return item;
      });
    }
  };
})()`, true)

var entryPoint = goja.MustCompile("hyperbricks:goja-main", `typeof main === "function" ? main : undefined`, true)

// Compile validates syntax and the main entry point in a disposable VM. Source
// parsing is a load-time operation; top-level execution uses the same deadline
// as requests. Top-level code must not depend on request input.
func Compile(name, source string, timeout time.Duration) (*Program, error) {
	if strings.TrimSpace(source) == "" {
		return nil, fmt.Errorf("script is required and must not be empty")
	}
	if len(source) > MaxDataBytes {
		return nil, fmt.Errorf("script exceeds %d bytes", MaxDataBytes)
	}
	if timeout <= 0 || timeout > MaxTimeout {
		return nil, fmt.Errorf("timeout must be greater than zero and at most %s", MaxTimeout)
	}
	code, err := goja.Compile(name, source, true)
	if err != nil {
		return nil, fmt.Errorf("compile script: %w", err)
	}
	p := &Program{code: code, timeout: timeout}
	if _, err := p.execute(context.Background(), nil); err != nil {
		return nil, fmt.Errorf("initialize script: %w", err)
	}
	return p, nil
}

// Run accepts JSON and returns independently owned plain Go data.
func (p *Program) Run(ctx context.Context, inputJSON []byte) (map[string]interface{}, error) {
	if len(inputJSON) == 0 || len(inputJSON) > MaxDataBytes {
		return nil, fmt.Errorf("input must contain 1 to %d JSON bytes", MaxDataBytes)
	}
	return p.execute(ctx, inputJSON)
}

func (p *Program) execute(parent context.Context, inputJSON []byte) (map[string]interface{}, error) {
	if p == nil || p.code == nil {
		return nil, fmt.Errorf("script program is not prepared")
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, p.timeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	vm := goja.New()
	vm.SetMaxCallStackSize(1024)
	stop := context.AfterFunc(ctx, func() { vm.Interrupt(ctx.Err()) })
	defer stop()

	dataBridge, err := vm.RunProgram(bridge)
	if err != nil {
		return nil, fmt.Errorf("initialize data bridge: %w", err)
	}
	parse, _ := goja.AssertFunction(dataBridge.ToObject(vm).Get("parse"))
	stringify, _ := goja.AssertFunction(dataBridge.ToObject(vm).Get("stringify"))
	if _, err := vm.RunProgram(p.code); err != nil {
		return nil, fmt.Errorf("execute script: %w", err)
	}
	entry, err := vm.RunProgram(entryPoint)
	if err != nil {
		return nil, fmt.Errorf("resolve main: %w", err)
	}
	main, ok := goja.AssertFunction(entry)
	if !ok {
		return nil, fmt.Errorf("script must declare function main(input)")
	}
	if inputJSON == nil {
		return nil, ctx.Err()
	}
	input, err := parse(goja.Undefined(), vm.ToValue(string(inputJSON)))
	if err != nil {
		return nil, fmt.Errorf("decode script input: %w", err)
	}
	result, err := main(goja.Undefined(), input)
	if err != nil {
		return nil, fmt.Errorf("main: %w", err)
	}
	encoded, err := stringify(goja.Undefined(), result)
	if err != nil {
		return nil, fmt.Errorf("encode script result: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	output := encoded.String()
	if len(output) > MaxDataBytes {
		return nil, fmt.Errorf("script result exceeds %d bytes", MaxDataBytes)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, fmt.Errorf("decode script result: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("main(input) must return a plain data object")
	}
	return data, nil
}
