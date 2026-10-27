# Image builds for the vcluster image set (PPD-1597 / PPD-1612), v0.27 line.
#
# The syncer and the upsync plugin are built in two variants from the same
# Dockerfiles, selected with FIPS=true|false (default true):
#
#   FIPS=true   compiled with the Go Cryptographic Module v1.0.0
#               (GOFIPS140=v1.0.0, NIST CMVP cert #4985), runs with
#               GODEBUG=fips140=on, pushed under palette-images-fips/...
#   FIPS=false  regular build against the standard Go crypto library,
#               pushed under palette-images/...
#
# Spectro CI (self-service-backend proxy-build-pipeline) entry points - syncer
# + upsync; the hostpath-mapper and coredns are not part of them (see below):
#
#   make build-fips push-fips          # FIPS variant
#   make build-nonfips push-nonfips    # non-FIPS variant
#
# Single images / other targets:
#
#   make build-all                 # build every image (current FIPS=; coredns is always FIPS)
#   make build-coredns             # build one
#   make push-all                  # push every image
#   make scan-all                  # bulwark crit/high scan
#   make verify-all                # assert the binaries match the current FIPS= variant
#   make print-images              # list the fully-qualified tags
#
# The hostpath-mapper (upstream v0.2.x) requires vcluster v0.29+, so it cannot
# be rebuilt against this branch's source the way the v0.36 line does; its
# 0.2.x tags are produced by the v0.36 pipeline and are left alone here.
#
# Auth for push: the registry is served by the gcloud credential helper (see the
# docker-auth target). Pushes need an account holding
# artifactregistry.repositories.uploadArtifacts on the target registry - ask
# DevOps for the push service account rather than hardcoding one here.

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# --- variant ----------------------------------------------------------------
FIPS ?= true
ifeq ($(filter $(FIPS),true false),)
$(error FIPS must be true or false, got '$(FIPS)')
endif
FIPS_ON    := $(filter true,$(FIPS))
CRYPTO_LIB := $(if $(FIPS_ON),fips,)
VARIANT    := $(if $(FIPS_ON),fips,nonfips)

# --- registry / tagging -----------------------------------------------------
# The defaults reproduce the shipping references (fips/vcluster.yaml for the
# FIPS variant), so the build-/push- targets with no overrides produce the
# shipping tags.
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
# Top-level product project for each variant.
FIPS_PROJECT    ?= palette-images-fips
NONFIPS_PROJECT ?= palette-images
PROJECT         := $(if $(FIPS_ON),$(FIPS_PROJECT),$(NONFIPS_PROJECT))

# Spectro CI: SPECTRO_VERSION is overridden by the pipeline from the branch
# (spectro-release-4.8 → 4.8.<patch>). TAG is what `make version` / image-tags /
# create-git-tag / marker MAJOR_VERSION consume. Image refs below stay on
# SYNCER_VERSION / UPSYNC_VERSION / HPM_VERSION unless you opt into Option B.

SPECTRO_VERSION ?= 0.0.0-dev
TAG             ?= v$(SYNCER_VERSION)-spectro-$(SPECTRO_VERSION)

# --- component versions -----------------------------------------------------
# coredns: CVE floor, do not drop below 1.14.6 (see Dockerfile.coredns).
COREDNS_VERSION ?= 1.14.6
# hostpath-mapper: upstream's latest tag; v0.2.7 == main HEAD.
HPM_VERSION     ?= 0.2.7
# upsync plugin: keep in sync with the image in fips/vcluster.yaml.
UPSYNC_VERSION  ?= v0.0.9
SYNCER_VERSION  ?= 0.27.3

# --- build inputs -----------------------------------------------------------
BUILDER_GOLANG_VERSION ?= 1.26.10
# Spectro third-party aggregate tag that provides the syncer's helm and kine,
# and which build of each to take: vertex = GOFIPS140, palette = regular.
THIRDPARTY_VERSION     ?= 4.9
THIRDPARTY_FLAVOR      := $(if $(FIPS_ON),vertex,palette)
# Leave BASE_IMAGE empty to take each Dockerfile's default. coredns and the
# hostpath-mapper fall back to gcr.io/distroless/static when it is unset.
BASE_IMAGE ?=

FIPS_DIR   := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
REPO_ROOT  := $(abspath $(FIPS_DIR)/..)
BUILD_DIR  := $(FIPS_DIR)/.build

# Dockerfile.upsync builds from the plugin repo (it COPYs the whole context).
# By default the plugin is cloned fresh at UPSYNC_GIT_REF so the build is
# reproducible from this repo alone (CI has no sibling checkout); set
# UPSYNC_SRC_DIR to build a local checkout instead (the ref is then ignored).
# main is the plugin line built against vcluster v0.27 (v0.0.9).
UPSYNC_GIT_URL ?= https://github.com/spectrocloud-labs/vcluster-container-resource-upsync-plugin.git
UPSYNC_GIT_REF ?= main
UPSYNC_SRC_DIR ?=
# Cloned per variant so FIPS and non-FIPS builds can run concurrently.
UPSYNC_CONTEXT := $(if $(UPSYNC_SRC_DIR),$(abspath $(UPSYNC_SRC_DIR)),$(BUILD_DIR)/upsync-src-$(VARIANT))

# Which vcluster code the upsync plugin is compiled against:
#   local    (default) this repo's source, via a go.mod replace - so the plugin
#            picks up this branch's dependency versions (CVE fixes)
#   upstream the github.com/loft-sh/vcluster version pinned in its go.mod
# Needs BuildKit (named build contexts); Docker >= 23 uses it by default.
VCLUSTER_SRC ?= local
ifeq ($(filter $(VCLUSTER_SRC),local upstream),)
$(error VCLUSTER_SRC must be local or upstream, got '$(VCLUSTER_SRC)')
endif
EMPTY_CONTEXT    := $(BUILD_DIR)/empty
VCLUSTER_CONTEXT := $(if $(filter local,$(VCLUSTER_SRC)),$(REPO_ROOT),$(EMPTY_CONTEXT))
VCLUSTER_SRC_ARG  = --build-context vcluster-src=$(VCLUSTER_CONTEXT)

# --- image references -------------------------------------------------------
# Per-image repository paths, matching fips/vcluster.yaml for the FIPS variant.
# Setting IMAGE_PREFIX overrides all of them and puts every image under
# $(IMAGE_PREFIX)/<name>. coredns is FIPS-only.
repo_path     = $(if $(IMAGE_PREFIX),$(IMAGE_PREFIX)/$(2),$(1))
COREDNS_REPO := $(call repo_path,$(FIPS_PROJECT)/third-party/vcluster/coredns,coredns)
HPM_REPO     := $(call repo_path,$(PROJECT)/third-party/loft/vcluster-hpm,vcluster-hpm)
UPSYNC_REPO  := $(call repo_path,$(PROJECT)/palette/vcluster-upsync-plugin,vcluster-upsync-plugin)
SYNCER_REPO  := $(call repo_path,$(PROJECT)/third-party/vcluster-oss,vcluster-oss)

FIPS_TAG := $(if $(FIPS_ON),-$(TAG_SUFFIX),)
COREDNS_IMAGE := $(REGISTRY)/$(COREDNS_REPO):v$(COREDNS_VERSION)-$(TAG_SUFFIX)
HPM_IMAGE     := $(REGISTRY)/$(HPM_REPO):$(HPM_VERSION)$(FIPS_TAG)
UPSYNC_IMAGE  := $(REGISTRY)/$(UPSYNC_REPO):$(UPSYNC_VERSION)$(FIPS_TAG)
# The syncer ships under the plain upstream version tag in each variant's own
# project; it only takes the suffix when sharing a repository under IMAGE_PREFIX.
SYNCER_IMAGE  := $(REGISTRY)/$(SYNCER_REPO):$(SYNCER_VERSION)$(if $(IMAGE_PREFIX),$(FIPS_TAG),)

# What the CI entry points build and push.
CI_IMAGES  := $(SYNCER_IMAGE) $(UPSYNC_IMAGE)
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

# --- CI entry points (syncer + upsync) ---------------------------------------
.PHONY: build-fips build-nonfips push-fips push-nonfips
CI_TARGETS := syncer upsync
build-fips: ## Build syncer + upsync, FIPS variant
	$(MAKE) $(addprefix build-,$(CI_TARGETS)) FIPS=true
build-nonfips: ## Build syncer + upsync, non-FIPS variant
	$(MAKE) $(addprefix build-,$(CI_TARGETS)) FIPS=false
push-fips: ## Push syncer + upsync, FIPS variant
	$(MAKE) $(addprefix push-,$(CI_TARGETS)) FIPS=true
push-nonfips: ## Push syncer + upsync, non-FIPS variant
	$(MAKE) $(addprefix push-,$(CI_TARGETS)) FIPS=false

# --- build ------------------------------------------------------------------
.PHONY: build-all
build-all: build-coredns build-hpm build-upsync build-syncer ## Build every image (current FIPS=; coredns is always FIPS)

.PHONY: build-coredns
build-coredns: ## Build the CoreDNS FIPS image
	$(DOCKER_BUILD) -f $(FIPS_DIR)/Dockerfile.coredns \
	  --build-arg BUILDER_GOLANG_VERSION=$(BUILDER_GOLANG_VERSION) \
	  --build-arg VERSION=$(COREDNS_VERSION) \
	  $(BASE_IMAGE_ARG) \
	  -t $(COREDNS_IMAGE) $(REPO_ROOT)

.PHONY: build-hpm
build-hpm: ## Build the vcluster-hostpath-mapper image (FIPS=true|false)
	$(DOCKER_BUILD) -f $(FIPS_DIR)/Dockerfile.hpm \
	  --build-arg BUILDER_GOLANG_VERSION=$(BUILDER_GOLANG_VERSION) \
	  --build-arg VERSION=$(HPM_VERSION) \
	  --build-arg CRYPTO_LIB=$(CRYPTO_LIB) \
	  $(BASE_IMAGE_ARG) \
	  -t $(HPM_IMAGE) $(REPO_ROOT)

.PHONY: build-upsync
build-upsync: upsync-src vcluster-src ## Build the upsync plugin image (FIPS=true|false; context: the plugin repo)
	$(DOCKER_BUILD) -f $(FIPS_DIR)/Dockerfile.upsync \
	  --build-arg BUILDER_GOLANG_VERSION=$(BUILDER_GOLANG_VERSION) \
	  $(VCLUSTER_SRC_ARG) \
	  --build-arg CRYPTO_LIB=$(CRYPTO_LIB) \
	  -t $(UPSYNC_IMAGE) $(UPSYNC_CONTEXT)

.PHONY: upsync-src
upsync-src: ## Fetch the upsync plugin source at UPSYNC_GIT_REF (skipped with UPSYNC_SRC_DIR)
ifeq ($(UPSYNC_SRC_DIR),)
	@rm -rf "$(UPSYNC_CONTEXT)" && mkdir -p "$(BUILD_DIR)"
	git -c advice.detachedHead=false clone --depth=1 -b $(UPSYNC_GIT_REF) $(UPSYNC_GIT_URL) "$(UPSYNC_CONTEXT)"
else
	@test -d "$(UPSYNC_CONTEXT)" || { echo "error: UPSYNC_SRC_DIR not found: $(UPSYNC_CONTEXT)"; exit 1; }
	@echo "using local upsync checkout: $(UPSYNC_CONTEXT)"
endif

# Provides the empty build context used by VCLUSTER_SRC=upstream.
.PHONY: vcluster-src
vcluster-src:
	@mkdir -p "$(EMPTY_CONTEXT)"
	@echo "upsync vcluster source: $(VCLUSTER_SRC) ($(VCLUSTER_CONTEXT))"

.PHONY: build-syncer
build-syncer: ## Build the vcluster syncer image (FIPS=true|false)
	$(DOCKER_BUILD) -f $(FIPS_DIR)/Dockerfile.syncfips \
	  --build-arg BUILDER_GOLANG_VERSION=$(BUILDER_GOLANG_VERSION) \
	  --build-arg BUILDER_3RDPARTY_VERSION=$(THIRDPARTY_VERSION) \
	  --build-arg THIRDPARTY_FLAVOR=$(THIRDPARTY_FLAVOR) \
	  --build-arg BUILD_VERSION=$(SYNCER_VERSION) \
	  --build-arg CRYPTO_LIB=$(CRYPTO_LIB) \
	  -t $(SYNCER_IMAGE) $(REPO_ROOT)

# --- push -------------------------------------------------------------------
.PHONY: push-all
push-all: push-coredns push-hpm push-upsync push-syncer ## Push every image (current FIPS=)

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

# image:binary pairs to check. The syncer carries kine and helm too - taken
# from the third-party aggregate in the matching flavor, so they are checked
# alongside the syncer binary. coredns is FIPS-only.
VERIFY_TARGETS := \
  $(if $(FIPS_ON),$(COREDNS_IMAGE):/coredns) \
  $(HPM_IMAGE):/vcluster-hpm \
  $(UPSYNC_IMAGE):/plugin/plugin \
  $(SYNCER_IMAGE):/vcluster \
  $(SYNCER_IMAGE):/usr/local/bin/kine \
  $(SYNCER_IMAGE):/usr/local/bin/helm

.PHONY: verify-all
verify-all: ## Assert each binary matches the current FIPS= variant (works on distroless)
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
	  if [ -n "$(FIPS_ON)" ] && [ -n "$$fips" ]; then echo "ok    $$img $$bin ($$gover, $$fips)"; \
	  elif [ -z "$(FIPS_ON)" ] && [ -z "$$fips" ]; then echo "ok    $$img $$bin ($$gover, no GOFIPS140)"; \
	  else echo "FAIL  $$img $$bin ($$gover, $${fips:-no GOFIPS140}, want $(VARIANT))"; rc=1; fi; \
	  rm -f "$$tmp/b"; \
	done; \
	exit $$rc

.PHONY: clean
clean: ## Remove the locally built image tags (current FIPS=) and the fetched sources
	-$(DOCKER) rmi $(ALL_IMAGES) 2>/dev/null || true
	rm -rf "$(BUILD_DIR)"
