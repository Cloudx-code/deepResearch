package entity

// FieldSchema 描述单个字段的约束（可按需再加枚举、pattern 等）
type FieldSchema struct {
	Name        string
	Type        string
	Description string
	MaxLength   int
	Required    bool
}
