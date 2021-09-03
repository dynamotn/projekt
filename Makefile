BINARY_FOLDER = bin

.PHONY: pj
pj:
	@mkdir -p $(BINARY_FOLDER)
	go build -o $(BINARY_FOLDER)/pj cmd/pj/main.go

.PHONY: project
project:
	@mkdir -p $(BINARY_FOLDER)
	go build -o $(BINARY_FOLDER)/project cmd/project/main.go

.PHONY: clean
clean:
	@rm -rf $(BINARY_FOLDER)

.PHONY: install
install: pj project
	@mv $(BINARY_FOLDER)/* /usr/local/bin

.PHONY: all
all: pj project install clean
