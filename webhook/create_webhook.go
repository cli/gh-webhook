package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"

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
func createHook(o *hookOptions) (string, error) {
	apiClient, err := gh.RESTClient(&api.ClientOptions{
		Host: o.Host,
	})
	if err != nil {
		return "", fmt.Errorf("error creating rest client: %w", err)
	}
	path := fmt.Sprintf("repos/%s/hooks", o.Repo)
	req := createHookRequest{
		Name:   "cli",
		Events: o.EventTypes,
		Active: true,
		Config: hookConfig{
			ContentType: "json",
			InsecureSSL: "0",
		},
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	var res createHookResponse
	err = apiClient.Post(path, bytes.NewReader(reqBytes), &res)
	if err != nil {
		return "", fmt.Errorf("error creating webhook: %w", err)
	}
	return res.WsURL, nil
}
