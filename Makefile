test: *.go
	go test -v .

testrace: *.go
	go test -v -race

testcover: *.go
	go test -v -cover .

lint: *.go
	gofmt -l -e -d .
	go vet .
	golint -set_exit_status .

testci: lint testrace
