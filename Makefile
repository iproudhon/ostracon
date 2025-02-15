PACKAGES=$(shell go list ./...)
SRCPATH=$(shell pwd)
OUTPUT?=build/ostracon

INCLUDE = -I=${GOPATH}/src/github.com/line/ostracon -I=${GOPATH}/src -I=${GOPATH}/src/github.com/gogo/protobuf/protobuf
BUILD_TAGS ?= ostracon
VERSION := $(shell git describe --always)
ifeq ($(LIBSODIUM), 1)
  BUILD_TAGS += libsodium
  LIBSODIUM_TARGET = libsodium
else
  BUILD_TAGS += r2ishiguro
  LIBSODIUM_TARGET =
endif
LD_FLAGS = -X github.com/line/ostracon/version.OCCoreSemVer=$(VERSION)
BUILD_FLAGS = -mod=readonly -ldflags "$(LD_FLAGS)"
HTTPS_GIT := https://github.com/line/ostracon.git
DOCKER_BUF := docker run -v $(shell pwd):/workspace --workdir /workspace bufbuild/buf
CGO_ENABLED ?= 0
TARGET_OS ?= $(shell go env GOOS)
TARGET_ARCH ?= $(shell go env GOARCH)

# handle nostrip
ifeq (,$(findstring nostrip,$(OSTRACON_BUILD_OPTIONS)))
  BUILD_FLAGS += -trimpath
  LD_FLAGS += -s -w
endif

# handle race
ifeq (race,$(findstring race,$(OSTRACON_BUILD_OPTIONS)))
  BUILD_FLAGS += -race
endif

# handle cleveldb
ifeq (cleveldb,$(findstring cleveldb,$(OSTRACON_BUILD_OPTIONS)))
  BUILD_TAGS += cleveldb
endif

# handle badgerdb
ifeq (badgerdb,$(findstring badgerdb,$(OSTRACON_BUILD_OPTIONS)))
  BUILD_TAGS += badgerdb
endif

# handle rocksdb
ifeq (rocksdb,$(findstring rocksdb,$(OSTRACON_BUILD_OPTIONS)))
  CGO_ENABLED=1
  BUILD_TAGS += rocksdb
endif

# handle boltdb
ifeq (boltdb,$(findstring boltdb,$(OSTRACON_BUILD_OPTIONS)))
  BUILD_TAGS += boltdb
endif

# allow users to pass additional flags via the conventional LDFLAGS variable
LD_FLAGS += $(LDFLAGS)

all: build test install
.PHONY: all

include tests.mk

###############################################################################
###                                Build Ostracon                           ###
###############################################################################

build: $(LIBSODIUM_TARGET)
	CGO_ENABLED=1 go build $(BUILD_FLAGS) -tags "$(BUILD_TAGS)" -o $(OUTPUT) ./cmd/ostracon/
.PHONY: build

install: $(LIBSODIUM_TARGET)
	CGO_ENABLED=1 go install $(BUILD_FLAGS) -tags "$(BUILD_TAGS)" ./cmd/ostracon
.PHONY: install

###############################################################################
###                                 Mockery                                 ###
###############################################################################

###
# https://github.com/vektra/mockery
# Should install
### brew
# brew install mockery
# brew upgrade mockery
### go get
# go get github.com/vektra/mockery/v2/.../

mock-gen:
	go generate ./...
.PHONY: mock

###############################################################################
###                                Protobuf                                 ###
###############################################################################

###
# https://github.com/protocolbuffers/protobuf
# https://developers.google.com/protocol-buffers/docs/gotutorial
# Should install
### go install
# go install google.golang.org/protobuf/cmd/protoc-gen-go
### Docker for Protocol Buffer
# https://hub.docker.com/r/bufbuild/buf

proto-all: proto-gen proto-lint proto-check-breaking
.PHONY: proto-all

proto-gen:
	@docker pull -q tendermintdev/docker-build-proto
	@echo "Generating Protobuf files"
	@docker run --rm -v $(shell pwd):/workspace --workdir /workspace tendermintdev/docker-build-proto sh ./scripts/protocgen.sh
.PHONY: proto-gen

proto-lint:
	@$(DOCKER_BUF) lint --error-format=json
.PHONY: proto-lint

proto-format:
	@echo "Formatting Protobuf files"
	docker run --rm -v $(shell pwd):/workspace --workdir /workspace tendermintdev/docker-build-proto find ./ -not -path "./third_party/*" -name *.proto -exec clang-format -i {} \;
.PHONY: proto-format

proto-check-breaking:
	@$(DOCKER_BUF) breaking --against .git#branch=main
.PHONY: proto-check-breaking

proto-check-breaking-ci:
	@$(DOCKER_BUF) breaking --against $(HTTPS_GIT)#branch=main
.PHONY: proto-check-breaking-ci

###############################################################################
###                              Build ABCI                                 ###
###############################################################################

build_abci:
	@go build -mod=readonly -i ./abci/cmd/...
.PHONY: build_abci

install_abci:
	@go install -mod=readonly ./abci/cmd/...
.PHONY: install_abci

###############################################################################
###                              Distribution                               ###
###############################################################################

########################################
### libsodium

VRF_ROOT = $(SRCPATH)/crypto/vrf/internal/vrf
LIBSODIUM_ROOT = $(VRF_ROOT)/libsodium
LIBSODIUM_OS = $(VRF_ROOT)/sodium/$(TARGET_OS)_$(TARGET_ARCH)
ifneq ($(TARGET_HOST), "")
LIBSODIUM_HOST = "--host=$(TARGET_HOST)"
endif

libsodium:
	@if [ ! -f $(LIBSODIUM_OS)/lib/libsodium.a ]; then \
		rm -rf $(LIBSODIUM_ROOT) && \
		mkdir $(LIBSODIUM_ROOT) && \
		git submodule update --init --recursive && \
		cd $(LIBSODIUM_ROOT) && \
		./autogen.sh && \
		./configure --disable-shared --prefix="$(LIBSODIUM_OS)" $(LIBSODIUM_HOST) &&	\
		$(MAKE) && \
		$(MAKE) install; \
	fi
.PHONY: libsodium

########################################
### Distribution

# dist builds binaries for all platforms and packages them for distribution
# TODO add abci to these scripts
dist:
	@BUILD_TAGS=$(BUILD_TAGS) sh -c "'$(CURDIR)/scripts/dist.sh'"
.PHONY: dist

go-mod-cache: go.sum
	@echo "--> Download go modules to local cache"
	@go mod download
.PHONY: go-mod-cache

go.sum: go.mod
	@echo "--> Ensure dependencies have not been modified"
	@go mod verify
	@go mod tidy

draw_deps:
	@# requires brew install graphviz or apt-get install graphviz
	go get github.com/RobotsAndPencils/goviz
	@goviz -i github.com/line/ostracon/cmd/ostracon -d 3 | dot -Tpng -o dependency-graph.png
.PHONY: draw_deps

get_deps_bin_size:
	@# Copy of build recipe with additional flags to perform binary size analysis
	$(eval $(shell go build -work -a $(BUILD_FLAGS) -tags $(BUILD_TAGS) -o $(OUTPUT) ./cmd/ostracon/ 2>&1))
	@find $(WORK) -type f -name "*.a" | xargs -I{} du -hxs "{}" | sort -rh | sed -e s:${WORK}/::g > deps_bin_size.log
	@echo "Results can be found here: $(CURDIR)/deps_bin_size.log"
.PHONY: get_deps_bin_size

###############################################################################
###                                  Libs                                   ###
###############################################################################

# generates certificates for TLS testing in remotedb and RPC server
gen_certs: clean_certs
	certstrap init --common-name "blockchain.line.me" --passphrase ""
	certstrap request-cert --common-name "server" -ip "127.0.0.1" --passphrase ""
	certstrap sign "server" --CA "blockchain.line.me" --passphrase ""
	mv out/server.crt rpc/jsonrpc/server/test.crt
	mv out/server.key rpc/jsonrpc/server/test.key
	rm -rf out
.PHONY: gen_certs

# deletes generated certificates
clean_certs:
	rm -f rpc/jsonrpc/server/test.crt
	rm -f rpc/jsonrpc/server/test.key
.PHONY: clean_certs

###############################################################################
###                  Formatting, linting, and vetting                       ###
###############################################################################

format:
	find . -name '*.go' -type f -not -path "*.git*" -not -name '*.pb.go' -not -name '*pb_test.go' | xargs gofmt -w -s
	find . -name '*.go' -type f -not -path "*.git*"  -not -name '*.pb.go' -not -name '*pb_test.go' | xargs goimports -w -local github.com/line/ostracon
.PHONY: format

lint:
	@echo "--> Running linter"
	@golangci-lint run
.PHONY: lint

DESTINATION = ./index.html.md

###############################################################################
###                           Documentation                                 ###
###############################################################################

BRANCH := $(shell git branch --show-current)
BRANCH_URI := $(shell git branch --show-current | sed 's/[\#]/%23/g')
build-docs:
	@cd docs && \
	npm install && \
	VUEPRESS_BASE="/$(BRANCH_URI)/" npm run build && \
	mkdir -p ~/output/$(BRANCH) && \
	cp -r .vuepress/dist/* ~/output/$(BRANCH)/ && \
	for f in `find . -name '*.png' | grep -v '/node_modules/'`; do if [ ! -e `dirname $$f` ]; then mkdir `dirname $$f`; fi; cp $$f ~/output/$(BRANCH)/`dirname $$f`; done && \
	echo '<html><head><meta http-equiv="refresh" content="0;/$(BRANCH_URI)/index.html"/></head></html>' > ~/output/index.html
.PHONY: build-docs

sync-docs:
	cd ~/output && \
	echo "role_arn = ${DEPLOYMENT_ROLE_ARN}" >> /root/.aws/config ; \
	echo "CI job = ${CIRCLE_BUILD_URL}" >> version.html ; \
	aws s3 sync . s3://${WEBSITE_BUCKET} --profile terraform --delete ; \
	aws cloudfront create-invalidation --distribution-id ${CF_DISTRIBUTION_ID} --profile terraform --path "/*" ;
.PHONY: sync-docs

###############################################################################
###                            Docker image                                 ###
###############################################################################

# Build linux binary on other platforms
# Should run from within a linux if CGO_ENABLED=1
build-linux:
	GOOS=linux GOARCH=$(TARGET_ARCH) $(MAKE) build
.PHONY: build-linux

build-linux-docker:
	docker build --label=ostracon --tag="ostracon/ostracon" -f ./DOCKER/Dockerfile .
.PHONY: build-linux-docker

standalone-linux-docker:
	docker run -it --rm -v "/tmp:/ostracon" -p 26656:26656 -p 26657:26657 -p 26660:26660  ostracon/ostracon
.PHONY: standalone-linux-docker

# XXX Warning: Not test yet
# Runs `make build OSTRACON_BUILD_OPTIONS=cleveldb` from within an Amazon
# Linux (v2)-based Docker build container in order to build an Amazon
# Linux-compatible binary. Produces a compatible binary at ./build/ostracon
build_c-amazonlinux:
	$(MAKE) -C ./DOCKER build_amazonlinux_buildimage
	docker run --rm -it -v `pwd`:/ostracon ostracon/ostracon:build_c-amazonlinux
.PHONY: build_c-amazonlinux

###############################################################################
###                       Local testnet using docker                        ###
###############################################################################

DOCKER_HOME = /go/src/github.com/line/ostracon
DOCKER_CMD = docker run --rm \
                        -v `pwd`:$(DOCKER_HOME) \
                        -w $(DOCKER_HOME)
DOCKER_IMG = golang:1.15-alpine
BUILD_CMD = apk add --update --no-cache git make gcc libc-dev build-base curl jq bash file gmp-dev clang libtool autoconf automake \
	&& cd $(DOCKER_HOME) \
	&& LIBSODIUM=$(LIBSODIUM) make build-linux

# Login docker-container for confirmation building linux binary
build-shell:
	$(DOCKER_CMD) -it --entrypoint '' ${DOCKER_IMG} /bin/sh
.PHONY: build-shell

build-localnode:
	$(DOCKER_CMD) ${DOCKER_IMG} /bin/sh -c "$(BUILD_CMD)"
.PHONY: build-localnode

build-localnode-docker: build-localnode
	@cd networks/local && make
.PHONY: build-localnode-docker

# Run a 4-node testnet locally
localnet-start: localnet-stop build-localnode-docker
	@if ! [ -f build/node0/config/genesis.json ]; then docker run --rm -v $(CURDIR)/build:/ostracon:Z ostracon/localnode testnet --config /etc/ostracon/config-template.toml --o . --starting-ip-address 192.167.10.2; fi
	docker-compose up
.PHONY: localnet-start

# Stop testnet
localnet-stop:
	docker-compose down
.PHONY: localnet-stop
