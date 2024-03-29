#
# Copyright 2022 DataPunch Organization
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
GO_VERSION := $(shell go version | awk '{print substr($$3, 3, 10)}')
MOD_VERSION := $(shell awk '/^go/ {print $$2}' go.mod)

GM := $(word 1,$(subst ., ,$(GO_VERSION)))
MM := $(word 1,$(subst ., ,$(MOD_VERSION)))
FAIL := $(shell if [ $(GM) -lt $(MM) ]; then echo MAJOR; fi)
ifdef FAIL
$(error Build should be run with at least go $(MOD_VERSION) or later, found $(GO_VERSION))
endif
GM := $(word 2,$(subst ., ,$(GO_VERSION)))
MM := $(word 2,$(subst ., ,$(MOD_VERSION)))
FAIL := $(shell if [ $(GM) -lt $(MM) ]; then echo MINOR; fi)
ifdef FAIL
$(error Build should be run with at least go $(MOD_VERSION) or later, found $(GO_VERSION))
endif

BASE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# Force Go modules
GO111MODULE := on
export GO111MODULE

all:
	$(MAKE) -C $(dir $(BASE_DIR)) clean test build

# Format files
.PHONY: format
format:
	@echo "formatting files ..."
	go fmt ./...

# Build binaries
.PHONY: build
build:
	@echo "building punch commands ..."
	go build -o dist/punch ./cmd/punch
	@echo "building rdsdatabase plugin ..."
	go build -buildmode=plugin -o dist/plugin/rdsdatabase.so ./pkg/topologyimpl/rdsdatabase/

# Build sparkcli binary
.PHONY: sparkcli
sparkcli:
	@echo "fetching and building sparkcli ..."
	mkdir -p dist/temp
	rm -rf dist/temp/spark-on-k8s-operator
	git clone -b master-datapunch https://github.com/datapunchorg/spark-on-k8s-operator.git dist/temp/spark-on-k8s-operator
	cd dist/temp/spark-on-k8s-operator && go build -o ../../sparkcli sparkcli/main.go
	rm -rf dist/temp

# Run the tests after building
.PHONY: test
test:
	@echo "running unit tests ..."
	go test ./...

# Generate release
.PHONY: release
release: build sparkcli
	@echo "generating release ..."
	mkdir -p dist
	mkdir -p dist/script
	mkdir -p dist/examples
	mkdir -p dist/third-party/helm-charts
	cp -R third-party/helm-charts/* dist/third-party/helm-charts/
	cp hack/pyspark-example.py dist/examples/
	cp hack/pyspark-iceberg-example.py dist/examples/
	cp hack/pyspark-hive-example.py dist/examples/
	cp script/* dist/script/
	zip -r dist.zip dist

# Clean up
.PHONY: clean
clean:
	@echo "cleaning test caches and release ..."
	go clean -cache -testcache -r -x ./... 2>&1 >/dev/null
	-rm -rf dist dist.zip