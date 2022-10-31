package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cli/go-gh"
	"github.com/cli/go-gh/pkg/api"
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

// createHook issues a request against the GitHub API to create a dev webhook
func createHook(o *hookOptions) (string, func() error, error) {
	apiClient, err := gh.RESTClient(&api.ClientOptions{
		Host: o.GitHubHost,
	})
	if err != nil {
		return "", nil, fmt.Errorf("error creating rest client: %w", err)
	}
	path := fmt.Sprintf("repos/%s/hooks", o.Repo)
	if o.Org != "" {
		path = fmt.Sprintf("orgs/%s/hooks", o.Org)
	}

	req := createHookRequest{
		Name:   "cli",
		Events: o.EventTypes,
		Active: false,
		Config: hookConfig{
			ContentType: "json",
			InsecureSSL: "0",
			Secret:      o.Secret,
		},
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return "", nil, err
	}
	var res createHookResponse
	err = apiClient.Post(path, bytes.NewReader(reqBytes), &res)
	if err != nil {
		var apierr api.HTTPError
		if errors.As(err, &apierr) && apierr.StatusCode == http.StatusForbidden {
			return "", nil, fmt.Errorf("you do not have access to this feature")
		}
		return "", nil, fmt.Errorf("error creating webhook: %w", err)
	}

	// reset path for activation.
	path += "/" + strconv.Itoa(res.ID)

	return res.WsURL, func() error {
		err = apiClient.Patch(path, strings.NewReader(`{"active": true}`), nil)
		if err != nil {
			return fmt.Errorf("error activating webhook: %w", err)
		}
		return nil
	}, nil
}
