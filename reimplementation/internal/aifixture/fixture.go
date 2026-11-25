package aifixture

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type Definition struct {
	Version   int      `json:"version"`
	Operation string   `json:"operation"`
	Argument  string   `json:"argument,omitempty"`
	Arguments []string `json:"arguments,omitempty"`
	Separator string   `json:"separator,omitempty"`
}

func Parse(raw string) (Definition, error) {
	var value Definition
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return Definition{}, errors.New("AI fixture definition must be valid JSON")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Definition{}, errors.New("AI fixture definition must contain one JSON value")
	}
	if value.Version != 1 {
		return Definition{}, errors.New("AI fixture definition version must be 1")
	}
	switch value.Operation {
	case "echo", "sha256":
		if !validKey(value.Argument) || len(value.Arguments) != 0 || value.Separator != "" {
			return Definition{}, errors.New("AI fixture operation requires one argument key")
		}
	case "concat":
		if value.Argument != "" || len(value.Arguments) < 1 || len(value.Arguments) > 16 || len(value.Separator) > 64 {
			return Definition{}, errors.New("AI concat fixture requires 1 to 16 argument keys")
		}
		for _, key := range value.Arguments {
			if !validKey(key) {
				return Definition{}, errors.New("invalid AI fixture argument key")
			}
		}
	default:
		return Definition{}, errors.New("unsupported AI fixture operation")
	}
	return value, nil
}

func Run(definition Definition, arguments map[string]string) (string, error) {
	for key, value := range arguments {
		if !validKey(key) || len(value) > 64<<10 {
			return "", errors.New("invalid AI fixture argument")
		}
	}
	switch definition.Operation {
	case "echo":
		return required(arguments, definition.Argument)
	case "sha256":
		value, err := required(arguments, definition.Argument)
		if err != nil {
			return "", err
		}
		digest := sha256.Sum256([]byte(value))
		return hex.EncodeToString(digest[:]), nil
	case "concat":
		values := make([]string, 0, len(definition.Arguments))
		for _, key := range definition.Arguments {
			value, err := required(arguments, key)
			if err != nil {
				return "", err
			}
			values = append(values, value)
		}
		return strings.Join(values, definition.Separator), nil
	default:
		return "", errors.New("unsupported AI fixture operation")
	}
}

func validKey(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if !(character == '_' || character == '-' || character == '.' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9') {
			return false
		}
	}
	return true
}

func required(arguments map[string]string, key string) (string, error) {
	value, ok := arguments[key]
	if !ok {
		return "", errors.New("missing AI fixture argument: " + key)
	}
	return value, nil
}
