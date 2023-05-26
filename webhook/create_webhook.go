package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/auth"
)

type createHookRequest struct {
	Name   string     `json:"name"`
	Events []string   `json:"events"`
	Active bool       `json:"active"`
	Config hookConfig `json:"config"`
}

type hookConfig struct {
	ContentType string `json:"content_type"`
	InsecureSSL string `json:"insecure_ssl"`
	URL         string `json:"url"`
	Secret      string `json:"secret,omitempty"`
}

type createHookResponse struct {
	Active bool       `json:"active"`
	Config hookConfig `json:"config"`
	Events []string   `json:"events"`
	ID     int        `json:"id"`
	Name   string     `json:"name"`
	URL    string     `json:"url"`
	WsURL  string     `json:"ws_url"`
}

type hookOptions struct {
	gitHubHost string
	authToken  string
	eventTypes []string
	repo       string
	org        string
	secret     string
}

// createHook issues a request against the GitHub API to create a dev webhook
func createHook(o *hookOptions) (string, func() error, error) {
	apiClient, err := api.NewRESTClient(api.ClientOptions{
		Host:      o.gitHubHost,
		AuthToken: o.authToken,
	})
	if err != nil {
		return "", nil, fmt.Errorf("error creating REST client: %w", err)
	}
	path := fmt.Sprintf("repos/%s/hooks", o.repo)
	if o.org != "" {
		path = fmt.Sprintf("orgs/%s/hooks", o.org)
	}

	req := createHookRequest{
		Name:   "cli",
		Events: o.eventTypes,
		Active: false,
		Config: hookConfig{
			ContentType: "json",
			InsecureSSL: "0",
			Secret:      o.secret,
		},
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return "", nil, err
	}
	var res createHookResponse
	err = apiClient.Post(path, bytes.NewReader(reqBytes), &res)
	if err != nil {
		var apierr *api.HTTPError
		if errors.As(err, &apierr) && apierr.StatusCode == http.StatusForbidden {
			return "", nil, fmt.Errorf("you do not have access to this feature")
		}
		return "", nil, fmt.Errorf("error creating webhook: %w", err)
	}

	return res.WsURL, func() error {
		err := apiClient.Patch(res.URL, strings.NewReader(`{"active": true}`), nil)
		if err != nil {
			return fmt.Errorf("error activating webhook: %w", err)
		}
		return nil
	}, nil
}

func authTokenForHost(host string) (string, error) {
	token, _ := auth.TokenForHost(host)
	if token == "" {
		return "", fmt.Errorf("gh auth token not found for host %q", host)
	}
	return token, nil
}
