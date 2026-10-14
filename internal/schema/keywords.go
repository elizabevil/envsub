package schema

// JSON Schema vocabulary used by the adapter. Only these keywords are
// understood; everything else is subject to the unknown keyword policy.
const (
	keywordType       = "type"
	keywordProperties = "properties"
	keywordItems      = "items"
	keywordDefault    = "default"
)

// extensionPrefix marks vendor extension keywords, which are always allowed.
const extensionPrefix = "x-"

// pathSeparator joins schema keyword paths and instance paths in diagnostics.
const pathSeparator = "."

// pointerSeparator starts a JSON Pointer.
const pointerSeparator = "/"

// indexPathFormat builds the instance path of an array element.
const indexPathFormat = "%s[%d]"

// JSON boolean spellings a declared boolean accepts when it is converted from
// its string form.
const (
	boolTrueText  = "true"
	boolFalseText = "false"
)

// schemaResourceName is the virtual resource name the schema is compiled under.
const schemaResourceName = "schema.json"

// JSON Schema type names, as they appear in a schema document. They are
// exported so the renderer can compare a declared type without repeating the
// literals.
const (
	// TypeNull and the names that follow are the JSON Schema type vocabulary the
	// resolver reports; they mirror types.SchemaType*.
	TypeNull    = "null"
	TypeBoolean = "boolean"
	TypeInteger = "integer"
	TypeNumber  = "number"
	TypeString  = "string"
	TypeArray   = "array"
	TypeObject  = "object"
)
