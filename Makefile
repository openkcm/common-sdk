.PHONY: test
test:
	go test -race -coverprofile cover.out ./...
	# On a Mac, you can use the following command to open the coverage report in the browser
	# go tool cover -html=cover.out -o cover.html && open cover.html

.PHONY: lint
lint:
	golangci-lint run ./...
