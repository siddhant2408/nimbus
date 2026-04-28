// Package linear implements the tracker.Tracker interface using Linear's
// GraphQL API. It translates the polling queries from SPEC Section 5.3.1
// into paginated GraphQL requests.
package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/anthropics/symphony/internal/config"
	"github.com/anthropics/symphony/internal/domain"
)

const (
	issuePageSize = 50
	httpTimeout   = 30 * time.Second
)

// GraphQL query for polling candidate issues by project + state, with cursor pagination.
const queryPoll = `
query SymphonyLinearPoll($projectSlug: String!, $stateNames: [String!]!, $first: Int!, $relationFirst: Int!, $after: String) {
  issues(filter: {project: {slugId: {eq: $projectSlug}}, state: {name: {in: $stateNames}}}, first: $first, after: $after) {
    nodes {
      id
      identifier
      title
      description
      priority
      state {
        name
      }
      branchName
      url
      assignee {
        id
      }
      labels {
        nodes {
          name
        }
      }
      inverseRelations(first: $relationFirst) {
        nodes {
          type
          issue {
            id
            identifier
            state {
              name
            }
          }
        }
      }
      createdAt
      updatedAt
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}
`

// GraphQL query for fetching issues by their IDs (no cursor pagination, batched by caller).
const queryByIDs = `
query SymphonyLinearIssuesById($ids: [ID!]!, $first: Int!, $relationFirst: Int!) {
  issues(filter: {id: {in: $ids}}, first: $first) {
    nodes {
      id
      identifier
      title
      description
      priority
      state {
        name
      }
      branchName
      url
      assignee {
        id
      }
      labels {
        nodes {
          name
        }
      }
      inverseRelations(first: $relationFirst) {
        nodes {
          type
          issue {
            id
            identifier
            state {
              name
            }
          }
        }
      }
      createdAt
      updatedAt
    }
  }
}
`

// GraphQL query for resolving the current viewer's identity (for assignee="me").
const queryViewer = `
query SymphonyLinearViewer {
  viewer {
    id
  }
}
`

// Client is a Linear GraphQL client that implements tracker.Tracker.
type Client struct {
	cfg        func() *config.Config
	httpClient *http.Client
}

// NewClient creates a new Linear client. The cfg function is called on every
// request so that hot-reloaded config changes (endpoint, API key, etc.) take
// effect immediately.
func NewClient(cfg func() *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

// ---- tracker.Tracker implementation -----------------------------------------

// FetchCandidateIssues returns issues in the configured active states for the
// configured project, optionally filtered by assignee.
func (c *Client) FetchCandidateIssues(ctx context.Context) ([]domain.Issue, error) {
	cfg := c.cfg()
	if cfg.Tracker.APIKey == "" {
		return nil, &TrackerError{Code: "missing_api_key", Message: "tracker.api_key is required"}
	}
	if cfg.Tracker.ProjectSlug == "" {
		return nil, &TrackerError{Code: "missing_project_slug", Message: "tracker.project_slug is required"}
	}

	assigneeFilter, err := c.resolveAssigneeFilter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve assignee filter: %w", err)
	}

	return c.fetchByStates(ctx, cfg, cfg.Tracker.ActiveStates, assigneeFilter)
}

// FetchIssuesByStates returns all issues whose state name matches one of the
// given values.
func (c *Client) FetchIssuesByStates(ctx context.Context, states []string) ([]domain.Issue, error) {
	states = uniqueStrings(states)
	if len(states) == 0 {
		return nil, nil
	}

	cfg := c.cfg()
	if cfg.Tracker.APIKey == "" {
		return nil, &TrackerError{Code: "missing_api_key", Message: "tracker.api_key is required"}
	}
	if cfg.Tracker.ProjectSlug == "" {
		return nil, &TrackerError{Code: "missing_project_slug", Message: "tracker.project_slug is required"}
	}

	return c.fetchByStates(ctx, cfg, states, nil)
}

// FetchIssueStatesByIDs fetches the current state of the given issue IDs,
// batching requests in groups of 50. Results are returned in the same order
// as the input IDs (missing IDs are omitted).
func (c *Client) FetchIssueStatesByIDs(ctx context.Context, ids []string) ([]domain.Issue, error) {
	ids = uniqueStrings(ids)
	if len(ids) == 0 {
		return nil, nil
	}

	cfg := c.cfg()
	if cfg.Tracker.APIKey == "" {
		return nil, &TrackerError{Code: "missing_api_key", Message: "tracker.api_key is required"}
	}

	assigneeFilter, err := c.resolveAssigneeFilter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve assignee filter: %w", err)
	}

	// Build an order index so we can restore the caller's requested order.
	orderIndex := make(map[string]int, len(ids))
	for i, id := range ids {
		orderIndex[id] = i
	}

	var allIssues []domain.Issue

	for start := 0; start < len(ids); start += issuePageSize {
		end := start + issuePageSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]

		body, err := c.graphql(ctx, cfg, queryByIDs, map[string]any{
			"ids":           batch,
			"first":         len(batch),
			"relationFirst": issuePageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("fetch issues by IDs batch: %w", err)
		}

		issues, err := decodeIssuesResponse(body, assigneeFilter)
		if err != nil {
			return nil, err
		}

		allIssues = append(allIssues, issues...)
	}

	// Restore requested order.
	sortIssuesByRequestedOrder(allIssues, orderIndex)

	return allIssues, nil
}

// ---- Pagination helpers -----------------------------------------------------

func (c *Client) fetchByStates(ctx context.Context, cfg *config.Config, states []string, assigneeFilter *assigneeMatch) ([]domain.Issue, error) {
	var allIssues []domain.Issue
	var cursor *string

	for {
		vars := map[string]any{
			"projectSlug":   cfg.Tracker.ProjectSlug,
			"stateNames":    states,
			"first":         issuePageSize,
			"relationFirst": issuePageSize,
		}
		if cursor != nil {
			vars["after"] = *cursor
		}

		body, err := c.graphql(ctx, cfg, queryPoll, vars)
		if err != nil {
			return nil, fmt.Errorf("fetch issues by states: %w", err)
		}

		issues, pageInfo, err := decodePagedIssuesResponse(body, assigneeFilter)
		if err != nil {
			return nil, err
		}

		allIssues = append(allIssues, issues...)

		if !pageInfo.HasNextPage {
			break
		}
		if pageInfo.EndCursor == "" {
			return nil, &TrackerError{Code: "missing_end_cursor", Message: "Linear returned hasNextPage=true but no endCursor"}
		}
		cursor = &pageInfo.EndCursor
	}

	return allIssues, nil
}

// ---- GraphQL transport ------------------------------------------------------

// graphqlRequest is the JSON payload sent to the Linear GraphQL endpoint.
type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// GraphQL executes an arbitrary GraphQL query against the configured Linear
// endpoint and returns the parsed JSON response body. It is exported so that
// dynamic tools (e.g. the linear_graphql tool) can reuse the authenticated
// transport.
func (c *Client) GraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	return c.graphql(ctx, c.cfg(), query, variables)
}

// graphql is the internal implementation shared by all query methods.
func (c *Client) graphql(ctx context.Context, cfg *config.Config, query string, variables map[string]any) (map[string]any, error) {
	payload, err := json.Marshal(graphqlRequest{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Tracker.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build http request: %w", err)
	}
	req.Header.Set("Authorization", cfg.Tracker.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("linear api request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read linear response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("Linear GraphQL request failed",
			"status", resp.StatusCode,
			"body", truncateBody(respBody, 1000),
		)
		return nil, &TrackerError{
			Code:    "linear_api_status",
			Message: fmt.Sprintf("Linear API returned HTTP %d", resp.StatusCode),
		}
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode linear response: %w", err)
	}

	// Surface GraphQL-level errors.
	if errs, ok := result["errors"]; ok {
		return nil, &TrackerError{
			Code:    "linear_graphql_errors",
			Message: fmt.Sprintf("Linear GraphQL errors: %v", errs),
		}
	}

	return result, nil
}

// ---- Assignee filtering -----------------------------------------------------

// assigneeMatch holds the resolved set of assignee IDs to filter on.
type assigneeMatch struct {
	matchValues map[string]struct{}
}

func (c *Client) resolveAssigneeFilter(ctx context.Context, cfg *config.Config) (*assigneeMatch, error) {
	raw := cfg.Tracker.Assignee
	if raw == "" {
		return nil, nil
	}

	if raw == "me" {
		return c.resolveViewerAssigneeFilter(ctx, cfg)
	}

	return &assigneeMatch{
		matchValues: map[string]struct{}{raw: {}},
	}, nil
}

func (c *Client) resolveViewerAssigneeFilter(ctx context.Context, cfg *config.Config) (*assigneeMatch, error) {
	body, err := c.graphql(ctx, cfg, queryViewer, map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("resolve viewer identity: %w", err)
	}

	data, _ := body["data"].(map[string]any)
	if data == nil {
		return nil, &TrackerError{Code: "missing_viewer_identity", Message: "could not resolve Linear viewer identity"}
	}
	viewer, _ := data["viewer"].(map[string]any)
	if viewer == nil {
		return nil, &TrackerError{Code: "missing_viewer_identity", Message: "could not resolve Linear viewer identity"}
	}
	viewerID, _ := viewer["id"].(string)
	if viewerID == "" {
		return nil, &TrackerError{Code: "missing_viewer_identity", Message: "could not resolve Linear viewer identity"}
	}

	return &assigneeMatch{
		matchValues: map[string]struct{}{viewerID: {}},
	}, nil
}

// matchesAssignee returns true when the issue should be included given the
// optional assignee filter.
func matchesAssignee(assigneeID *string, filter *assigneeMatch) bool {
	if filter == nil {
		return true
	}
	if assigneeID == nil || *assigneeID == "" {
		return false
	}
	_, ok := filter.matchValues[*assigneeID]
	return ok
}

// ---- Response decoding ------------------------------------------------------

// pageInfo holds pagination metadata from a Linear GraphQL response.
type pageInfo struct {
	HasNextPage bool
	EndCursor   string
}

// decodePagedIssuesResponse extracts issues and pagination info from a poll response.
func decodePagedIssuesResponse(body map[string]any, filter *assigneeMatch) ([]domain.Issue, pageInfo, error) {
	data, _ := body["data"].(map[string]any)
	if data == nil {
		return nil, pageInfo{}, &TrackerError{Code: "linear_unknown_payload", Message: "missing data in Linear response"}
	}
	issuesObj, _ := data["issues"].(map[string]any)
	if issuesObj == nil {
		return nil, pageInfo{}, &TrackerError{Code: "linear_unknown_payload", Message: "missing data.issues in Linear response"}
	}

	nodes, _ := issuesObj["nodes"].([]any)
	issues := normalizeIssueNodes(nodes, filter)

	pi := pageInfo{}
	if piObj, ok := issuesObj["pageInfo"].(map[string]any); ok {
		if hn, ok := piObj["hasNextPage"].(bool); ok {
			pi.HasNextPage = hn
		}
		if ec, ok := piObj["endCursor"].(string); ok {
			pi.EndCursor = ec
		}
	}

	return issues, pi, nil
}

// decodeIssuesResponse extracts issues from a non-paginated (by-IDs) response.
func decodeIssuesResponse(body map[string]any, filter *assigneeMatch) ([]domain.Issue, error) {
	data, _ := body["data"].(map[string]any)
	if data == nil {
		return nil, &TrackerError{Code: "linear_unknown_payload", Message: "missing data in Linear response"}
	}
	issuesObj, _ := data["issues"].(map[string]any)
	if issuesObj == nil {
		return nil, &TrackerError{Code: "linear_unknown_payload", Message: "missing data.issues in Linear response"}
	}

	nodes, _ := issuesObj["nodes"].([]any)
	issues := normalizeIssueNodes(nodes, filter)

	return issues, nil
}

// ---- Sorting helper ---------------------------------------------------------

func sortIssuesByRequestedOrder(issues []domain.Issue, orderIndex map[string]int) {
	fallback := len(orderIndex)
	for i := 1; i < len(issues); i++ {
		key := issues[i]
		keyIdx := fallback
		if idx, ok := orderIndex[key.ID]; ok {
			keyIdx = idx
		}
		j := i - 1
		for j >= 0 {
			jIdx := fallback
			if idx, ok := orderIndex[issues[j].ID]; ok {
				jIdx = idx
			}
			if jIdx <= keyIdx {
				break
			}
			issues[j+1] = issues[j]
			j--
		}
		issues[j+1] = key
	}
}

// ---- Misc helpers -----------------------------------------------------------

// TrackerError is a typed error for Linear tracker failures.
type TrackerError struct {
	Code    string
	Message string
}

func (e *TrackerError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func truncateBody(body []byte, maxLen int) string {
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "...<truncated>"
}

func uniqueStrings(ss []string) []string {
	seen := make(map[string]struct{}, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
