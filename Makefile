#!/usr/bin/make

TAGS := omit_load_extension foreign_keys icu json1 fts5 secure_delete
BUILD_TAGS := assets $(TAGS)

SITECONFIG_GIT=https://github.com/j0k3r/graby-site-config.git
SITECONFIG=graby-site-config

# Build the server
.PHONY: build
build:
	go generate
	go build -tags "$(BUILD_TAGS)" -ldflags="-s -w" -o dist/readeck

# Build the server in dev mode, without compiling the assets
.PHONY: build-dev
build-dev:
	go build -tags "$(TAGS)" -o dist/readeck

# Clean the build
.PHONY: clean
clean:
	rm -rf dist
	rm -f pkg/assets/assets_vfsdata.go
	rm -f pkg/extract/fftr/siteconfig_vfsdata.go
	go clean

# Launch the documentation
.PHONY: doc
doc:
	@echo "Visit http://localhost:6060/pkg/github.com/readeck/readeck/"
	godoc

# Lint code
.PHONY: lint
lint:
	golint ./...

# Vet
.PHONY: vet
vet:
	go vet -tags "$(TAGS)" -ldflags="-s -w" ./...

# Launch tests
.PHONY: test
test:
	go test -v -cover ./...

# Start the HTTP server
.PHONY: serve
serve:
	modd -f modd.conf

# Update site-config folder with definitions from
# graby git repository
.PHONY: update-site-config
update-site-config:
	git clone $(SITECONFIG_GIT) $(SITECONFIG)

	rm -rf site-config/standard
	go run tools/fftr_convert.go $(SITECONFIG) site-config/standard
	rm -rf $(SITECONFIG)
