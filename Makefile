BINARY_FOLDER = bin

.PHONY: pj
pj:
	@mkdir -p $(BINARY_FOLDER)
	go build -o $(BINARY_FOLDER)/pj cmd/pj/main.go

.PHONY: project
project:
	@mkdir -p $(BINARY_FOLDER)
	go build -o $(BINARY_FOLDER)/project cmd/project/main.go

.PHONY: all
all: pj project

.PHONY: clean
clean:
	@rm -rf $(BINARY_FOLDER)
