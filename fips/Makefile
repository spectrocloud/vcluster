# FIPS 140-3 image builds for the vcluster image set (PPD-1597 / PPD-1612).
#
# Every binary here is compiled with the Go Cryptographic Module v1.0.0
# (GOFIPS140=v1.0.0, NIST CMVP cert #4985) and runs with GODEBUG=fips140=on.
#
#   make build-all                 # build every image
#   make build-coredns             # build one
#   make push-all                  # push every image
#   make scan-all                  # bulwark crit/high scan
#   make verify-all                # assert the binaries are GOFIPS140 builds
#   make print-images              # list the fully-qualified tags
#
# Auth for push: the registry is served by the gcloud credential helper (see the
# docker-auth target). Pushes need an account holding
# artifactregistry.repositories.uploadArtifacts on the target registry - ask
# DevOps for the push service account rather than hardcoding one here.

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# --- registry / tagging -----------------------------------------------------
# The defaults reproduce exactly the references fips/vcluster.yaml consumes, so
# `make build-all push-all` with no overrides produces the shipping tags.
#
# For a personal or dev build, set IMAGE_PREFIX to flatten every image under one
# scratch namespace instead of the product paths:
#
#   make build-all push-all \
#     REGISTRY=us-east1-docker.pkg.dev \
#     IMAGE_PREFIX=spectro-images/dev/$USER \
#     TAG_SUFFIX=fips140-3-dev
REGISTRY     ?= us-docker.pkg.dev
IMAGE_PREFIX ?=
PLATFORM     ?= linux/amd64
TAG_SUFFIX   ?= fips140-3

# --- component versions -----------------------------------------------------
# coredns: CVE floor, do not drop below 1.14.6 (see Dockerfile.coredns).
COREDNS_VERSION ?= 1.14.6
# hostpath-mapper: upstream's latest tag; v0.2.7 == main HEAD.
HPM_VERSION     ?= 0.2.7
# upsync plugin: keep in sync with the image in fips/vcluster.yaml.
UPSYNC_VERSION  ?= v0.0.9
SYNCER_VERSION  ?= 0.27.3

# --- build inputs -----------------------------------------------------------
BUILDER_GOLANG_VERSION ?= 1.26.5
# Leave BASE_IMAGE empty to take each Dockerfile's default. coredns and the
# hostpath-mapper fall back to gcr.io/distroless/static when it is unset.
BASE_IMAGE ?=

FIPS_DIR   := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
REPO_ROOT  := $(abspath $(FIPS_DIR)/..)
# Dockerfile.upsync builds from the plugin repo (it COPYs the whole context),
# so that checkout - not this repo - is the build context.
UPSYNC_SRC_DIR ?= $(abspath $(REPO_ROOT)/../vcluster-container-resource-upsync-plugin)

# --- image references -------------------------------------------------------
# Per-image repository paths, matching fips/vcluster.yaml. Setting IMAGE_PREFIX
# overrides all of them and puts every image under $(IMAGE_PREFIX)/<name>.
repo_path     = $(if $(IMAGE_PREFIX),$(IMAGE_PREFIX)/$(2),$(1))
COREDNS_REPO := $(call repo_path,palette-images-fips/third-party/vcluster/coredns,coredns)
HPM_REPO     := $(call repo_path,palette-images-fips/third-party/loft/vcluster-hpm,vcluster-hpm)
UPSYNC_REPO  := $(call repo_path,palette-images-fips/palette/vcluster-upsync-plugin,vcluster-upsync-plugin)
SYNCER_REPO  := $(call repo_path,palette-images-fips/third-party/vcluster-oss,vcluster-oss)

COREDNS_IMAGE := $(REGISTRY)/$(COREDNS_REPO):v$(COREDNS_VERSION)-$(TAG_SUFFIX)
HPM_IMAGE     := $(REGISTRY)/$(HPM_REPO):$(HPM_VERSION)-$(TAG_SUFFIX)
UPSYNC_IMAGE  := $(REGISTRY)/$(UPSYNC_REPO):$(UPSYNC_VERSION)-$(TAG_SUFFIX)
# The syncer ships under the plain upstream version tag, no suffix.
SYNCER_IMAGE  := $(REGISTRY)/$(SYNCER_REPO):$(SYNCER_VERSION)

ALL_IMAGES := $(COREDNS_IMAGE) $(HPM_IMAGE) $(UPSYNC_IMAGE) $(SYNCER_IMAGE)

DOCKER        ?= docker
DOCKER_BUILD  := $(DOCKER) build --platform $(PLATFORM)
BASE_IMAGE_ARG = $(if $(BASE_IMAGE),--build-arg BASE_IMAGE=$(BASE_IMAGE),)

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: print-images
print-images: ## Print the fully-qualified image references
	@printf '%s\n' $(ALL_IMAGES)

# --- build ------------------------------------------------------------------
.PHONY: build-all
build-all: build-coredns build-hpm build-upsync build-syncer ## Build every FIPS image

.PHONY: build-coredns
build-coredns: ## Build the CoreDNS FIPS image
	$(DOCKER_BUILD) -f $(FIPS_DIR)/Dockerfile.coredns \
	  --build-arg BUILDER_GOLANG_VERSION=$(BUILDER_GOLANG_VERSION) \
	  --build-arg VERSION=$(COREDNS_VERSION) \
	  $(BASE_IMAGE_ARG) \
	  -t $(COREDNS_IMAGE) $(REPO_ROOT)

.PHONY: build-hpm
build-hpm: ## Build the vcluster-hostpath-mapper FIPS image
	$(DOCKER_BUILD) -f $(FIPS_DIR)/Dockerfile.hpm \
	  --build-arg BUILDER_GOLANG_VERSION=$(BUILDER_GOLANG_VERSION) \
	  --build-arg VERSION=$(HPM_VERSION) \
	  $(BASE_IMAGE_ARG) \
	  -t $(HPM_IMAGE) $(REPO_ROOT)

.PHONY: build-upsync
build-upsync: ## Build the upsync plugin FIPS image (context: the plugin repo)
	@test -d "$(UPSYNC_SRC_DIR)" || { \
	  echo "error: UPSYNC_SRC_DIR not found: $(UPSYNC_SRC_DIR)"; \
	  echo "       pass UPSYNC_SRC_DIR=/path/to/vcluster-container-resource-upsync-plugin"; \
	  exit 1; }
	$(DOCKER_BUILD) -f $(FIPS_DIR)/Dockerfile.upsync \
	  --build-arg BUILDER_GOLANG_VERSION=$(BUILDER_GOLANG_VERSION) \
	  --build-arg CRYPTO_LIB=fips \
	  -t $(UPSYNC_IMAGE) $(UPSYNC_SRC_DIR)

.PHONY: build-syncer
build-syncer: ## Build the vcluster syncer FIPS image
	$(DOCKER_BUILD) -f $(FIPS_DIR)/Dockerfile.syncfips \
	  --build-arg BUILDER_GOLANG_VERSION=$(BUILDER_GOLANG_VERSION) \
	  --build-arg BUILD_VERSION=$(SYNCER_VERSION) \
	  -t $(SYNCER_IMAGE) $(REPO_ROOT)

# --- push -------------------------------------------------------------------
.PHONY: push-all
push-all: push-coredns push-hpm push-upsync push-syncer ## Push every FIPS image

.PHONY: push-coredns push-hpm push-upsync push-syncer
push-coredns: ## Push the CoreDNS image
	$(DOCKER) push $(COREDNS_IMAGE)
push-hpm: ## Push the hostpath-mapper image
	$(DOCKER) push $(HPM_IMAGE)
push-upsync: ## Push the upsync plugin image
	$(DOCKER) push $(UPSYNC_IMAGE)
push-syncer: ## Push the syncer image
	$(DOCKER) push $(SYNCER_IMAGE)

.PHONY: docker-auth
docker-auth: ## Configure the gcloud credential helper for $(REGISTRY)
	gcloud auth configure-docker $(REGISTRY) --quiet

# --- scan / verify ----------------------------------------------------------
BULWARK  ?= bulwark
SEVERITY ?= critical,high

.PHONY: scan-all
scan-all: ## bulwark crit/high scan of every image
	@set -euo pipefail; \
	for img in $(ALL_IMAGES); do \
	  echo "=== $$img ==="; \
	  $(BULWARK) image --severity $(SEVERITY) $$img; \
	done

# image:binary pairs to check. The syncer carries kine and helm too - both are
# GOFIPS140 carrier builds and are checked alongside the syncer binary.
VERIFY_TARGETS := \
  $(COREDNS_IMAGE):/coredns \
  $(HPM_IMAGE):/vcluster-hpm \
  $(UPSYNC_IMAGE):/plugin/plugin \
  $(SYNCER_IMAGE):/vcluster \
  $(SYNCER_IMAGE):/usr/local/bin/kine \
  $(SYNCER_IMAGE):/usr/local/bin/helm

.PHONY: verify-all
verify-all: ## Assert each shipped binary is a GOFIPS140 build (works on distroless)
	@set -euo pipefail; \
	rc=0; tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
	for spec in $(VERIFY_TARGETS); do \
	  img=$${spec%:*}; bin=$${spec##*:}; \
	  if ! cid=$$($(DOCKER) create --platform $(PLATFORM) "$$img"); then \
	    echo "FAIL  $$img (docker create failed)"; rc=1; continue; fi; \
	  cp_rc=0; \
	  $(DOCKER) cp "$$cid:$$bin" "$$tmp/b" >/dev/null 2>&1 || cp_rc=$$?; \
	  $(DOCKER) rm -f "$$cid" >/dev/null 2>&1 || true; \
	  if [ "$$cp_rc" -ne 0 ]; then echo "FAIL  $$img $$bin (not found)"; rc=1; continue; fi; \
	  info=$$(go version -m "$$tmp/b" 2>/dev/null) || info=''; \
	  fips=$$(printf '%s\n' "$$info" | grep -o 'GOFIPS140=[^[:space:]]*' | head -1 || true); \
	  gover=$$(printf '%s\n' "$$info" | head -1 | awk '{print $$2}'); \
	  if [ -n "$$fips" ]; then echo "ok    $$img $$bin ($$gover, $$fips)"; \
	  else echo "FAIL  $$img $$bin ($$gover, no GOFIPS140)"; rc=1; fi; \
	  rm -f "$$tmp/b"; \
	done; \
	exit $$rc

.PHONY: clean
clean: ## Remove the locally built image tags
	-$(DOCKER) rmi $(ALL_IMAGES) 2>/dev/null || true
