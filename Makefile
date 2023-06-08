# Copyright 2023 SUSE, LLC.
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

all: build container push

.PHONY: build container clean

ARCH := $(if $(GOARCH),$(GOARCH),$(shell go env GOARCH))

build:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o ./bin/s3gw-cosi-driver ./cmd/*

test:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go test ./cmd/*

container:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) -f Dockerfile .
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(REGISTRY_NAME)/$(IMAGE_NAME):$(IMAGE_TAG)

push:
	docker push $(REGISTRY_NAME)/$(IMAGE_NAME):$(IMAGE_TAG)

clean:
	-rm -rf bin
