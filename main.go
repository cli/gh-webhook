package main

import (
	"os"

	"github.com/cli/gh-webhook/webhook"
)

func main() {
	if err := webhook.NewCmdForward().Execute(); err != nil {
		os.Exit(1)
	}
}
