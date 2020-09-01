package api

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/pinpt/agent.next/sdk"
)

// FetchPullRequests gets team members
func (a *API) FetchPullRequests(reponame string, repoid string, updated time.Time,
	prchan chan<- *sdk.SourceCodePullRequest,
	prcommentchan chan<- *sdk.SourceCodePullRequestComment,
	prcommitchan chan<- *sdk.SourceCodePullRequestCommit,
	prreviewchan chan<- *sdk.SourceCodePullRequestReview,
	prreviewrequestchan chan<- *sdk.SourceCodePullRequestReviewRequest,
) error {
	sdk.LogDebug(a.logger, "fetching pull requests", "repo", reponame)
	endpoint := sdk.JoinURL("repositories", reponame, "pullrequests")
	params := url.Values{}
	params.Add("state", "MERGED")
	params.Add("state", "SUPERSEDED")
	params.Add("state", "OPEN")
	if !updated.IsZero() {
		params.Set("q", `updated_on > `+updated.Format(updatedFormat))
	}
	params.Set("sort", "-updated_on")

	// Greater than 50 throws "Invalid pagelen"
	params.Set("pagelen", "50")

	out := make(chan objects)
	errchan := make(chan error)
	var count int
	go func() {
		for obj := range out {
			if len(obj) == 0 {
				continue
			}
			rawResponse := []PullRequestResponse{}
			if err := obj.Unmarshal(&rawResponse); err != nil {
				errchan <- err
				return
			}
			if err := a.processPullRequests(rawResponse, reponame, repoid, updated,
				prchan,
				prcommentchan,
				prcommitchan,
				prreviewchan,
				prreviewrequestchan,
			); err != nil {
				errchan <- err
				return
			}
			count += len(rawResponse)
		}
		errchan <- nil
	}()
	if err := a.paginate(endpoint, params, out); err != nil {
		return fmt.Errorf("error fetching prs. err %v", err)
	}
	if err := <-errchan; err != nil {
		return err
	}
	sdk.LogDebug(a.logger, "finished fetching pull requests", "repo", reponame, "count", count)
	return nil
}

func (a *API) processPullRequests(raw []PullRequestResponse, reponame string, repoid string, updated time.Time,
	prchan chan<- *sdk.SourceCodePullRequest,
	prcommentchan chan<- *sdk.SourceCodePullRequestComment,
	prcommitchan chan<- *sdk.SourceCodePullRequestCommit,
	prreviewchan chan<- *sdk.SourceCodePullRequestReview,
	prreviewrequestchan chan<- *sdk.SourceCodePullRequestReviewRequest,
) error {
	async := sdk.NewAsync(10)
	for _, _pr := range raw {
		pr := _pr
		async.Do(func() error {
			return a.fetchPullRequestComments(pr, reponame, repoid, updated, prcommentchan)
		})
		async.Do(func() error {
			return a.fetchPullRequestCommits(pr, reponame, repoid, updated, prcommitchan)
		})
		a.sendPullRequestReview(pr, repoid, prreviewchan, prreviewrequestchan)
	}
	if err := async.Wait(); err != nil {
		return err
	}
	// we need the first commit of every pr in the pr object, wait for the commits to be fetched before processing prs
	for _, pr := range raw {
		a.sendPullRequest(pr, repoid, updated, prchan)
	}
	return nil
}

func (a *API) sendPullRequestReview(raw PullRequestResponse, repoid string, prreviewchan chan<- *sdk.SourceCodePullRequestReview, prreviewrequestchan chan<- *sdk.SourceCodePullRequestReviewRequest) {
	repoID := sdk.NewSourceCodeRepoID(a.customerID, repoid, a.refType)
	prID := sdk.NewSourceCodePullRequestID(a.customerID, strconv.FormatInt(raw.ID, 10), a.refType, repoID)
	for _, participant := range raw.Participants {
		if participant.Role == "REVIEWER" {
			if participant.Approved {
				prreviewchan <- &sdk.SourceCodePullRequestReview{
					Active:        true,
					CustomerID:    a.customerID,
					PullRequestID: prID,
					RefID:         sdk.Hash(raw.ID, participant.User.AccountID),
					RefType:       a.refType,
					RepoID:        repoID,
					UserRefID:     participant.User.UUID,
					State:         sdk.SourceCodePullRequestReviewStateApproved,
				}
			} else if participant.ParticipatedOn.IsZero() {
				// a non-participated reviewer is counted as a request
				prreviewrequestchan <- &sdk.SourceCodePullRequestReviewRequest{
					Active:                 true,
					CreatedDate:            sdk.SourceCodePullRequestReviewRequestCreatedDate(*sdk.NewDateWithTime(raw.UpdatedOn)),
					RequestedReviewerRefID: participant.User.UUID,
					RefType:                a.refType,
					PullRequestID:          prID,
					CustomerID:             a.customerID,
					ID:                     sdk.NewSourceCodePullRequestReviewRequestID(a.customerID, a.refType, prID, participant.User.UUID),
				}
			}
		}
	}
}

// ConvertPullRequest converts from raw response to pinpoint object
func (a *API) ConvertPullRequest(raw PullRequestResponse, repoid, firstsha string) *sdk.SourceCodePullRequest {

	commitid := sdk.NewSourceCodeCommitID(a.customerID, firstsha, a.refType, repoid)
	pr := &sdk.SourceCodePullRequest{
		Active:         true,
		CustomerID:     a.customerID,
		RefType:        a.refType,
		RefID:          fmt.Sprint(raw.ID),
		RepoID:         sdk.NewSourceCodeRepoID(a.customerID, repoid, a.refType),
		BranchID:       sdk.NewSourceCodeBranchID(a.customerID, repoid, a.refType, raw.Source.Branch.Name, commitid),
		BranchName:     raw.Source.Branch.Name,
		Title:          raw.Title,
		Description:    `<div class="source-bitbucket">` + sdk.ConvertMarkdownToHTML(raw.Description) + "</div>",
		URL:            raw.Links.HTML.Href,
		Identifier:     fmt.Sprintf("#%d", raw.ID), // in bitbucket looks like #1 is the format for PR identifiers in their UI
		CreatedByRefID: raw.Author.UUID,
	}
	sdk.ConvertTimeToDateModel(raw.CreatedOn, &pr.CreatedDate)
	sdk.ConvertTimeToDateModel(raw.UpdatedOn, &pr.MergedDate)
	sdk.ConvertTimeToDateModel(raw.UpdatedOn, &pr.ClosedDate)
	sdk.ConvertTimeToDateModel(raw.UpdatedOn, &pr.UpdatedDate)
	switch raw.State {
	case "OPEN":
		pr.Status = sdk.SourceCodePullRequestStatusOpen
	case "DECLINED":
		pr.Status = sdk.SourceCodePullRequestStatusClosed
		pr.ClosedByRefID = raw.ClosedBy.AccountID
	case "MERGED":
		pr.MergeSha = raw.MergeCommit.Hash
		pr.MergeCommitID = sdk.NewSourceCodeCommitID(a.customerID, raw.MergeCommit.Hash, a.refType, pr.RepoID)
		pr.MergedByRefID = raw.ClosedBy.AccountID
		pr.Status = sdk.SourceCodePullRequestStatusMerged
	default:
		sdk.LogError(a.logger, "PR has an unknown state", "state", raw.State, "ref_id", pr.RefID)
	}
	return pr
}

func (a *API) sendPullRequest(raw PullRequestResponse, repoid string, updated time.Time, prchan chan<- *sdk.SourceCodePullRequest) {
	if raw.UpdatedOn.Before(updated) {
		return
	}
	prid := fmt.Sprint(raw.ID)
	var firstsha string
	ok, _ := a.state.Get(FirstSha(repoid, prid), &firstsha)
	if !ok {
		sdk.LogInfo(a.logger, "no first commit sha found for pr", "pr", raw.ID, "repo", repoid)
	}
	prchan <- a.ConvertPullRequest(raw, repoid, firstsha)
}
