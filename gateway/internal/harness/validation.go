package harness

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ==================== Schema Validation 层 ====================
// 对标 aimagicx / LangChain 的结构化验证模式：
// LLM 输出必须经过 Schema 验证，验证失败自动注入错误信息触发重试。
// Harness 级别的 validation 比 prompt "请输出JSON" 可靠 28-47%。

// FieldRule 单个字段的验证规则
type FieldRule struct {
	Name     string      // 字段名
	Required bool        // 是否必填
	Type     string      // 期望类型: "string", "number", "bool", "array", "object"
	Min      *float64    // 最小值（number）或最小长度（string/array）
	Max      *float64    // 最大值（number）或最大长度（string/array）
	Enum     []string    // 枚举值（string 类型）
}

// SchemaValidator JSON Schema 验证器
type SchemaValidator struct {
	Fields []FieldRule
}

// ValidationError 结构化验证错误
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("field '%s': %s", e.Field, e.Message)
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

func (r ValidationResult) ErrorString() string {
	if r.Valid {
		return ""
	}
	msgs := make([]string, len(r.Errors))
	for i, e := range r.Errors {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "; ")
}

// Validate 验证 JSON 字符串是否符合 Schema
func (v *SchemaValidator) Validate(jsonStr string) ValidationResult {
	result := ValidationResult{Valid: true}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{{Field: "_root", Message: "invalid JSON: " + err.Error()}},
		}
	}

	for _, rule := range v.Fields {
		val, exists := data[rule.Name]

		if rule.Required && !exists {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field: rule.Name, Message: "required field missing",
			})
			continue
		}

		if !exists {
			continue
		}

		// 类型检查
		if err := checkType(rule, val); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
			continue
		}

		// 范围检查
		if errs := checkRange(rule, val); len(errs) > 0 {
			result.Valid = false
			result.Errors = append(result.Errors, errs...)
		}

		// 枚举检查
		if len(rule.Enum) > 0 {
			if strVal, ok := val.(string); ok {
				found := false
				for _, e := range rule.Enum {
					if strVal == e {
						found = true
						break
					}
				}
				if !found {
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Field:   rule.Name,
						Message: fmt.Sprintf("value '%s' not in enum %v", strVal, rule.Enum),
					})
				}
			}
		}
	}

	return result
}

// ValidateArray 验证 JSON 数组，每个元素都要符合 Schema
func (v *SchemaValidator) ValidateArray(jsonStr string) ValidationResult {
	result := ValidationResult{Valid: true}

	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &arr); err != nil {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{{Field: "_root", Message: "not a valid JSON array: " + err.Error()}},
		}
	}

	if len(arr) == 0 {
		return ValidationResult{
			Valid: false,
			Errors: []ValidationError{{Field: "_root", Message: "array is empty"}},
		}
	}

	for i, item := range arr {
		r := v.Validate(string(item))
		if !r.Valid {
			result.Valid = false
			for _, e := range r.Errors {
				result.Errors = append(result.Errors, ValidationError{
					Field:   fmt.Sprintf("[%d].%s", i, e.Field),
					Message: e.Message,
				})
			}
		}
	}

	return result
}

func checkType(rule FieldRule, val interface{}) *ValidationError {
	if rule.Type == "" {
		return nil
	}

	var actual string
	switch val.(type) {
	case string:
		actual = "string"
	case float64:
		actual = "number"
	case bool:
		actual = "bool"
	case []interface{}:
		actual = "array"
	case map[string]interface{}:
		actual = "object"
	default:
		actual = fmt.Sprintf("%T", val)
	}

	if actual != rule.Type {
		return &ValidationError{
			Field:   rule.Name,
			Message: fmt.Sprintf("expected type '%s', got '%s'", rule.Type, actual),
		}
	}
	return nil
}

func checkRange(rule FieldRule, val interface{}) []ValidationError {
	var errs []ValidationError

	switch v := val.(type) {
	case float64:
		if rule.Min != nil && v < *rule.Min {
			errs = append(errs, ValidationError{
				Field:   rule.Name,
				Message: fmt.Sprintf("value %v is less than minimum %v", v, *rule.Min),
			})
		}
		if rule.Max != nil && v > *rule.Max {
			errs = append(errs, ValidationError{
				Field:   rule.Name,
				Message: fmt.Sprintf("value %v exceeds maximum %v", v, *rule.Max),
			})
		}
	case string:
		l := float64(len(v))
		if rule.Min != nil && l < *rule.Min {
			errs = append(errs, ValidationError{
				Field:   rule.Name,
				Message: fmt.Sprintf("string length %d is less than minimum %v", len(v), *rule.Min),
			})
		}
		if rule.Max != nil && l > *rule.Max {
			errs = append(errs, ValidationError{
				Field:   rule.Name,
				Message: fmt.Sprintf("string length %d exceeds maximum %v", len(v), *rule.Max),
			})
		}
	case []interface{}:
		l := float64(len(v))
		if rule.Min != nil && l < *rule.Min {
			errs = append(errs, ValidationError{
				Field:   rule.Name,
				Message: fmt.Sprintf("array length %d is less than minimum %v", len(v), *rule.Min),
			})
		}
		if rule.Max != nil && l > *rule.Max {
			errs = append(errs, ValidationError{
				Field:   rule.Name,
				Message: fmt.Sprintf("array length %d exceeds maximum %v", len(v), *rule.Max),
			})
		}
	}

	return errs
}

// ==================== 预定义 Schema ====================

// PlannerSchema Planner 输出的任务列表 Schema
func PlannerSchema() *SchemaValidator {
	min1 := 1.0
	return &SchemaValidator{
		Fields: []FieldRule{
			{Name: "id", Required: true, Type: "string", Min: &min1},
			{Name: "title", Required: true, Type: "string", Min: &min1},
			{Name: "description", Required: true, Type: "string", Min: &min1},
		},
	}
}

// GeneratorSchema Generator 输出 Schema
func GeneratorSchema() *SchemaValidator {
	min1 := 1.0
	return &SchemaValidator{
		Fields: []FieldRule{
			{Name: "result", Required: true, Type: "string", Min: &min1},
			{Name: "updated_artifact_data", Required: false, Type: "object"},
		},
	}
}

// EvaluatorSchema Evaluator 输出 Schema
func EvaluatorSchema() *SchemaValidator {
	minScore := 0.0
	maxScore := 100.0
	return &SchemaValidator{
		Fields: []FieldRule{
			{Name: "score", Required: true, Type: "number", Min: &minScore, Max: &maxScore},
			{Name: "feedback", Required: true, Type: "string"},
			{Name: "passed", Required: true, Type: "bool"},
		},
	}
}
