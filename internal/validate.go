package internal

import (
	"fmt"

	"github.com/pinpt/agent/v4/sdk"
	"github.com/pinpt/bitbucket/internal/api"
)

// Validate is called when the integration is requesting a validation from the app
func (g *BitBucketIntegration) Validate(validate sdk.Validate) (map[string]interface{}, error) {
	logger := validate.Logger()
	config := validate.Config()
	sdk.LogInfo(logger, "validate", "customer_id", validate.CustomerID())
	// FIXME(robin): make api okay with nil state/pipe
	a := api.New(logger, g.httpClient, nil, nil, validate.CustomerID(), validate.IntegrationInstanceID(), g.refType, g.getHTTPCredOpts(logger, config))
	workspaces, err := a.FetchWorkSpaces()
	if err != nil {
		return nil, fmt.Errorf("error fetching user workspaces: %w", err)
	}
	currentUser, err := a.FetchMyUser()
	if err != nil {
		return nil, fmt.Errorf("error fetching current user: %w", err)
	}
	var accounts []*sdk.ConfigAccount
	for _, workspace := range workspaces {
		count, err := a.FetchRepoCount(workspace.Slug)
		if err != nil {
			return nil, fmt.Errorf("error getting count of repos for workspace (%s): %w", workspace.Slug, err)
		}
		accType := sdk.ConfigAccountTypeOrg
		if isUserWorkspace(workspace, currentUser) {
			accType = sdk.ConfigAccountTypeUser
		}
		accounts = append(accounts, toAccount(workspace, accType, count))
	}
	return map[string]interface{}{
		"accounts": accounts,
	}, nil
}
