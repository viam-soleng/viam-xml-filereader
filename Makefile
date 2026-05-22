GO_BUILD_ENV := CGO_ENABLED=0
GO_BUILD_FLAGS :=
MODULE_BINARY := bin/viam-xml-filereader

# Target architectures supported by this module (must match meta.json "build.arch").
ARCHES := linux-amd64 linux-arm64 darwin-arm64 windows-amd64
PER_ARCH_TARBALLS := $(addsuffix /module.tar.gz,$(addprefix bin/,$(ARCHES)))

ifeq ($(VIAM_TARGET_OS), windows)
	GO_BUILD_ENV += GOOS=windows GOARCH=amd64
	GO_BUILD_FLAGS := -tags no_cgo
	MODULE_BINARY = bin/viam-xml-filereader.exe
endif

# Single-arch host/dev build (e.g. `make bin/viam-xml-filereader`).
$(MODULE_BINARY): Makefile go.mod *.go cmd/module/*.go
	GOOS=$(VIAM_BUILD_OS) GOARCH=$(VIAM_BUILD_ARCH) $(GO_BUILD_ENV) go build $(GO_BUILD_FLAGS) -o $(MODULE_BINARY) cmd/module/main.go

lint:
	gofmt -s -w .

update:
	go get go.viam.com/rdk@latest
	go mod tidy

test:
	go test ./...

# `make module.tar.gz` iterates every architecture in $(ARCHES) and produces one
# tarball per arch at bin/<goos>-<goarch>/module.tar.gz. Each tarball contains
# `bin/viam-xml-filereader[.exe]` and `meta.json` at the top level, matching the
# entrypoint path declared in meta.json.
.PHONY: module.tar.gz
module.tar.gz: $(PER_ARCH_TARBALLS)

bin/%/module.tar.gz: Makefile go.mod *.go cmd/module/*.go meta.json
	@set -e; os=$$(echo $* | cut -d- -f1); arch=$$(echo $* | cut -d- -f2); \
	  workdir=bin/$*; bin=viam-xml-filereader; ext=""; tags=""; \
	  if [ "$$os" = "windows" ]; then ext=".exe"; tags="-tags no_cgo"; fi; \
	  host_os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	  host_arch=$$(uname -m); \
	  case $$host_arch in x86_64) host_arch=amd64;; aarch64) host_arch=arm64;; esac; \
	  mkdir -p $$workdir/bin; \
	  echo ">> building $$os/$$arch -> $$workdir/bin/$$bin$$ext"; \
	  GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build $$tags -o $$workdir/bin/$$bin$$ext cmd/module/main.go; \
	  if [ "$$os" = "linux" ] && [ "$$host_os" = "linux" ] && [ "$$arch" = "$$host_arch" ]; then \
	    strip $$workdir/bin/$$bin; \
	  fi; \
	  tar -czf $$workdir/module.tar.gz -C $$workdir bin/$$bin$$ext -C $(CURDIR) meta.json

module: test module.tar.gz

all: test module.tar.gz

setup:
	go mod tidy

clean:
	rm -rf bin

VERSION ?= 0.0.1

# Override the version with e.g. `make upload VERSION=1.2.3`.
upload:
	@echo viam module upload --version \"$(VERSION)\" --platform \"linux/amd64\" bin/linux-amd64/module.tar.gz
	@echo viam module upload --version \"$(VERSION)\" --platform \"linux/arm64\" bin/linux-arm64/module.tar.gz
	@echo viam module upload --version \"$(VERSION)\" --platform \"darwin/arm64\" bin/darwin-arm64/module.tar.gz
	@echo viam module upload --version \"$(VERSION)\" --platform \"windows/amd64\" bin/windows-amd64/module.tar.gz

