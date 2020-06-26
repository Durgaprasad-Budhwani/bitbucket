package internal

import (
	"errors"
	"strings"
	"time"

	"github.com/pinpt/agent.next.bitbucket/internal/api"
	"github.com/pinpt/agent.next/sdk"
)

// BitBucketIntegration is an integration for BitBucket
type BitBucketIntegration struct {
	logger  sdk.Logger
	config  sdk.Config
	manager sdk.Manager
	refType string

	httpClient sdk.HTTPClient
}

var _ sdk.Integration = (*BitBucketIntegration)(nil)

// Start is called when the integration is starting up
func (g *BitBucketIntegration) Start(logger sdk.Logger, config sdk.Config, manager sdk.Manager) error {
	g.logger = sdk.LogWith(logger, "pkg", "bitbucket")
	g.config = config
	g.manager = manager
	g.refType = "bitbucket"
	sdk.LogInfo(g.logger, "starting")
	return nil
}

// Enroll is called when a new integration instance is added
func (g *BitBucketIntegration) Enroll(instance sdk.Instance) error {
	sdk.LogInfo(g.logger, "enroll not implemented")
	return nil
}

// Dismiss is called when an existing integration instance is removed
func (g *BitBucketIntegration) Dismiss(instance sdk.Instance) error {
	sdk.LogInfo(g.logger, "dismiss not implemented")
	return nil
}

// WebHook is called when a webhook is received on behalf of the integration
func (g *BitBucketIntegration) WebHook(webhook sdk.WebHook) error {
	sdk.LogInfo(g.logger, "webhook not implemented")
	return nil
}

// Stop is called when the integration is shutting down for cleanup
func (g *BitBucketIntegration) Stop() error {
	sdk.LogInfo(g.logger, "stopping")
	return nil
}

// Export is called to tell the integration to run an export
func (g *BitBucketIntegration) Export(export sdk.Export) error {
	sdk.LogInfo(g.logger, "export started")

	// Pipe must be called to begin an export and receive a pipe for sending data
	pipe := export.Pipe()

	// State is a customer specific state object for this integration and customer
	state := export.State()

	// CustomerID will return the customer id for the export
	customerID := export.CustomerID()

	// Config is any customer specific configuration for this customer
	config := export.Config()
	if config.BasicAuth == nil {
		return errors.New("missing username")
	}
	hasInclusions := config.Inclusions != nil
	hasExclusions := config.Exclusions != nil
	accounts := config.Accounts
	if accounts == nil {
		sdk.LogInfo(g.logger, "no accounts configured, will do only customer's account")
	}

	g.httpClient = g.manager.HTTPManager().New("https://api.bitbucket.org/2.0", nil)

	sdk.LogInfo(g.logger, "export starting", "customer", customerID)

	client := g.httpClient
	creds := &api.BasicCreds{
		Username: config.BasicAuth.Username,
		Password: config.BasicAuth.Password,
	}

	var updated time.Time
	if !export.Historical() {
		var strTime string
		if ok, _ := state.Get("updated", &strTime); ok {
			updated, _ = time.Parse(time.RFC3339Nano, strTime)
		}
	}
	a := api.New(g.logger, client, creds, customerID, g.refType)
	teams, err := a.FetchWorkSpaces()
	if err != nil {
		return err
	}
	if accounts != nil {
		for name := range *accounts {
			teams = append(teams, name)
		}
	}

	errchan := make(chan error)
	repochan := make(chan *sdk.SourceCodeRepo)
	userchan := make(chan *sdk.SourceCodeUser)
	prchan := make(chan *sdk.SourceCodePullRequest)
	prcommentchan := make(chan *sdk.SourceCodePullRequestComment)
	prcommitchan := make(chan *sdk.SourceCodePullRequestCommit)
	prreviewchan := make(chan *sdk.SourceCodePullRequestReview)

	// =========== repo ============
	go func() {
		var count int
		for r := range repochan {
			if hasInclusions || hasExclusions {
				name := strings.Split(r.Name, "/")
				if hasInclusions && !config.Inclusions.Matches(name[0], r.Name) {
					continue
				}
				if hasExclusions && config.Exclusions.Matches(name[0], r.Name) {
					continue
				}
			}

			if err := pipe.Write(r); err != nil {
				errchan <- err
				return
			}
			if err := a.FetchPullRequests(r.Name, r.RefID, updated,
				prchan,
				prcommentchan,
				prcommitchan,
				prreviewchan,
			); err != nil {
				errchan <- err
				return
			}
			count++
		}
		sdk.LogDebug(g.logger, "finished sending repos", "len", count)
	}()
	// =========== prs ============
	go func() {
		var count int
		for r := range prchan {
			if err := pipe.Write(r); err != nil {
				errchan <- err
				return
			}
			count++
		}
		sdk.LogDebug(g.logger, "finished sending prs", "len", count)
	}()
	// =========== pr comment ============
	go func() {
		var count int
		for r := range prcommentchan {
			if err := pipe.Write(r); err != nil {
				errchan <- err
				return
			}
			count++
		}
		sdk.LogDebug(g.logger, "finished sending pr comments", "len", count)
	}()
	// =========== pr commit ============
	go func() {
		var count int
		for r := range prcommitchan {
			if err := pipe.Write(r); err != nil {
				errchan <- err
				return
			}
			count++
		}
		sdk.LogDebug(g.logger, "finished sending commits", "len", count)
	}()
	// =========== pr review ============
	go func() {
		var count int
		for r := range prreviewchan {
			if err := pipe.Write(r); err != nil {
				errchan <- err
				return
			}
			count++
		}
		sdk.LogDebug(g.logger, "finished sending reviews", "len", count)
	}()
	// =========== user ============
	go func() {
		var count int
		for r := range userchan {
			if err := pipe.Write(r); err != nil {
				errchan <- err
				return
			}
			count++
		}
		sdk.LogDebug(g.logger, "finished sending users", "len", count)
	}()
	go func() {
		for _, team := range teams {
			if err := a.FetchRepos(team, updated, repochan); err != nil {
				sdk.LogError(g.logger, "error fetching repos", "err", err)
				errchan <- err
				return
			}
			if err := a.FetchUsers(team, updated, userchan); err != nil {
				sdk.LogError(g.logger, "error fetching repos", "err", err)
				errchan <- err
				return
			}
		}
		errchan <- nil
	}()

	if err := <-errchan; err != nil {
		sdk.LogError(g.logger, "export finished with error", "err", err)
		return err
	}
	state.Set("updated", time.Now().Format(time.RFC3339Nano))

	close(repochan)
	close(userchan)
	close(prchan)
	close(prcommentchan)
	close(prcommitchan)
	close(prreviewchan)

	sdk.LogInfo(g.logger, "export finished")

	return nil
}