package jsoncel

import (
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// Provider extends the CEL ref.TypeProvider interface and
// provides a JSON Schema-based type-system.
type Provider struct {
	// fallback proto-based type provider
	protos ref.TypeProvider

	schema *Schema

	typeName string

	// typeMap is a map of CEL type references to
	// the corresponding JSON schema node.
	// nested fields in the JSON schema are mapped using
	// a dot notation, e.g. 'group.id'.
	//
	// for example, if we have a JSON object like:
	// 	{
	//	  "group": {
	// 		"type": "object",
	//	    "properties": {
	//	      "id": {
	//	        "type": "string",
	//	      }
	//	  }
	//	}
	//
	// typeMap will be:
	// 	group -> {"group": {"type": "object", "properties": {"id": {"type": "string"}}}}
	// 	group.id -> {"type": "string"}
	typeMap map[string]*Schema
}

func NewProvider(typeName string, schema *Schema) *Provider {
	if schema == nil {
		schema = &Schema{}
	}

	p := &Provider{
		protos:   types.NewEmptyRegistry(),
		schema:   schema,
		typeName: typeName,
		typeMap:  map[string]*Schema{},
	}

	// build the typeMap so that we can look up CEL references
	// into the corresponding JSON schema nodes.
	p.mapSchema(typeName, schema)

	return p
}

// mapSchema builds up the typeMap for the JSON schema.
// Each entry in the type map is a particular node in the schema.
// The map keys use dot notation, for example:
// 'group.id' references
//
//	{ "group": {"id": "foo"}}
//				 ↑
//				this node
//
// The 'key' argument is the key to register the schema as (e.g. 'group.id')
func (p *Provider) mapSchema(key string, s *Schema) {
	p.typeMap[key] = s

	for childKey, child := range s.Properties {
		p.mapSchema(key+"."+childKey, child)
	}
}

var _ ref.TypeProvider = &Provider{}

// EnumValue returns the numeric value of the given enum value name.
func (p *Provider) EnumValue(enumName string) ref.Val {
	return p.protos.EnumValue(enumName)
}

// FindIdent takes a qualified identifier name and returns a Value if one
// exists.
func (p *Provider) FindIdent(identName string) (ref.Val, bool) {
	return p.protos.FindIdent(identName)
}

// FindType looks up the Type given a qualified typeName. Returns false
// if not found.
//
// Used during type-checking only.
func (p *Provider) FindType(typeName string) (*exprpb.Type, bool) {
	if f, ok := p.typeMap[typeName]; ok {
		switch f.Type {
		case Null:
			return decls.Null, true
		case Boolean:
			return decls.Bool, true
		case Object:
			// if the 'AdditionalProperties' field is set,
			// we can't enforce compile time type checking
			// for child keys in this type.
			//
			// e.g. {"tags": {"prod": true}}
			// we don't know whether tags.prod
			// will exist or not at compile time.
			if f.AdditionalProperties == TrueSchema {
				return decls.Any, true
			}
			return decls.NewObjectType(typeName), true
		case Array:
			return decls.NewListType(decls.String), true
		case Number:
			return decls.Double, true
		case String:
			return decls.String, true
		case Integer:
			return decls.Int, true
		}
	}

	return p.protos.FindType(typeName)
}

// FieldFieldType returns the field type for a checked type value. Returns
// false if the field could not be found.
//
// Used during type-checking only.
func (p *Provider) FindFieldType(messageType string, fieldName string) (*ref.FieldType, bool) {
	if f, ok := p.typeMap[fieldName]; ok {
		switch f.Type {
		case Null:
			return &ref.FieldType{Type: decls.Null}, true
		case Boolean:
			return &ref.FieldType{Type: decls.Bool}, true
		case Object:
			return &ref.FieldType{Type: decls.NewObjectType(messageType + "." + fieldName)}, true
		case Array:
			return &ref.FieldType{Type: decls.NewListType(decls.String)}, true
		case Number:
			return &ref.FieldType{Type: decls.Double}, true
		case String:
			return &ref.FieldType{Type: decls.String}, true
		case Integer:
			return &ref.FieldType{Type: decls.Int}, true
		}
	}

	// fall back to the default
	return p.protos.FindFieldType(messageType, fieldName)
}

// NewValue creates a new type value from a qualified name and map of field
// name to value.
//
// Note, for each value, the Val.ConvertToNative function will be invoked
// to convert the Val to the field's native type. If an error occurs during
// conversion, the NewValue will be a types.Err.
func (p *Provider) NewValue(typeName string, fields map[string]ref.Val) ref.Val {
	return p.protos.NewValue(typeName, fields)
}
