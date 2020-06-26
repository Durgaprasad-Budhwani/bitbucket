package api

import (
	"fmt"
	"net/url"

	"github.com/pinpt/agent.next/sdk"
)

// FetchWorkSpaces returns all workspaces
func (a *API) FetchWorkSpaces() ([]string, error) {
	sdk.LogDebug(a.logger, "fetching workspaces")
	endpoint := "workspaces"
	params := url.Values{}
	params.Set("pagelen", "100")
	params.Set("role", "member")

	var ids []string
	out := make(chan objects)
	errchan := make(chan error, 1)
	go func() {
		for obj := range out {
			res := []workSpacesResponse{}
			if err := obj.Unmarshal(&res); err != nil {
				errchan <- err
				return
			}
			ids = append(ids, workSpaceIDs(res)...)
		}
		errchan <- nil
	}()
	go func() {
		err := a.paginate(endpoint, params, out)
		if err != nil {
			errchan <- fmt.Errorf("error fetching workspaces. err %v", err)
		}
	}()
	if err := <-errchan; err != nil {
		return nil, err
	}
	sdk.LogDebug(a.logger, "finished fetching workspaces")
	return ids, nil
}

func workSpaceIDs(ws []workSpacesResponse) []string {
	var ids []string
	for _, w := range ws {
		ids = append(ids, w.Slug)
	}
	return ids
}