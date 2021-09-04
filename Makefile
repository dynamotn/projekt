BINARY_FOLDER = bin
DOC_FOLDER = doc

.PHONY: pj
pj:
	@mkdir -p $(BINARY_FOLDER)
	go build -o $(BINARY_FOLDER)/pj cmd/pj/main.go

.PHONY: t
t:
	@mkdir -p $(BINARY_FOLDER)
	go build -o $(BINARY_FOLDER)/t cmd/t/main.go

.PHONY: b
b:
	@mkdir -p $(BINARY_FOLDER)
	go build -o $(BINARY_FOLDER)/b cmd/b/main.go

.PHONY: projekt
projekt:
	@mkdir -p $(BINARY_FOLDER)
	go build -o $(BINARY_FOLDER)/projekt cmd/projekt/main.go

.PHONY: clean
clean:
	@rm -rf $(BINARY_FOLDER)

.PHONY: install
install: pj t b projekt
	@mv $(BINARY_FOLDER)/* /usr/local/bin

.PHONY: doc
doc:
	@rm -rf ${DOC_FOLDER}
	@mkdir -p ${DOC_FOLDER}
	go run doc.go

.PHONY: all
all: install clean doc
