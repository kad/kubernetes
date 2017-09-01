# Copyright 2016 The Kubernetes Authors.
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

# Build the CNI binaries.
#
# Usage:
#	[ARCH=amd64] [CNI_RELEASE=v0.3.0] make (build|push)

ARCH?=amd64
CNI_RELEASE?=v0.6.0
CNI_TARBALL=cni-$(ARCH)-$(CNI_RELEASE).tar.gz
CUR_DIR=$(shell pwd)
OUTPUT_DIR=$(CUR_DIR)/output
GOLANG_VERSION=1.8.3

all: build

build:
	mkdir -p $(OUTPUT_DIR)

	docker run -it -e http_proxy=$(http_proxy) -e https_proxy=$(https_proxy) -v $(OUTPUT_DIR):/output:Z golang:$(GOLANG_VERSION) /bin/bash -c "\
    mkdir -p build/bin \
    && cd build \
	&& git clone https://github.com/containernetworking/cni\
	&& cd cni \
	&& git checkout $(CNI_RELEASE) \
    && ln -s ../bin bin \
	&& CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) ./build.sh \
    && cd .. \
    && git clone https://github.com/containernetworking/plugins\
    && cd plugins \
	&& git checkout $(CNI_RELEASE) \
    && ln -s ../bin bin \
	&& CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) ./build.sh \
    && cd .. \
	&& tar -zcvf $(CNI_TARBALL) bin/ \
	&& mv $(CNI_TARBALL) /output/"

# Backward Compatibility
ifeq ($(ARCH),amd64)
	cp $(OUTPUT_DIR)/$(CNI_TARBALL) $(OUTPUT_DIR)/cni-$(CNI_RELEASE).tar.gz
endif

push: build
	gsutil cp $(OUTPUT_DIR)/$(CNI_TARBALL) gs://kubernetes-release/network-plugins

ifeq ($(ARCH),amd64)
	gsutil cp $(OUTPUT_DIR)/cni-$(CNI_RELEASE).tar.gz gs://kubernetes-release/network-plugins
endif

clean:
	rm -rf output/

.PHONY: all
