APP=kobo-highlights

.PHONY: build vet clean

build: ## Build binary
	go build -o $(APP) .

vet: ## Run go vet
	go vet ./...

clean: ## Remove built binary
	rm -f $(APP)
