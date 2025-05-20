package kaitaistruct

import (
	"fmt"
	"strings"
)

// SwitchTypeSelector handles type selection in switch cases
type SwitchTypeSelector struct {
	schema      *KaitaiSchema
	switchOn    string
	cases       map[string]string
	defaultType string
}

// NewSwitchTypeSelector creates a switch type selector from a switch type definition
func NewSwitchTypeSelector(switchType any, schema *KaitaiSchema) (*SwitchTypeSelector, error) {
	// Handle different ways switch can be represented
	var switchOn string
	var cases map[string]string
	var defaultType string

	// Handle map representation
	if m, ok := switchType.(map[string]any); ok {
		// Get switch-on expression
		if s, ok := m["switch-on"].(string); ok {
			switchOn = s
		} else {
			return nil, fmt.Errorf("switch-on must be a string, got %T", m["switch-on"])
		}

		// Get cases
		if c, ok := m["cases"].(map[string]any); ok {
			cases = make(map[string]string)
			for k, v := range c {
				if typeStr, ok := v.(string); ok {
					cases[k] = typeStr
				} else {
					return nil, fmt.Errorf("case value must be a string, got %T", v)
				}
			}

			// Check for default case
			if defaultCase, ok := c["_"]; ok {
				if defaultStr, ok := defaultCase.(string); ok {
					defaultType = defaultStr
				}
			}
		} else {
			return nil, fmt.Errorf("cases must be a map, got %T", m["cases"])
		}
	} else {
		return nil, fmt.Errorf("switch type must be a map, got %T", switchType)
	}

	return &SwitchTypeSelector{
		schema:      schema,
		switchOn:    switchOn,
		cases:       cases,
		defaultType: defaultType,
	}, nil
}

// ResolveType resolves the actual type based on the switch value
func (s *SwitchTypeSelector) ResolveType(ctx *ParseContext, interpreter *KaitaiInterpreter) (string, error) {
	// Evaluate switch-on expression
	result, err := interpreter.evaluateExpression(s.switchOn, ctx)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate switch expression: %w", err)
	}

	// Convert result to string for case matching
	var switchValue string
	switch v := result.(type) {
	case string:
		switchValue = v
	case int, int64, uint, uint64, float64:
		switchValue = fmt.Sprintf("%v", v)
	case bool:
		if v {
			switchValue = "true"
		} else {
			switchValue = "false"
		}
	default:
		switchValue = fmt.Sprintf("%v", v)
	}

	// Look for exact match
	if typeName, ok := s.cases[switchValue]; ok {
		return typeName, nil
	}

	// Look for enum references (format: enum_name::enum_value)
	for caseKey, typeName := range s.cases {
		if strings.Contains(caseKey, "::") {
			parts := strings.Split(caseKey, "::")
			if len(parts) == 2 {
				enumName := parts[0]
				enumValue := parts[1]

				// Check if switch value is an enum value
				enumIntValue, isEnum := ResolveEnumValue(s.schema, enumName, switchValue)
				if isEnum {
					// Check if this case matches the enum value
					caseEnumIntValue, isCaseEnum := ResolveEnumValue(s.schema, enumName, enumValue)
					if isCaseEnum && enumIntValue == caseEnumIntValue {
						return typeName, nil
					}
				}
			}
		}
	}

	// Use default type if available
	if s.defaultType != "" {
		return s.defaultType, nil
	}

	return "", fmt.Errorf("no matching case for switch value '%s'", switchValue)
}

// ResolveEnumValue resolves an enum name to its integer value
func ResolveEnumValue(schema *KaitaiSchema, enumName string, enumValueName string) (int, bool) {
	// Look up enum in schema
	if schema.Enums == nil {
		return 0, false
	}

	enumDef, ok := schema.Enums[enumName]
	if !ok {
		return 0, false
	}

	// Look up enum value
	for value, name := range enumDef {
		if name == enumValueName {
			// Convert value to int
			if intValue, ok := value.(int); ok {
				return intValue, true
			}
			if floatValue, ok := value.(float64); ok {
				return int(floatValue), true
			}
		}
	}

	return 0, false
}

// ResolveEnumName resolves an enum integer value to its name
func ResolveEnumName(schema *KaitaiSchema, enumName string, enumValue int) (string, bool) {
	// Look up enum in schema
	if schema.Enums == nil {
		return "", false
	}

	enumDef, ok := schema.Enums[enumName]
	if !ok {
		return "", false
	}

	// Look up enum value
	for value, name := range enumDef {
		// Convert value to int
		var intValue int
		if v, ok := value.(int); ok {
			intValue = v
		} else if v, ok := value.(float64); ok {
			intValue = int(v)
		} else {
			continue
		}

		if intValue == enumValue {
			return name, true
		}
	}

	return "", false
}
