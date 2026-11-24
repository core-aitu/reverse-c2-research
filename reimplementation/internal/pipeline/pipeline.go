package pipeline

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const maxNodes = 128

type Definition struct {
	Version int      `json:"version"`
	Nodes   []Node   `json:"nodes"`
	Outputs []string `json:"outputs"`
}

type Node struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	DependsOn []string `json:"dependsOn"`
	Value     string   `json:"value,omitempty"`
}

type Result struct {
	Outputs map[string]string `json:"outputs"`
	Order   []string          `json:"order"`
}

var safeTypes = map[string]bool{
	"input": true, "literal": true, "concat": true, "base64": true,
	"base64decode": true, "sha256": true, "uppercase": true, "lowercase": true,
}

func Parse(raw json.RawMessage) (Definition, error) {
	var definition Definition
	if err := json.Unmarshal(raw, &definition); err != nil {
		return Definition{}, errors.New("invalid pipeline definition")
	}
	if definition.Version != 1 || len(definition.Nodes) == 0 || len(definition.Nodes) > maxNodes {
		return Definition{}, errors.New("pipeline must use version 1 and contain 1-128 nodes")
	}
	if len(definition.Outputs) == 0 {
		return Definition{}, errors.New("pipeline must declare outputs")
	}
	return definition, nil
}

func Validate(definition Definition) ([]string, error) {
	nodes := make(map[string]Node, len(definition.Nodes))
	for _, node := range definition.Nodes {
		if node.ID == "" || len(node.ID) > 64 || nodes[node.ID].ID != "" {
			return nil, errors.New("node IDs must be unique and non-empty")
		}
		if !safeTypes[node.Type] {
			return nil, fmt.Errorf("unsupported safe node type %q", node.Type)
		}
		nodes[node.ID] = node
	}
	for _, output := range definition.Outputs {
		if nodes[output].ID == "" {
			return nil, fmt.Errorf("unknown output node %q", output)
		}
	}
	state := map[string]uint8{}
	order := make([]string, 0, len(nodes))
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return errors.New("pipeline contains a cycle")
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		node := nodes[id]
		for _, dependency := range node.DependsOn {
			if nodes[dependency].ID == "" {
				return fmt.Errorf("node %q depends on unknown node %q", id, dependency)
			}
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[id] = 2
		order = append(order, id)
		return nil
	}
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return order, nil
}

func Execute(definition Definition, input string) (Result, error) {
	if len(input) > 1<<20 {
		return Result{}, errors.New("pipeline input exceeds 1 MiB")
	}
	order, err := Validate(definition)
	if err != nil {
		return Result{}, err
	}
	nodes := make(map[string]Node, len(definition.Nodes))
	for _, node := range definition.Nodes {
		nodes[node.ID] = node
	}
	values := map[string]string{}
	for _, id := range order {
		node := nodes[id]
		dependencies := make([]string, 0, len(node.DependsOn))
		for _, dependency := range node.DependsOn {
			dependencies = append(dependencies, values[dependency])
		}
		value, err := runNode(node, dependencies, input)
		if err != nil {
			return Result{}, fmt.Errorf("node %s: %w", id, err)
		}
		if len(value) > 2<<20 {
			return Result{}, fmt.Errorf("node %s output exceeds 2 MiB", id)
		}
		values[id] = value
	}
	outputs := map[string]string{}
	for _, id := range definition.Outputs {
		outputs[id] = values[id]
	}
	return Result{Outputs: outputs, Order: order}, nil
}

func runNode(node Node, dependencies []string, input string) (string, error) {
	joined := strings.Join(dependencies, "")
	switch node.Type {
	case "input":
		return input, nil
	case "literal":
		return node.Value, nil
	case "concat":
		return joined, nil
	case "base64":
		return base64.StdEncoding.EncodeToString([]byte(joined)), nil
	case "base64decode":
		decoded, err := base64.StdEncoding.DecodeString(joined)
		return string(decoded), err
	case "sha256":
		digest := sha256.Sum256([]byte(joined))
		return hex.EncodeToString(digest[:]), nil
	case "uppercase":
		return strings.ToUpper(joined), nil
	case "lowercase":
		return strings.ToLower(joined), nil
	default:
		return "", errors.New("unsupported node")
	}
}
