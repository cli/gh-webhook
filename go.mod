module github.com/cli/gh-webhook

go 1.19

require (
	github.com/MakeNowJust/heredoc v1.0.0
	github.com/spf13/cobra v1.5.0
)

require (
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

require github.com/gorilla/websocket v1.5.0

replace golang.org/x/crypto => github.com/cli/crypto v0.0.0-20210929142629-6be313f59b03
