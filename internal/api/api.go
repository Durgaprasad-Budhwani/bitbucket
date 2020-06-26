package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pinpt/agent.next/sdk"
)

// Creds generic credentials object
type Creds interface {
	auth() string
}

// BasicCreds basic authorization object, username and password
type BasicCreds struct {
	Username string
	Password string
}

func (b *BasicCreds) auth() string {
	auth := b.Username + ":" + b.Password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}

// OAuthCreds oauth2 authorization object
type OAuthCreds struct {
	Token   string
	Refresh string
	Manager sdk.Manager
}

func (o *OAuthCreds) auth() string {
	return "Bearer " + o.Token
}
func (o *OAuthCreds) refresh(refType string) error {
	token, err := o.Manager.RefreshOAuth2Token(refType, o.Refresh)
	if err != nil {
		return err
	}
	o.Token = token
	return nil
}

var _ Creds = (*BasicCreds)(nil)
var _ Creds = (*OAuthCreds)(nil)

// API the api object
type API struct {
	client     sdk.HTTPClient
	creds      Creds
	refType    string
	customerID string
	logger     sdk.Logger
}

func New(logger sdk.Logger, client sdk.HTTPClient, creds Creds, customerID, refType string) *API {
	return &API{
		logger:     logger,
		client:     client,
		creds:      creds,
		customerID: customerID,
		refType:    refType,
	}
}

func (a *API) paginate(endpoint string, params url.Values, out chan<- objects) error {
	defer close(out)
	var page string
	for {
		var res paginationResponse
		if page != "" {
			params.Set("page", page)
		}
		_, err := a.get(endpoint, params, &res)
		if err != nil {
			return err
		}
		out <- res.Values
		if res.Next == "" {
			return nil
		}
		u, _ := url.Parse(res.Next)
		page = u.Query().Get("page")
		if page == "" {
			return fmt.Errorf("no `page` in next. %v", u.String())
		}
	}
}

func (a *API) get(endpoint string, params url.Values, out interface{}) (*sdk.HTTPResponse, error) {

	resp, err := a.client.Get(out, sdk.WithEndpoint(endpoint), sdk.WithGetQueryParameters(params), sdk.WithAuthorization(a.creds.auth()))

	if resp.StatusCode == http.StatusUnauthorized {
		if creds, ok := a.creds.(*OAuthCreds); ok {
			if err := creds.refresh(a.refType); err != nil {
				return nil, err
			}
			return a.get(endpoint, params, out)
		}
		return nil, fmt.Errorf("error calling api. response code: %v", resp.StatusCode)
	}
	if err != nil {
		return nil, err
	}
	return resp, nil
}

type objects []map[string]interface{}

func (o objects) Unmarshal(out interface{}) error {
	b, err := json.Marshal(o)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}