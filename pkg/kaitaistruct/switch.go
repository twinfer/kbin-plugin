package kaitaistruct

import (
	"context"
	"fmt"
	"strings"
)

// TODO: Add context.Context to ResolveType and pass it to evaluateExpression
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
		casesValue, casesOk := m["cases"]
		if !casesOk {
			return nil, fmt.Errorf("missing 'cases' in switch definition")
		}

		// Handle cases being map[string]any or map[string]string
		if cMapAny, ok := casesValue.(map[string]any); ok {
			cases = make(map[string]string)
			for k, v := range cMapAny {
				if typeStr, ok := v.(string); ok {
					cases[k] = typeStr
				} else {
					return nil, fmt.Errorf("case value must be a string, got %T", v)
				}
			}
		} else if cMapStr, ok := casesValue.(map[string]string); ok {
			cases = cMapStr // Directly assign if it's already map[string]string
		} else {
			return nil, fmt.Errorf("cases must be a map[string]any or map[string]string, got %T", casesValue)
		}

		// Check for default case within the now-standardized `cases` map
		if defaultCaseStr, ok := cases["_"]; ok {
			// No need to check type again, as `cases` is now map[string]string
			defaultType = defaultCaseStr
		} else {
			// If no explicit default case "_" is found, it means there's no default.
			// The ResolveType method will handle this if no other case matches.
			// We don't need to set defaultType to "" explicitly here, as it's already the zero value.
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
func (s *SwitchTypeSelector) ResolveType(goCtx context.Context, pCtx *ParseContext, interpreter *KaitaiInterpreter) (string, error) {
	// Evaluate switch-on expression
	result, err := interpreter.evaluateExpression(goCtx, s.switchOn, pCtx)
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
