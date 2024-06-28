include ./Makefile.Common

BUILD_DIR ?= $(SRC_ROOT)/build
OS := $(shell uname | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)

CHECKS = generate lint test tidy fmt

# set ARCH var based on output
ifeq ($(ARCH),x86_64)
	ARCH = amd64
endif
ifeq ($(ARCH),aarch64)
	ARCH = arm64
endif

# .PHONY: all
# # all: install-tools
# all: build

.DEFAULT_GOAL := build

.PHONY: build
build: install-tools
	GOOS=$(OS) GOARCH=$(ARCH) go build -o $(BUILD_DIR)/dora-the-explorer

# TODO: fix this release through goreleaser. Goreleaser installed through tools.go
# is the OSS version and doesn't support the `partial:` option in the
# .goreleaser.yaml. This option is needed for CI builds but isn't available locally.
.PHONY: grbuild
grbuild:
	$(GORELEASER) build --clean --snapshot

.PHONY: dockerbuild
dockerbuild:
	$(MAKE) build OS=linux ARCH=$(ARCH)
	docker build . -t liatrio/tag-o11y-dora-the-explorer:localdev --build-arg BIN_PATH="./build/dora-the-explorer" --platform linux/$(ARCH)

# Setting the paralellism to 1 to improve output readability. Reevaluate later as needed for performance
.PHONY: checks
checks: install-tools
	$(MAKE) -j 1 $(CHECKS)
	@if [ -n "$$(git diff --name-only)" ]; then \
		echo "Some files have changed. Please commit them."; \
		exit 1; \
	else \
		echo "completed successfully."; \
	fi
