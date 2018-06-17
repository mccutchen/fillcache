test: lint
	go test -v -race -cover $(GO_TEST_ARGS) .

lint: *.go
	gofmt -l -e -d .
	go vet .
	golint -set_exit_status .
