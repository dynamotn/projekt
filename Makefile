BINARY_FOLDER = bin

.PHONY: pj
pj:
	@mkdir -p $(BINARY_FOLDER)
	go build -o $(BINARY_FOLDER)/pj cmd/pj/main.go

.PHONY: projekt
projekt:
	@mkdir -p $(BINARY_FOLDER)
	go build -o $(BINARY_FOLDER)/projekt cmd/projekt/main.go

.PHONY: clean
clean:
	@rm -rf $(BINARY_FOLDER)

.PHONY: install
install: pj projekt
	@mv $(BINARY_FOLDER)/* /usr/local/bin

.PHONY: all
all: install clean
