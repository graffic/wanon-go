.PHONY: test tools

tools:
	go install gotest.tools/gotestsum@latest
	go install github.com/jackc/tern/v2@latest

test: tools
	# golangci-lint run
	gotestsum  -- --count=1 -coverprofile=coverage.out -coverpkg=github.com/graffic/wanon-go/... ./...
	go tool cover -func=coverage.out