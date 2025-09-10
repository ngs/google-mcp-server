package slides

import (
	"context"
	"encoding/json"
	"fmt"

	"go.ngs.io/google-mcp-server/auth"
	"go.ngs.io/google-mcp-server/server"
)

type MultiAccountService struct {
	authManager *auth.AccountManager
}

func NewMultiAccountService(authManager *auth.AccountManager) *MultiAccountService {
	return &MultiAccountService{
		authManager: authManager,
	}
}

func (s *MultiAccountService) GetTools() []server.Tool {
	return []server.Tool{
		{
			Name:        "slides_presentations_list_all_accounts",
			Description: "List presentations from all authenticated Google accounts",
			InputSchema: server.InputSchema{
				Type: "object",
				Properties: map[string]server.Property{
					"max_results": {
						Type:        "number",
						Description: "Maximum number of presentations per account",
					},
				},
			},
		},
	}
}

func (s *MultiAccountService) HandleToolCall(ctx context.Context, name string, arguments json.RawMessage) (interface{}, error) {
	switch name {
	case "slides_presentations_list_all_accounts":
		var args map[string]interface{}
		if err := json.Unmarshal(arguments, &args); err != nil {
			return nil, err
		}

		maxResults := 10
		if mr, ok := args["max_results"].(float64); ok {
			maxResults = int(mr)
		}

		accounts := s.authManager.ListAccounts()
		allPresentations := []map[string]interface{}{}

		for _, account := range accounts {
			// Skip if no OAuth client
			if account.OAuthClient == nil {
				continue
			}

			// Create Drive client to list presentations (Slides API doesn't have direct list)
			// We would typically use Drive API to list presentations
			// For now, we'll return a placeholder
			accountPresentations := map[string]interface{}{
				"account":     account.Email,
				"note":        "Use Drive API with mimeType='application/vnd.google-apps.presentation' to list presentations",
				"max_results": maxResults,
			}

			allPresentations = append(allPresentations, accountPresentations)
		}

		return map[string]interface{}{
			"accounts":      len(accounts),
			"presentations": allPresentations,
		}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}
