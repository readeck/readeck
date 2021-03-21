#!/usr/bin/make
VERSION := $(shell git describe --tags)
DATE := $(shell git log -1 --format=%cI)

TAGS := omit_load_extension foreign_keys json1 fts5 secure_delete
BUILD_TAGS := $(TAGS)
VERSION_FLAGS := \
	-X 'codeberg.org/readeck/readeck/configs.version=$(VERSION)' \
	-X 'codeberg.org/readeck/readeck/configs.buildTimeStr=$(DATE)'

SITECONFIG_REPO=https://github.com/j0k3r/graby-site-config.git
SITECONFIG_CLONE=graby-site-config
SITECONFIG_DEST=pkg/extract/fftr/site-config/standard

# Build the app
.PHONY: all
all: web-build build build-pg

# Build the server
.PHONY: build
build:
	go build \
		-tags "$(BUILD_TAGS)" \
		-ldflags="$(VERSION_FLAGS) -s -w" \
		-o dist/readeck

# Build the server with only PG support (full static)
.PHONY: build-pg
build-pg:
	go build \
		-tags "$(BUILD_TAGS) without_sqlite" \
		-ldflags="$(VERSION_FLAGS) -s -w" \
		-o dist/readeck_pg

# Build the server in dev mode, without compiling the assets
.PHONY: build-dev
build-dev:
	go build -tags "$(TAGS)" -o dist/readeck


# Clean the build
.PHONY: clean
clean:
	rm -rf dist
	rm -rf assets/www/*
	rm -f  assets/templates/base.gohtml
	go clean

list:
	go list \
		-tags "$(BUILD_TAGS)" \
		-ldflags="$(VERSION_FLAGS) -s -w" \
		-f "{{ .GoFiles }}"

# Launch the documentation
.PHONY: doc
doc:
	@echo "Visit http://localhost:6060/pkg/codeberg.org/readeck/readeck/?m=all"
	godoc

# Linting
.PHONY: lint
lint:
	golangci-lint run

# SLOC
.PHONY: sloc
sloc:
	scc -i go,js,sass

# Launch tests
.PHONY: test
test:
	go test -tags "$(TAGS)" -cover ./...

# Start the HTTP server
.PHONY: serve
serve:
	modd -f modd.conf

# Update site-config folder with definitions from
# graby git repository
.PHONY: update-site-config
update-site-config:
	git clone $(SITECONFIG_REPO) $(SITECONFIG_CLONE)

	rm -rf $(SITECONFIG_DEST)
	go run tools/fftr_convert.go $(SITECONFIG_CLONE) $(SITECONFIG_DEST)
	rm -rf $(SITECONFIG_CLONE)

.PHONY: dev
dev:
	${MAKE} -j2 web-watch serve

.PHONY: web-build
web-build:
	@$(MAKE) -C web build

.PHONY: web-watch
web-watch:
	@$(MAKE) -C web watch
