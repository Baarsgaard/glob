glob: main.go index.html
	@go fmt ./...
	@go build -o glob

.PHONY: run
run:
	@DEBUG=1 go run ./...
