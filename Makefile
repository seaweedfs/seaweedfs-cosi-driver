# Copyright 2023 SUSE, LLC.
# Copyright 2024 s3gw maintainers.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

REGISTRY_NAME ?= quay.io/s3gw
IMAGE_NAME ?= s3gw-cosi-driver
IMAGE_TAG ?= latest

GO ?= go
ENGINE ?= docker

GOFLAGS ?= -trimpath
LDFLAGS ?= -s -w -extldflags "-static"
GO_SETTINGS += CGO_ENABLED=0

.PHONY: all
all: build container push

.PHONY: build
build:
	$(GO_SETTINGS) $(GO) build \
		$(GOFLAGS) \
		-ldflags="$(LDFLAGS)" \
		-o=./bin/s3gw-cosi-driver \
		./cmd/s3gw-cosi-driver

.PHONY: test
test:
	$(GO_SETTINGS) $(GO) test $(GOFLAGS) \
		-race \
		-cover -covermode=atomic -coverprofile=coverage.out \
		./...

.PHONY: container
container:
	$(ENGINE) build \
		--tag=$(IMAGE_NAME):$(IMAGE_TAG) \
		--file=Dockerfile \
		.

.PHONY: push
push:
	$(ENGINE) tag \
		$(IMAGE_NAME):$(IMAGE_TAG)
		$(REGISTRY_NAME)/$(IMAGE_NAME):$(IMAGE_TAG)
	$(ENGINE) push \
		$(REGISTRY_NAME)/$(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: clean
clean:
	-rm -rf bin
