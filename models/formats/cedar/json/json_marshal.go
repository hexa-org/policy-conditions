// Package json Note: This code is from github.com/cedar-policy/cedar-go and is used under the APL 2.0 license terms.
package json

import (
	"encoding/json"
	"fmt"

	"github.com/cedar-policy/cedar-go/types"
	"github.com/cedar-policy/cedar-go/x/exp/ast"
)

func (s *ScopeJSON) FromNode(src ast.IsScopeNode) {
	switch t := src.(type) {
	case ast.ScopeTypeAll:
		s.Op = "All"
		return
	case ast.ScopeTypeEq:
		s.Op = "=="
		e := types.ImplicitlyMarshaledEntityUID(t.Entity)
		s.Entity = &e
		return
	case ast.ScopeTypeIn:
		s.Op = "in"
		e := types.ImplicitlyMarshaledEntityUID(t.Entity)
		s.Entity = &e
		return
	case ast.ScopeTypeInSet:
		s.Op = "in"
		es := make([]types.ImplicitlyMarshaledEntityUID, len(t.Entities))
		for i, e := range t.Entities {
			es[i] = types.ImplicitlyMarshaledEntityUID(e)
		}
		s.Entities = es
		return
	case ast.ScopeTypeIs:
		s.Op = "is"
		s.EntityType = string(t.Type)
		return
	case ast.ScopeTypeIsIn:
		s.Op = "is"
		s.EntityType = string(t.Type)
		s.In = &scopeInJSON{
			Entity: types.ImplicitlyMarshaledEntityUID(t.Entity),
		}
		return
	default:
		panic(fmt.Sprintf("unknown scope type %T", t))
	}
}

func unaryToJSON(dest **unaryJSON, src ast.UnaryNode) {
	res := &unaryJSON{}
	res.Arg.FromNode(src.Arg)
	*dest = res
}

func binaryToJSON(dest **BinaryJSON, src ast.BinaryNode) {
	res := &BinaryJSON{}
	res.Left.FromNode(src.Left)
	res.Right.FromNode(src.Right)
	*dest = res
}

func arrayToJSON(dest *arrayJSON, args []ast.IsNode) {
	res := arrayJSON{}
	for _, n := range args {
		var nn NodeJSON
		nn.FromNode(n)
		res = append(res, nn)
	}
	*dest = res
}

func extToJSON(dest *extensionJSON, name string, src types.Value) {
	res := arrayJSON{}
	str := src.String()
	val := ValueJSON{V: types.String(str)}
	res = append(res, NodeJSON{
		Value: &val,
	})
	*dest = extensionJSON{
		name: res,
	}
}

func extCallToJSON(dest extensionJSON, src ast.NodeTypeExtensionCall) {
	jsonArgs := arrayJSON{}
	arrayToJSON(&jsonArgs, src.Args)
	dest[string(src.Name)] = jsonArgs
}

func strToJSON(dest **strJSON, src ast.StrOpNode) {
	res := &strJSON{}
	res.Left.FromNode(src.Arg)
	res.Attr = string(src.Value)
	*dest = res
}

func likeToJSON(dest **likeJSON, src ast.NodeTypeLike) {
	res := &likeJSON{}
	res.Left.FromNode(src.Arg)
	res.Pattern = src.Value
	*dest = res
}

func recordToJSON(dest *recordJSON, src ast.NodeTypeRecord) {
	res := recordJSON{}
	for _, kv := range src.Elements {
		var nn NodeJSON
		nn.FromNode(kv.Value)
		res[string(kv.Key)] = nn
	}
	*dest = res
}

func ifToJSON(dest **ifThenElseJSON, src ast.NodeTypeIfThenElse) {
	res := &ifThenElseJSON{}
	res.If.FromNode(src.If)
	res.Then.FromNode(src.Then)
	res.Else.FromNode(src.Else)
	*dest = res
}

func isToJSON(dest **isJSON, src ast.NodeTypeIs) {
	res := &isJSON{}
	res.Left.FromNode(src.Left)
	res.EntityType = string(src.EntityType)
	*dest = res
}

func isInToJSON(dest **isJSON, src ast.NodeTypeIsIn) {
	res := &isJSON{}
	res.Left.FromNode(src.Left)
	res.EntityType = string(src.EntityType)
	res.In = &NodeJSON{}
	res.In.FromNode(src.Entity)
	*dest = res
}

func (j *NodeJSON) FromNode(src ast.IsNode) {
	switch t := src.(type) {
	// Value
	// Value *json.RawMessage `json:"Value"` // could be any
	case ast.NodeValue:
		// Any other function: decimal, ip
		// Decimal arrayJSON `json:"decimal"`
		// IP      arrayJSON `json:"ip"`
		switch tt := t.Value.(type) {
		case types.Decimal:
			extToJSON(&j.ExtensionCall, "decimal", tt)
			return
		case types.IPAddr:
			extToJSON(&j.ExtensionCall, "ip", tt)
			return
		}
		val := ValueJSON{V: t.Value}
		j.Value = &val
		return

	// Var
	// Var *string `json:"Var"`
	case ast.NodeTypeVariable:
		val := string(t.Name)
		j.Var = &val
		return

	// ! or neg operators
	// Not    *unaryJSON `json:"!"`
	// Negate *unaryJSON `json:"neg"`
	case ast.NodeTypeNot:
		unaryToJSON(&j.Not, t.UnaryNode)
		return
	case ast.NodeTypeNegate:
		unaryToJSON(&j.Negate, t.UnaryNode)
		return

	// Binary operators: ==, !=, in, <, <=, >, >=, &&, ||, +, -, *, contains, containsAll, containsAny, hasTag, getTag
	case ast.NodeTypeAdd:
		binaryToJSON(&j.Add, t.BinaryNode)
		return
	case ast.NodeTypeAnd:
		binaryToJSON(&j.And, t.BinaryNode)
		return
	case ast.NodeTypeContains:
		binaryToJSON(&j.Contains, t.BinaryNode)
		return
	case ast.NodeTypeContainsAll:
		binaryToJSON(&j.ContainsAll, t.BinaryNode)
		return
	case ast.NodeTypeContainsAny:
		binaryToJSON(&j.ContainsAny, t.BinaryNode)
		return
	case ast.NodeTypeEquals:
		binaryToJSON(&j.Equals, t.BinaryNode)
		return
	case ast.NodeTypeGreaterThan:
		binaryToJSON(&j.GreaterThan, t.BinaryNode)
		return
	case ast.NodeTypeGreaterThanOrEqual:
		binaryToJSON(&j.GreaterThanOrEqual, t.BinaryNode)
		return
	case ast.NodeTypeIn:
		binaryToJSON(&j.In, t.BinaryNode)
		return
	case ast.NodeTypeLessThan:
		binaryToJSON(&j.LessThan, t.BinaryNode)
		return
	case ast.NodeTypeLessThanOrEqual:
		binaryToJSON(&j.LessThanOrEqual, t.BinaryNode)
		return
	case ast.NodeTypeMult:
		binaryToJSON(&j.Multiply, t.BinaryNode)
		return
	case ast.NodeTypeNotEquals:
		binaryToJSON(&j.NotEquals, t.BinaryNode)
		return
	case ast.NodeTypeOr:
		binaryToJSON(&j.Or, t.BinaryNode)
		return
	case ast.NodeTypeSub:
		binaryToJSON(&j.Subtract, t.BinaryNode)
		return
	case ast.NodeTypeGetTag:
		binaryToJSON(&j.GetTag, t.BinaryNode)
		return
	case ast.NodeTypeHasTag:
		binaryToJSON(&j.HasTag, t.BinaryNode)
		return

	// ., has
	// Access *strJSON `json:"."`
	// Has    *strJSON `json:"has"`
	case ast.NodeTypeAccess:
		strToJSON(&j.Access, t.StrOpNode)
		return
	case ast.NodeTypeHas:
		strToJSON(&j.Has, t.StrOpNode)
		return
	// is
	case ast.NodeTypeIs:
		isToJSON(&j.Is, t)
		return
	case ast.NodeTypeIsIn:
		isInToJSON(&j.Is, t)
		return

	// like
	// Like *strJSON `json:"like"`
	case ast.NodeTypeLike:
		likeToJSON(&j.Like, t)
		return

	// if-then-else
	// IfThenElse *ifThenElseJSON `json:"if-then-else"`
	case ast.NodeTypeIfThenElse:
		ifToJSON(&j.IfThenElse, t)
		return

	// Set
	// Set arrayJSON `json:"Set"`
	case ast.NodeTypeSet:
		arrayToJSON(&j.Set, t.Elements)
		return

	// Record
	// Record recordJSON `json:"Record"`
	case ast.NodeTypeRecord:
		recordToJSON(&j.Record, t)
		return

	// Any other method: ip, decimal, lessThan, lessThanOrEqual, greaterThan, greaterThanOrEqual, isIpv4, isIpv6, isLoopback, isMulticast, isInRange
	// ExtensionMethod map[string]arrayJSON `json:"-"`
	case ast.NodeTypeExtensionCall:
		j.ExtensionCall = extensionJSON{}
		extCallToJSON(j.ExtensionCall, t)
		return
	default:
		panic(fmt.Sprintf("unknown node type %T", t))

	}

}

func (j *NodeJSON) MarshalJSON() ([]byte, error) {
	if len(j.ExtensionCall) > 0 {
		return json.Marshal(j.ExtensionCall)
	}

	type nodeJSONAlias NodeJSON
	return json.Marshal((*nodeJSONAlias)(j))
}

type Policy ast.Policy

func wrapPolicy(p *ast.Policy) *Policy {
	return (*Policy)(p)
}

func (p *Policy) unwrap() *ast.Policy {
	return (*ast.Policy)(p)
}

func (p *Policy) MarshalJSON() ([]byte, error) {
	var j PolicyJSON
	j.Effect = "forbid"
	if p.Effect {
		j.Effect = "permit"
	}
	if len(p.Annotations) > 0 {
		j.Annotations = map[string]string{}
	}
	for _, a := range p.Annotations {
		j.Annotations[string(a.Key)] = string(a.Value)
	}
	j.Principal.FromNode(p.Principal)
	j.Action.FromNode(p.Action)
	j.Resource.FromNode(p.Resource)
	for _, c := range p.Conditions {
		var cond ConditionJSON
		cond.Kind = "when"
		if c.Condition == ast.ConditionUnless {
			cond.Kind = "unless"
		}
		cond.Body.FromNode(c.Body)
		j.Conditions = append(j.Conditions, cond)
	}
	return json.Marshal(j)
}
