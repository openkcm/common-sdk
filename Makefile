.PHONY: test
test:
	go test -race -coverprofile cover.out ./...
	# On a Mac, you can use the following command to open the coverage report in the browser
	# go tool cover -html=cover.out -o cover.html && open cover.html

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: reuse-lint
reuse-lint:
	docker run --rm --volume $(PWD):/data fsfe/reuse lint
