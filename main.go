package main

import (
	"os"

	"github.com/github/gh-webhook/webhook"
)

func main() {
	if err := webhook.NewCmdForward(nil).Execute(); err != nil {
		os.Exit(1)
	}
}
