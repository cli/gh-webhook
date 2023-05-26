module github.com/cli/gh-webhook

go 1.19

require (
	github.com/MakeNowJust/heredoc v1.0.0
	github.com/cli/go-gh v0.1.1-0.20220913125123-04019861008e
	github.com/spf13/cobra v1.5.0
)

require (
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/testify v1.7.5 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
)

require (
	github.com/gorilla/websocket v1.5.0
	github.com/kr/text v0.2.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace golang.org/x/crypto => github.com/cli/crypto v0.0.0-20210929142629-6be313f59b03
