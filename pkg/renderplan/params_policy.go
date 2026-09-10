package renderplan

import (
	"html/template"
	"reflect"
	"text/template/parse"
)

type paramsPolicy uint8

const (
	paramsUnused paramsPolicy = iota
	paramsReadOnly
	paramsPrivate
)

func templateParamsPolicy(parsedTemplate *template.Template) paramsPolicy {
	if parsedTemplate == nil {
		return paramsPrivate
	}
	policy := paramsUnused
	for _, associated := range parsedTemplate.Templates() {
		if associated.Tree == nil {
			continue
		}
		policy = stricterParamsPolicy(policy, nodeParamsPolicy(associated.Tree.Root))
	}
	return policy
}

func nodeParamsPolicy(node parse.Node) paramsPolicy {
	if node == nil {
		return paramsUnused
	}
	value := reflect.ValueOf(node)
	if value.Kind() == reflect.Ptr && value.IsNil() {
		return paramsUnused
	}

	switch typed := node.(type) {
	case *parse.ListNode:
		policy := paramsUnused
		for _, child := range typed.Nodes {
			policy = stricterParamsPolicy(policy, nodeParamsPolicy(child))
		}
		return policy
	case *parse.ActionNode:
		return nodeParamsPolicy(typed.Pipe)
	case *parse.PipeNode:
		policy := paramsUnused
		for _, command := range typed.Cmds {
			policy = stricterParamsPolicy(policy, nodeParamsPolicy(command))
		}
		return policy
	case *parse.CommandNode:
		policy := paramsUnused
		for _, argument := range typed.Args {
			policy = stricterParamsPolicy(policy, nodeParamsPolicy(argument))
		}
		if len(typed.Args) > 0 {
			if _, callsFunction := typed.Args[0].(*parse.IdentifierNode); callsFunction && policy == paramsReadOnly {
				return paramsPrivate
			}
		}
		return policy
	case *parse.FieldNode:
		if len(typed.Ident) == 0 || typed.Ident[0] != "Params" {
			return paramsUnused
		}
		if len(typed.Ident) == 1 {
			return paramsPrivate
		}
		return paramsReadOnly
	case *parse.ChainNode:
		return nodeParamsPolicy(typed.Node)
	case *parse.IfNode:
		return branchParamsPolicy(typed.Pipe, typed.List, typed.ElseList)
	case *parse.RangeNode:
		return branchParamsPolicy(typed.Pipe, typed.List, typed.ElseList)
	case *parse.WithNode:
		return branchParamsPolicy(typed.Pipe, typed.List, typed.ElseList)
	case *parse.TemplateNode:
		return paramsPrivate
	case *parse.DotNode, *parse.VariableNode:
		return paramsPrivate
	case *parse.TextNode, *parse.CommentNode, *parse.IdentifierNode,
		*parse.StringNode, *parse.NumberNode, *parse.BoolNode, *parse.NilNode,
		*parse.BreakNode, *parse.ContinueNode:
		return paramsUnused
	default:
		return paramsPrivate
	}
}

func branchParamsPolicy(pipe *parse.PipeNode, list, elseList *parse.ListNode) paramsPolicy {
	policy := nodeParamsPolicy(pipe)
	policy = stricterParamsPolicy(policy, nodeParamsPolicy(list))
	return stricterParamsPolicy(policy, nodeParamsPolicy(elseList))
}

func stricterParamsPolicy(left, right paramsPolicy) paramsPolicy {
	if right > left {
		return right
	}
	return left
}
