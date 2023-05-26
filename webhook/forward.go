package webhook

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

const (
	webhookForwarderProdURL = "wss://webhook-forwarder.github.com"
)

type hookOptions struct {
	Out              io.Writer
	ErrOut           io.Writer
	WebhookForwarder string
	EventTypes       []string
	Repo             string
	Org              string
	Secret           string
}

// NewCmdForward returns a forward command.
func NewCmdForward(runF func(*hookOptions) error) *cobra.Command {
	var githubHostname string
	var localURL string
	opts := &hookOptions{
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	cmd := &cobra.Command{
		Use:   "forward --events=<types> [--url=<url>]",
		Short: "Receive test events locally",
		Example: heredoc.Doc(`
			# create a dev webhook for the 'issue_open' event in the monalisa/smile repo in GitHub running locally, and
			# forward payloads for the triggered event to http://localhost:9999/webhooks

			$ gh webhook forward --events=issues --repo=monalisa/smile --url="http://localhost:9999/webhooks"
			$ gh webhook forward --events=issues --org=github --url="http://localhost:9999/webhooks"
		`),
		RunE: func(*cobra.Command, []string) error {
			if opts.Repo == "" && opts.Org == "" {
				return errors.New("`--repo` or `--org` flag required")
			}

			if localURL == "" {
				fmt.Fprintln(opts.ErrOut, "note: no `--url` specified; printing webhook payloads to stdout")
			}

			if runF != nil {
				return runF(opts)
			}

			token, err := authTokenForHost(githubHostname)
			if err != nil {
				return fmt.Errorf("fatal: error fetching gh token: %w", err)
			} else if token == "" {
				return errors.New("fatal: you must be authenticated with gh to run this command")
			}

			if opts.WebhookForwarder == "" {
				opts.WebhookForwarder = webhookForwarderProdURL
			}
			wsURL := strings.TrimSuffix(opts.WebhookForwarder, "/") + "/forward"
			chp := createHookParams{
				Events: opts.EventTypes,
				Repo:   opts.Repo,
				Org:    opts.Org,
				Secret: opts.Secret,
			}
			for i := 0; i < 3; i++ {
				if err = runFwd(opts.Out, localURL, token, wsURL, chp); err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						return nil
					}
				}
			}
			return err
		},
	}
	cmd.Flags().StringSliceVarP(&opts.EventTypes, "events", "E", nil, "Names of the event `types` to forward. Use `*` to forward all events.")
	cmd.MarkFlagRequired("events")
	cmd.Flags().StringVarP(&opts.Repo, "repo", "R", "", "Name of the repo where the webhook is installed")
	cmd.Flags().StringVarP(&githubHostname, "github-host", "H", "github.com", "GitHub host name")
	cmd.Flags().StringVarP(&localURL, "url", "U", "", "Address of the local server to receive events. If omitted, events will be printed to stdout.")
	cmd.Flags().StringVarP(&opts.Org, "org", "O", "", "Name of the org where the webhook is installed")
	cmd.Flags().StringVarP(&opts.Secret, "secret", "S", "", "Webhook secret for incoming events")
	return cmd
}

type wsEventReceived struct {
	Header http.Header
	Body   []byte
}

func runFwd(out io.Writer, url, token, wsURL string, chp createHookParams) error {
	for i := 0; i < 3; i++ {
		err := handleWebsocket(out, url, token, wsURL, chp)
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

type createHookParams struct {
	Events []string
	Repo   string
	Org    string
	Secret string
}

type eventBody struct{ Type, Message string }

// handleWebsocket mediates between websocket server and local web server
func handleWebsocket(out io.Writer, url, token, wsURL string, chp createHookParams) error {
	c, err := dial(token, wsURL)
	if err != nil {
		return fmt.Errorf("error dialing to ws server: %w", err)
	}
	defer c.Close()

	err = c.WriteJSON(chp)
	if err != nil {
		return fmt.Errorf("error sending create hook params: %w", err)
	}

	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	var eb eventBody
	err = c.ReadJSON(&eb)
	if err != nil {
		return fmt.Errorf("error reading create hook response: %w", err)
	}
	if eb.Type == "error" {
		return fmt.Errorf("error creating hook: %s", eb.Message)
	}
	c.SetReadDeadline(time.Time{})

	fmt.Fprintf(out, "Forwarding Webhook events from GitHub...\n")

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

func authTokenForHost(host string) (string, error) {
	ghExe := os.Getenv("GH_PATH")
	if ghExe == "" {
		var err error
		ghExe, err = exec.LookPath("gh")
		if err != nil {
			return "", err
		}
	}
	cmd := exec.Command(ghExe, "auth", "token", "--secure-storage", "--hostname", host)
	result, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(result)), nil
}
