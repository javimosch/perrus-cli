package main

import (
	"fmt"
	"strconv"
	"strings"
)

// evalCondition parses and evaluates a Gatus-style condition string against a result.
// Supported placeholders: [STATUS], [RESPONSE_TIME], [CONNECTED], [BODY]
// Supported operators: ==, !=, <, <=, >, >=, contains
func evalCondition(cond string, res *Result) (bool, error) {
	cond = strings.TrimSpace(cond)
	if !strings.HasPrefix(cond, "[") {
		return false, fmt.Errorf("condition must begin with [PLACEHOLDER]")
	}
	end := strings.Index(cond, "]")
	if end < 0 {
		return false, fmt.Errorf("unclosed '[' in condition")
	}
	placeholder := cond[1:end]
	rest := strings.TrimSpace(cond[end+1:])

	op, valStr, err := parseOpValue(rest)
	if err != nil {
		return false, fmt.Errorf("condition %q: %w", cond, err)
	}

	switch placeholder {
	case "STATUS":
		want, err := strconv.Atoi(valStr)
		if err != nil {
			return false, fmt.Errorf("[STATUS] value must be an integer, got %q", valStr)
		}
		return cmpInt(res.StatusCode, op, want)

	case "RESPONSE_TIME":
		want, err := strconv.ParseInt(valStr, 10, 64)
		if err != nil {
			return false, fmt.Errorf("[RESPONSE_TIME] value must be an integer, got %q", valStr)
		}
		return cmpInt64(res.Duration, op, want)

	case "CONNECTED":
		switch op {
		case "==":
			return res.Connected == (valStr == "true"), nil
		case "!=":
			return res.Connected != (valStr == "true"), nil
		}
		return false, fmt.Errorf("[CONNECTED] only supports == and !=")

	case "BODY":
		body := res.Body
		switch op {
		case "==":
			return body == valStr, nil
		case "!=":
			return body != valStr, nil
		case "contains":
			return strings.Contains(body, valStr), nil
		}
		return false, fmt.Errorf("[BODY] supports ==, !=, contains — got %q", op)
	}

	return false, fmt.Errorf("unknown placeholder [%s]", placeholder)
}

func parseOpValue(s string) (op, val string, err error) {
	for _, candidate := range []string{"contains", "==", "!=", "<=", ">=", "<", ">"} {
		if strings.HasPrefix(s, candidate) {
			remainder := strings.TrimSpace(s[len(candidate):])
			// Strip surrounding quotes from string values
			if len(remainder) >= 2 && remainder[0] == '"' && remainder[len(remainder)-1] == '"' {
				remainder = remainder[1 : len(remainder)-1]
			}
			return candidate, remainder, nil
		}
	}
	return "", "", fmt.Errorf("no recognized operator in %q", s)
}

func cmpInt(a int, op string, b int) (bool, error) {
	switch op {
	case "==":
		return a == b, nil
	case "!=":
		return a != b, nil
	case "<":
		return a < b, nil
	case "<=":
		return a <= b, nil
	case ">":
		return a > b, nil
	case ">=":
		return a >= b, nil
	}
	return false, fmt.Errorf("unknown operator %q", op)
}

func cmpInt64(a int64, op string, b int64) (bool, error) {
	switch op {
	case "==":
		return a == b, nil
	case "!=":
		return a != b, nil
	case "<":
		return a < b, nil
	case "<=":
		return a <= b, nil
	case ">":
		return a > b, nil
	case ">=":
		return a >= b, nil
	}
	return false, fmt.Errorf("unknown operator %q", op)
}
