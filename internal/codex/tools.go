package codex

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// ToolSpec describes a dynamic tool advertised to the Codex app-server
// during the thread/start handshake.
type ToolSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// GraphQLExecutor is a function that executes a GraphQL query with variables
// and returns the raw JSON response body. The caller (e.g. the Linear client)
// is responsible for authentication.
type GraphQLExecutor func(query string, variables map[string]any) (json.RawMessage, error)

// toolDispatcher handles item/tool/call messages by routing to the
// appropriate handler based on tool name.
type toolDispatcher struct {
	graphqlExec GraphQLExecutor
}

// newToolDispatcher creates a dispatcher. graphqlExec may be nil if no
// GraphQL backend is configured; calls to linear_graphql will return an error.
func newToolDispatcher(graphqlExec GraphQLExecutor) *toolDispatcher {
	return &toolDispatcher{graphqlExec: graphqlExec}
}

// dispatch handles a tool call and returns the result payload to send back.
func (td *toolDispatcher) dispatch(params json.RawMessage) map[string]any {
	toolName := toolCallName(params)
	arguments := toolCallArguments(params)

	switch toolName {
	case "linear_graphql":
		return td.executeLinearGraphQL(arguments)
	default:
		slog.Warn("unsupported dynamic tool call", "tool", toolName)
		return failureResponse(map[string]any{
			"error": map[string]any{
				"message":        fmt.Sprintf("Unsupported dynamic tool: %q.", toolName),
				"supportedTools": []string{"linear_graphql"},
			},
		})
	}
}

// executeLinearGraphQL handles the linear_graphql tool call.
func (td *toolDispatcher) executeLinearGraphQL(arguments map[string]any) map[string]any {
	if td.graphqlExec == nil {
		return failureResponse(map[string]any{
			"error": map[string]any{
				"message": "Nimbus is missing Linear auth. Set `linear.api_key` in `WORKFLOW.md` or export `LINEAR_API_KEY`.",
			},
		})
	}

	query, variables, err := normalizeLinearGraphQLArguments(arguments)
	if err != nil {
		return failureResponse(map[string]any{
			"error": map[string]any{
				"message": err.Error(),
			},
		})
	}

	resp, err := td.graphqlExec(query, variables)
	if err != nil {
		return failureResponse(map[string]any{
			"error": map[string]any{
				"message": "Linear GraphQL request failed before receiving a successful response.",
				"reason":  err.Error(),
			},
		})
	}

	return graphqlResponse(resp)
}

// normalizeLinearGraphQLArguments extracts query and variables from the arguments map.
func normalizeLinearGraphQLArguments(arguments map[string]any) (string, map[string]any, error) {
	rawQuery, _ := arguments["query"]
	query, ok := rawQuery.(string)
	if !ok || strings.TrimSpace(query) == "" {
		return "", nil, fmt.Errorf("`linear_graphql` requires a non-empty `query` string")
	}
	query = strings.TrimSpace(query)

	variables := map[string]any{}
	if rawVars, exists := arguments["variables"]; exists && rawVars != nil {
		vars, ok := rawVars.(map[string]any)
		if !ok {
			return "", nil, fmt.Errorf("`linear_graphql.variables` must be a JSON object when provided")
		}
		variables = vars
	}

	return query, variables, nil
}

// graphqlResponse builds a success/failure tool result from a GraphQL response.
func graphqlResponse(resp json.RawMessage) map[string]any {
	// Check if the response contains errors.
	var parsed map[string]any
	success := true
	if json.Unmarshal(resp, &parsed) == nil {
		if errList, ok := parsed["errors"]; ok {
			if errs, ok := errList.([]any); ok && len(errs) > 0 {
				success = false
			}
		}
	}

	output := string(resp)
	return dynamicToolResponse(success, output)
}

// failureResponse builds a failure tool result.
func failureResponse(payload map[string]any) map[string]any {
	output, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		output = []byte(fmt.Sprintf("%v", payload))
	}
	return dynamicToolResponse(false, string(output))
}

// dynamicToolResponse builds the standard tool response envelope.
func dynamicToolResponse(success bool, output string) map[string]any {
	return map[string]any{
		"success": success,
		"output":  output,
		"contentItems": []map[string]any{
			{
				"type": "inputText",
				"text": output,
			},
		},
	}
}

// normalizeDynamicToolResult ensures a tool result has the expected shape.
func normalizeDynamicToolResult(result map[string]any) map[string]any {
	if _, ok := result["success"]; !ok {
		output := fmt.Sprintf("%v", result)
		return dynamicToolResponse(false, output)
	}

	// Ensure output field.
	if _, ok := result["output"].(string); !ok {
		if items, ok := result["contentItems"].([]any); ok && len(items) > 0 {
			if item, ok := items[0].(map[string]any); ok {
				if text, ok := item["text"].(string); ok {
					result["output"] = text
				}
			}
		}
		if _, ok := result["output"].(string); !ok {
			data, _ := json.MarshalIndent(result, "", "  ")
			result["output"] = string(data)
		}
	}

	// Ensure contentItems field.
	if _, ok := result["contentItems"].([]any); !ok {
		output, _ := result["output"].(string)
		result["contentItems"] = []map[string]any{
			{"type": "inputText", "text": output},
		}
	}

	return result
}

// toolCallName extracts the tool name from a tool/call params payload.
func toolCallName(params json.RawMessage) string {
	var p map[string]any
	if json.Unmarshal(params, &p) != nil {
		return ""
	}
	for _, key := range []string{"tool", "name"} {
		if v, ok := p[key].(string); ok {
			if trimmed := strings.TrimSpace(v); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

// toolCallArguments extracts the arguments map from a tool/call params payload.
func toolCallArguments(params json.RawMessage) map[string]any {
	var p map[string]any
	if json.Unmarshal(params, &p) != nil {
		return map[string]any{}
	}
	if args, ok := p["arguments"].(map[string]any); ok {
		return args
	}
	return map[string]any{}
}

// DefaultToolSpecs returns the standard set of dynamic tools advertised to Codex.
func DefaultToolSpecs() []ToolSpec {
	return []ToolSpec{
		{
			Name:        "linear_graphql",
			Description: "Execute a raw GraphQL query or mutation against Linear using Nimbus's configured auth.",
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"query"},
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "GraphQL query or mutation document to execute against Linear.",
					},
					"variables": map[string]any{
						"type":                 []string{"object", "null"},
						"description":          "Optional GraphQL variables object.",
						"additionalProperties": true,
					},
				},
			},
		},
	}
}
