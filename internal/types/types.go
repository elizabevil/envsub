package types

// Kind is the runtime value kind used by the extended renderer.
type Kind string

// Kind values the extended renderer understands.
const (
	Null   Kind = "null"
	String Kind = "string"
	Bool   Kind = "bool"
	Int    Kind = "int"
	Float  Kind = "float"
	Array  Kind = "array"
	Object Kind = "object"
)
