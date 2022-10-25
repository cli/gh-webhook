package webhook

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/go-gh/pkg/auth"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

const gitHubAPIProdURL = "api.github.com"

type hookOptions struct {
	Out        io.Writer
	GitHubHost string
	EventTypes []string
	Repo       string
	Org        string
	URL        string
	Secret     string
}

// NewCmdForward returns a forward command.
func NewCmdForward(runF func(*hookOptions) error) *cobra.Command {
	opts := &hookOptions{
		Out: os.Stdout,
	}
	cmd := &cobra.Command{
		Use:   "forward --events=<event_types> --repo|org=<repo|org> [--url=\"<url>\"] [--github-host=<github-host>]",
		Short: "Receive test events on a server running locally",
		Long: heredoc.Doc(`To output event payloads to stdout instead of sending to a server,
			omit the --url flag. If the --github-host flag is not specified, webhooks will be created against github.com`),
		Example: heredoc.Doc(`
			# create a dev webhook for the 'issue_open' event in the monalisa/smile repo in GitHub running locally, and
			# forward payloads for the triggered event to http://localhost:9999/webhooks

			$ gh webhooks forward --events=issues --repo=monalisa/smile --url="http://localhost:9999/webhooks"
			$ gh webhooks forward --events=issues --org=github --url="http://localhost:9999/webhooks"
		`),
		RunE: func(*cobra.Command, []string) error {
			if opts.EventTypes == nil {
				return cmdutil.FlagErrorf("`--events` flag required")
			}
			if opts.Repo == "" && opts.Org == "" {
				return cmdutil.FlagErrorf("`--repo` or `--org` flag required")
			}
			if opts.GitHubHost == "" {
				opts.GitHubHost = gitHubAPIProdURL
			}

			if opts.URL == "" {
				fmt.Fprintf(opts.Out, "No --url specified, printing webhook payloads to stdout.\n")
			}

			if runF != nil {
				return runF(opts)
			}

			token, _ := auth.TokenForHost(opts.GitHubHost)
			if token == "" {
				return fmt.Errorf("you must be authenticated to run this command")
			}

			wsURL, activate, err := createHook(opts)
			if err != nil {
				return err
			}

			for i := 0; i < 3; i++ {
				if err = runFwd(opts.Out, opts.URL, token, wsURL, activate); err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						return nil
					}
				}
			}
			return err
		},
	}
	cmd.Flags().StringSliceVarP(&opts.EventTypes, "events", "E", []string{}, "(required) Names of the event types to forward. Event types can be separated by commas (e.g. `issues,pull_request`) or passed as multiple arguments (e.g. `--events issues --events pull_request`.")
	cmd.Flags().StringVarP(&opts.Repo, "repo", "R", "", "Name of the repo where the webhook is installed")
	cmd.Flags().StringVarP(&opts.GitHubHost, "github-host", "H", "", "(optional) Host for the GitHub API, default: api.github.com")
	cmd.Flags().StringVarP(&opts.URL, "url", "U", "", "(optional) Local address where the server which will receive webhooks is running")
	cmd.Flags().StringVarP(&opts.Org, "org", "O", "", "Name of the org where the webhook is installed")
	cmd.Flags().StringVarP(&opts.Secret, "secret", "S", "", "(optional) webhook secret for incoming events")
	return cmd
}

type wsEventReceived struct {
	Header http.Header
	Body   []byte
}

func runFwd(out io.Writer, url, token, wsURL string, activateHook func() error) error {
	for i := 0; i < 3; i++ {
		err := handleWebsocket(out, url, token, wsURL, activateHook)
		if err != nil {
			// If the error is a server disconnect (1006), retry connecting
			if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
				time.Sleep(5 * time.Second)
				continue
			}
			return err
		}
	}
	return fmt.Errorf("unable to connect to webhooks server, forwarding stopped")
}

// handleWebsocket mediates between websocket server and local web server
func handleWebsocket(out io.Writer, url, token, wsURL string, activateHook func() error) error {
	c, err := dial(token, wsURL)
	if err != nil {
		return fmt.Errorf("error dialing to ws server: %w", err)
	}
	defer c.Close()

	fmt.Fprintf(out, "Forwarding Webhook events from GitHub...\n")
	err = activateHook()
	if err != nil {
		return fmt.Errorf("error activating hook: %w", err)
	}

	for {
		var ev wsEventReceived
		err := c.ReadJSON(&ev)
		if err != nil {
			return fmt.Errorf("error receiving json event: %w", err)
		}

		resp, err := forwardEvent(url, ev)
		if err != nil {
			fmt.Fprintf(out, "Error forwarding event: %v\n", err)
			continue
		}

		err = c.WriteJSON(resp)
		if err != nil {
			return fmt.Errorf("error writing json event: %w", err)
		}
	}
}

// dial connects to the websocket server
func dial(token, url string) (*websocket.Conn, error) {
	h := make(http.Header)
	h.Set("Authorization", token)
	c, resp, err := websocket.DefaultDialer.Dial(url, h)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			err = fmt.Errorf("code: %v - body: %s - err: %w", resp.StatusCode, body, err)
		}
		return nil, err
	}
	return c, nil
}

type httpEventForward struct {
	Status int
	Header http.Header
	Body   []byte
}

// forwardEvent forwards events to the server running on the local port specified by the user
func forwardEvent(url string, ev wsEventReceived) (*httpEventForward, error) {
	event := ev.Header.Get("X-GitHub-Event")
	event = strings.ReplaceAll(event, "\n", "")
	event = strings.ReplaceAll(event, "\r", "")
	log.Printf("[LOG] received the following event: %v \n", event)
	if url == "" {
		fmt.Printf("%s\n", ev.Body)
		return &httpEventForward{Status: 200, Header: make(http.Header), Body: []byte("OK")}, nil
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(ev.Body))
	if err != nil {
		return nil, err
	}

	for k := range ev.Header {
		req.Header.Set(k, ev.Header.Get(k))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &httpEventForward{
		Status: resp.StatusCode,
		Header: resp.Header,
		Body:   body,
	}, nil
}
