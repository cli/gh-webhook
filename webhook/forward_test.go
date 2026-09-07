package webhook

import (
	"fmt"
	"testing"

	"github.com/gorilla/websocket"
)

func TestNewCmdForwardSilencesUsage(t *testing.T) {
	cmd := NewCmdForward()

	if !cmd.SilenceUsage {
		t.Fatal("expected runtime errors not to print command usage")
	}
}

func TestIsWebsocketCloseErrorUnwrapsError(t *testing.T) {
	err := fmt.Errorf(
		"error receiving json event: %w",
		&websocket.CloseError{Code: websocket.CloseAbnormalClosure},
	)

	if !isWebsocketCloseError(err, websocket.CloseAbnormalClosure) {
		t.Fatal("expected wrapped abnormal closure to be recognized")
	}
}

func TestIsWebsocketCloseErrorRejectsDifferentCode(t *testing.T) {
	err := fmt.Errorf(
		"error receiving json event: %w",
		&websocket.CloseError{Code: websocket.CloseNormalClosure},
	)

	if isWebsocketCloseError(err, websocket.CloseAbnormalClosure) {
		t.Fatal("expected close error with a different code not to match")
	}
}
