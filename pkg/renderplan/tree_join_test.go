package renderplan

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestCompiledTreeAssembly(t *testing.T) {
	for _, test := range []struct {
		name, enclose string
		parts         []string
		want          string
	}{
		{name: "zero children"},
		{name: "empty children slice", parts: []string{}},
		{name: "one empty child", parts: []string{""}},
		{name: "one child", parts: []string{" <b>&raw</b>\n"}, want: " <b>&raw</b>\n"},
		{name: "many empty children", parts: []string{"", "", ""}},
		{name: "many children", parts: []string{"third", "first", "second"}, want: "thirdfirstsecond"},
		{name: "empty parts and exact bytes", parts: []string{"", "<b>&</b>", "", "\x00\xc3\xa9\n", ""}, want: "<b>&</b>\x00\xc3\xa9\n"},
		{name: "enclosed zero children", enclose: "section", want: "<section></section>"},
		{name: "enclosed empty child", enclose: "<p>", parts: []string{""}, want: "<p></p>"},
		{name: "enclosed one child", enclose: "p", parts: []string{"raw"}, want: "<p>raw</p>"},
		{name: "enclosed many children", enclose: "  <section> | </section>  ", parts: []string{" first ", "", "<b>last</b>"}, want: "<section> first <b>last</b></section>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := newRenderState(context.Background(), nil)
			calls := make([]int, 0, len(test.parts))
			wantCalls := make([]int, len(test.parts))
			node := &treeNode{enclose: test.enclose, children: make([]treeChild, len(test.parts))}
			for index, part := range test.parts {
				wantCalls[index] = index
				node.children[index] = treeChild{
					key: fmt.Sprintf("child-%d", len(test.parts)-index),
					node: forwardChildNodeFunc(func(gotState *renderState) (string, []error) {
						if gotState != state {
							t.Error("child received a different render state")
						}
						calls = append(calls, index)
						return part, nil
					}),
				}
			}
			output, renderErrors := node.Render(state)
			if output != test.want {
				t.Errorf("output = %q, want %q", output, test.want)
			}
			if renderErrors != nil {
				t.Errorf("errors = %v, want nil", renderErrors)
			}
			if !reflect.DeepEqual(calls, wantCalls) {
				t.Errorf("child calls = %v, want exactly once in order %v", calls, wantCalls)
			}
		})
	}
}

func TestCompiledTreeErrorsMatchCompositeSorter(t *testing.T) {
	keys := []string{"z", "a", "m"}
	parts := []string{"<b>partial</b>", "", "tail"}
	first := shared.ComponentError{Hash: "first", Key: "z", Path: "root.z", File: "tree.yaml", Type: "<TEXT>", Err: "first failed", Rejected: true}
	second := shared.ComponentError{Hash: "second", Key: "a", Err: "second failed", Rejected: true}
	last := shared.ComponentError{Hash: "last", Key: "m", Err: "last failed", Rejected: true}
	unknown := shared.ComponentError{Hash: "unknown", Key: "missing", Err: "unknown key"}
	plainFirst, plainLast := errors.New("plain first"), errors.New("plain last")
	pointer := &shared.ComponentError{Hash: "pointer", Key: "a", Err: "pointer component"}
	other := shared.CompositeError{Key: "z", Err: "composite error"}
	for _, test := range []struct {
		name     string
		warnings []string
		children [][]error
	}{
		{name: "one warning", warnings: []string{"warning one"}},
		{name: "many warnings", warnings: []string{"warning one", "warning two"}},
		{name: "warning with successful children", warnings: []string{"warning one"}, children: [][]error{nil, {}, nil}},
		{name: "one component error", children: [][]error{nil, {second}, nil}},
		{name: "one plain error", children: [][]error{nil, {plainFirst}, nil}},
		{name: "component key order", children: [][]error{{last, second}, {first}, nil}},
		{name: "plain errors", children: [][]error{{plainFirst}, {plainLast}, nil}},
		{name: "mixed errors", children: [][]error{{plainFirst, last, second}, {pointer, first, unknown}, {other, plainLast}}},
		{
			name:     "warnings and mixed errors with ties",
			warnings: []string{"warning one", "warning two"},
			children: [][]error{
				{plainFirst, last, second, unknown, pointer},
				{last, first, other, second, plainLast},
				{first, unknown, last, second, plainFirst},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			node := &treeNode{warnings: test.warnings, enclose: "  <section> | </section>  "}
			var wantErrors []error
			for _, warning := range test.warnings {
				wantErrors = append(wantErrors, shared.ComponentError{Err: warning})
			}
			calls := make([]int, 0, len(test.children))
			wantCalls := make([]int, len(test.children))
			for index, childErrors := range test.children {
				wantCalls[index] = index
				wantErrors = append(wantErrors, childErrors...)
				node.children = append(node.children, treeChild{
					key: keys[index],
					node: forwardChildNodeFunc(func(*renderState) (string, []error) {
						calls = append(calls, index)
						return parts[index], childErrors
					}),
				})
			}
			// Keep the old sorter's tie and non-component behavior as the oracle.
			wantErrors = composite.SortCompositeErrors(wantErrors, keys[:len(test.children)])
			output, renderErrors := node.Render(newRenderState(context.Background(), nil))
			wantOutput := "<section></section>"
			if len(test.children) != 0 {
				wantOutput = "<section><b>partial</b>tail</section>"
			}
			if output != wantOutput {
				t.Errorf("output = %q, want %q", output, wantOutput)
			}
			if !reflect.DeepEqual(calls, wantCalls) {
				t.Errorf("child calls = %v, want exactly once in order %v", calls, wantCalls)
			}
			if len(renderErrors) != len(wantErrors) {
				t.Fatalf("errors = %v, want %v", renderErrors, wantErrors)
			}
			for index, got := range renderErrors {
				// Only tree warnings have generated hashes and no key in these cases.
				if warning, ok := got.(shared.ComponentError); ok && warning.Key == "" {
					if warning.Hash == "" {
						t.Error("warning is missing its generated hash")
					}
					warning.Hash = ""
					got = warning
				}
				if got != wantErrors[index] {
					t.Errorf("error[%d] = %#v, want %#v", index, got, wantErrors[index])
				}
			}
		})
	}
}

func TestCompiledTreeRetainsOutputAcrossRequests(t *testing.T) {
	type requestKey struct{}
	requestError := errors.New("request failed")
	node := &treeNode{children: []treeChild{
		{key: "lead", node: forwardChildNodeFunc(func(state *renderState) (string, []error) {
			return state.ctx.Value(requestKey{}).(string), nil
		})},
		{key: "empty", node: forwardChildNodeFunc(func(*renderState) (string, []error) {
			return "", nil
		})},
		{key: "tail", node: forwardChildNodeFunc(func(state *renderState) (string, []error) {
			request := state.ctx.Value(requestKey{}).(string)
			output := "<b>" + request + "</b>"
			if request == "failed" {
				return output, []error{requestError}
			}
			return output, nil
		})},
	}}
	requests := []string{"first", strings.Repeat("large", 1024), "", "failed", "last", "first"}
	outputs := make([]string, len(requests))
	renderErrors := make([][]error, len(requests))
	for index, request := range requests {
		state := newRenderState(context.WithValue(context.Background(), requestKey{}, request), nil)
		outputs[index], renderErrors[index] = node.Render(state)
	}
	for index, request := range requests {
		if want := request + "<b>" + request + "</b>"; outputs[index] != want {
			t.Errorf("retained output[%d] = %q, want %q", index, outputs[index], want)
		}
		if request == "failed" {
			if len(renderErrors[index]) != 1 || renderErrors[index][0] != requestError {
				t.Errorf("retained errors[%d] = %v, want original request error", index, renderErrors[index])
			}
		} else if renderErrors[index] != nil {
			t.Errorf("request %d inherited errors: %v", index, renderErrors[index])
		}
	}
}

var benchmarkCompiledTreeOutput string

func BenchmarkCompiledTreeAssembly(b *testing.B) {
	for _, test := range []struct{ children, partBytes int }{
		{0, 0}, {1, 32}, {8, 64}, {32, 256}, {128, 256}, {8, 4096},
	} {
		b.Run(fmt.Sprintf("children_%d/bytes_%d", test.children, test.partBytes), func(b *testing.B) {
			part := strings.Repeat("x", test.partBytes)
			node := &treeNode{children: make([]treeChild, test.children)}
			for index := range node.children {
				node.children[index] = treeChild{
					key: fmt.Sprintf("child-%d", index),
					node: forwardChildNodeFunc(func(*renderState) (string, []error) {
						return part, nil
					}),
				}
			}
			state := newRenderState(context.Background(), nil)
			b.ReportAllocs()
			b.SetBytes(int64(test.children * test.partBytes))
			for b.Loop() {
				output, renderErrors := node.Render(state)
				if len(renderErrors) != 0 {
					b.Fatalf("render errors: %v", renderErrors)
				}
				benchmarkCompiledTreeOutput = output
			}
			if len(benchmarkCompiledTreeOutput) != test.children*test.partBytes {
				b.Fatal("incorrect assembled output size")
			}
		})
	}
}
