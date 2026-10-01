# Image builds for the vcluster image set (PPD-1597 / PPD-1612), v0.36 line.
#
# The syncer (vcluster-oss), the upsync plugin and the hostpath-mapper are
# built in two variants from the same Dockerfiles, selected with
# FIPS=true|false (default true):
#
#   FIPS=true   compiled with the Go Cryptographic Module v1.0.0
#               (GOFIPS140=v1.0.0, NIST CMVP cert #4985), runs with
#               GODEBUG=fips140=on, pushed under palette-images-fips/...
#   FIPS=false  regular build against the standard Go crypto library,
#               pushed under palette-images/...
#
# coredns is FIPS-only and is not part of the entry points below.
#
# Common entry points (syncer + upsync + hostpath-mapper):
#
#   make build-fips                # build all three images, FIPS variant
#   make build-nonfips             # build all three images, non-FIPS variant
#   make push-fips push-nonfips    # push them
#   make release                   # build + push both variants
#   make verify-fips verify-nonfips
#   make print-images-all          # list every tag the above produce
#
# Single image, single variant:
#
#   make build-syncer FIPS=false
#   make build-upsync push-upsync FIPS=true
#
# Toolchain / third-party images - bump the defaults below, or override per
# run, e.g. make release BUILDER_GOLANG_VERSION=1.26.6 THIRDPARTY_VERSION=4.11
#   BUILDER_GOLANG_VERSION  tag of $(BUILDER_REPO), <v>-alpine; every image
#   THIRDPARTY_VERSION      tag of $(THIRDPARTY_REPO); the syncer's helm and
#                           kine (each the aggregate's "latest", vertex flavor
#                           for FIPS, palette flavor for non-FIPS)
# or replace the whole reference with BUILDER_IMAGE= / THIRDPARTY_IMAGE=
#
# Requirements: Docker with BuildKit (Docker >= 23), git, make; Go only for the
# verify targets (they read the binaries' build info with `go version -m`).
#
# Auth: pulling the base images needs read access to palette-images and
# palette-images-dev (us-docker.pkg.dev, us-central1-docker.pkg.dev); pushing
# needs artifactregistry.repositories.uploadArtifacts on $(REGISTRY). `make
# docker-auth` configures the gcloud credential helper for all of them - ask
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
# FIPS variant), so `make release` with no overrides produces the shipping tags.
#
# For a personal or dev build, set IMAGE_PREFIX to put every image under one
# scratch namespace instead of the product paths:
#
#   make release \
#     REGISTRY=us-east1-docker.pkg.dev \
#     IMAGE_PREFIX=spectro-images/dev/$USER
#
# Both variants then share a repository, so under IMAGE_PREFIX every FIPS tag
# carries the -$(TAG_SUFFIX) suffix (the syncer included) to keep them apart.
REGISTRY     ?= us-docker.pkg.dev
IMAGE_PREFIX ?=
PLATFORM     ?= linux/amd64
TAG_SUFFIX   ?= fips140-3
# Top-level product project for each variant.
FIPS_PROJECT    ?= palette-images-fips
NONFIPS_PROJECT ?= palette-images
PROJECT         := $(if $(FIPS_ON),$(FIPS_PROJECT),$(NONFIPS_PROJECT))

# Spectro CI: SPECTRO_VERSION is overridden by the pipeline from the branch
# (spectro-release-4.10 → 4.10.<patch>). TAG is what `make version` / image-tags /
# create-git-tag / marker MAJOR_VERSION consume. Image refs below stay on
# SYNCER_VERSION / UPSYNC_VERSION / HPM_VERSION unless you opt into Option B.

SPECTRO_VERSION ?= 0.0.0-dev
TAG             ?= v$(SYNCER_VERSION)-spectro-$(SPECTRO_VERSION)

# --- component versions -----------------------------------------------------
SYNCER_VERSION  ?= 0.36.3
# upsync plugin: keep in sync with the image in fips/vcluster.yaml. v0.0.9 is
# the v0.27-compatible release; v0.36 needs the plugin rebuilt against vcluster
# v0.36 (config field ControlPlaneNamespace was renamed to HostNamespace).
UPSYNC_VERSION  ?= v0.0.10
# coredns: CVE floor, do not drop below 1.14.6 (see Dockerfile.coredns).
COREDNS_VERSION ?= 1.14.6
# hostpath-mapper: upstream's latest tag; v0.2.7 == main HEAD.
HPM_VERSION     ?= 0.2.7

# --- build inputs -----------------------------------------------------------
HARDENED_REGISTRY      ?= us-central1-docker.pkg.dev/palette-images-dev/hardened-images
BUILDER_REPO           ?= $(HARDENED_REGISTRY)/builder/golang
BUILDER_GOLANG_VERSION ?= 1.26
BUILDER_IMAGE          ?= $(BUILDER_REPO):$(BUILDER_GOLANG_VERSION)-alpine
THIRDPARTY_REPO        ?= $(HARDENED_REGISTRY)/third-party/spectro-third-party
THIRDPARTY_VERSION     ?= 4.10
THIRDPARTY_IMAGE       ?= $(THIRDPARTY_REPO):$(THIRDPARTY_VERSION)
# Which build of each third-party binary to take: vertex = GOFIPS140.
THIRDPARTY_FLAVOR      := $(if $(FIPS_ON),vertex,palette)
# Leave BASE_IMAGE empty to take each Dockerfile's default. coredns and the
# hostpath-mapper fall back to gcr.io/distroless/static when it is unset.
BASE_IMAGE ?=

FIPS_DIR  := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
REPO_ROOT := $(abspath $(FIPS_DIR)/..)
BUILD_DIR := $(FIPS_DIR)/.build

# Dockerfile.upsync builds from the plugin repo (it COPYs the whole context).
# By default the plugin is cloned fresh at UPSYNC_GIT_REF so the build is
# reproducible from this repo alone; set UPSYNC_SRC_DIR to build a local
# checkout instead (the ref is then ignored).
UPSYNC_GIT_URL ?= https://github.com/spectrocloud-labs/vcluster-container-resource-upsync-plugin.git
UPSYNC_GIT_REF ?= vcluster-v0.36
UPSYNC_SRC_DIR ?=
# Cloned per variant so FIPS and non-FIPS builds can run concurrently.
UPSYNC_CONTEXT := $(if $(UPSYNC_SRC_DIR),$(abspath $(UPSYNC_SRC_DIR)),$(BUILD_DIR)/upsync-src-$(VARIANT))

# Which vcluster code upsync and the hostpath-mapper are compiled against:
#   local    (default) this repo's source, via a go.mod replace - so both track
#            whatever vcluster branch they are built from
#   upstream the github.com/loft-sh/vcluster version pinned in their go.mod
# Needs BuildKit (named build contexts); Docker >= 23 uses it by default.
VCLUSTER_SRC ?= local
ifeq ($(filter $(VCLUSTER_SRC),local upstream),)
$(error VCLUSTER_SRC must be local or upstream, got '$(VCLUSTER_SRC)')
endif
EMPTY_CONTEXT    := $(BUILD_DIR)/empty
VCLUSTER_CONTEXT := $(if $(filter local,$(VCLUSTER_SRC)),$(REPO_ROOT),$(EMPTY_CONTEXT))
VCLUSTER_SRC_ARG  = --build-context vcluster-src=$(VCLUSTER_CONTEXT)

# --- image references -------------------------------------------------------
# Per-image repository paths. Setting IMAGE_PREFIX overrides all of them and
# puts every image under $(IMAGE_PREFIX)/<name>.
repo_path     = $(if $(IMAGE_PREFIX),$(IMAGE_PREFIX)/$(2),$(1))
SYNCER_REPO  := $(call repo_path,$(PROJECT)/third-party/vcluster-oss,vcluster-oss)
UPSYNC_REPO  := $(call repo_path,$(PROJECT)/palette/vcluster-upsync-plugin,vcluster-upsync-plugin)
COREDNS_REPO := $(call repo_path,$(FIPS_PROJECT)/third-party/vcluster/coredns,coredns)
HPM_REPO     := $(call repo_path,$(PROJECT)/third-party/loft/vcluster-hpm,vcluster-hpm)

FIPS_TAG := $(if $(FIPS_ON),-$(TAG_SUFFIX),)
# The FIPS syncer ships under the plain upstream version tag in its own
# project; it only takes the suffix when sharing a repository under IMAGE_PREFIX.
SYNCER_IMAGE  := $(REGISTRY)/$(SYNCER_REPO):$(SYNCER_VERSION)$(if $(IMAGE_PREFIX),$(FIPS_TAG),)
UPSYNC_IMAGE  := $(REGISTRY)/$(UPSYNC_REPO):$(UPSYNC_VERSION)$(FIPS_TAG)
COREDNS_IMAGE := $(REGISTRY)/$(COREDNS_REPO):v$(COREDNS_VERSION)-$(TAG_SUFFIX)
HPM_IMAGE     := $(REGISTRY)/$(HPM_REPO):$(HPM_VERSION)$(FIPS_TAG)

VCLUSTER_IMAGES := $(SYNCER_IMAGE) $(UPSYNC_IMAGE) $(HPM_IMAGE)
ALL_IMAGES      := $(VCLUSTER_IMAGES) $(COREDNS_IMAGE)

DOCKER        ?= docker
DOCKER_BUILD  := $(DOCKER) build --platform $(PLATFORM)
BASE_IMAGE_ARG = $(if $(BASE_IMAGE),--build-arg BASE_IMAGE=$(BASE_IMAGE),)

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: print-images
print-images: ## Print the syncer/upsync/hpm references for the current FIPS=
	@printf '%s\n' $(VCLUSTER_IMAGES)

.PHONY: print-images-all
.PHONY: print-versions
print-versions: ## Print the toolchain and component versions in effect
	@printf '%-24s %s\n' \
	  BUILDER_IMAGE $(BUILDER_IMAGE) \
	  THIRDPARTY_IMAGE $(THIRDPARTY_IMAGE) \
	  THIRDPARTY_FLAVOR "$(THIRDPARTY_FLAVOR) (FIPS=$(FIPS))" \
	  SYNCER_VERSION $(SYNCER_VERSION) \
	  UPSYNC_VERSION $(UPSYNC_VERSION) \
	  HPM_VERSION $(HPM_VERSION) \
	  COREDNS_VERSION $(COREDNS_VERSION)

print-images-all: ## Print the syncer/upsync/hpm references for both variants
	@$(MAKE) -s --no-print-directory print-images FIPS=true
	@$(MAKE) -s --no-print-directory print-images FIPS=false

# --- both variants (syncer + upsync + hpm) ----------------------------------
.PHONY: build-fips build-nonfips push-fips push-nonfips verify-fips verify-nonfips release
VCLUSTER_TARGETS := syncer upsync hpm
build-fips: ## Build syncer + upsync + hpm, FIPS variant
	$(MAKE) $(addprefix build-,$(VCLUSTER_TARGETS)) FIPS=true
build-nonfips: ## Build syncer + upsync + hpm, non-FIPS variant
	$(MAKE) $(addprefix build-,$(VCLUSTER_TARGETS)) FIPS=false
push-fips: ## Push syncer + upsync + hpm, FIPS variant
	$(MAKE) $(addprefix push-,$(VCLUSTER_TARGETS)) FIPS=true
push-nonfips: ## Push syncer + upsync + hpm, non-FIPS variant
	$(MAKE) $(addprefix push-,$(VCLUSTER_TARGETS)) FIPS=false
verify-fips: ## Verify syncer + upsync + hpm, FIPS variant
	$(MAKE) verify FIPS=true
verify-nonfips: ## Verify syncer + upsync + hpm, non-FIPS variant
	$(MAKE) verify FIPS=false
release: ## Build and push syncer + upsync + hpm, both variants (no coredns)
	$(MAKE) build-fips
	$(MAKE) build-nonfips
	$(MAKE) push-fips
	$(MAKE) push-nonfips

# --- build ------------------------------------------------------------------
.PHONY: build-syncer
build-syncer: ## Build the vcluster syncer image (FIPS=true|false)
	$(DOCKER_BUILD) -f $(FIPS_DIR)/Dockerfile.syncfips \
	  --build-arg BUILDER_IMAGE=$(BUILDER_IMAGE) \
	  --build-arg THIRDPARTY_IMAGE=$(THIRDPARTY_IMAGE) \
	  --build-arg THIRDPARTY_FLAVOR=$(THIRDPARTY_FLAVOR) \
	  --build-arg BUILD_VERSION=$(SYNCER_VERSION) \
	  --build-arg CRYPTO_LIB=$(CRYPTO_LIB) \
	  -t $(SYNCER_IMAGE) $(REPO_ROOT)

.PHONY: build-upsync
build-upsync: upsync-src vcluster-src ## Build the upsync plugin image (FIPS=true|false)
	$(DOCKER_BUILD) -f $(FIPS_DIR)/Dockerfile.upsync \
	  --build-arg BUILDER_IMAGE=$(BUILDER_IMAGE) \
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

# Also provides the empty build context used by images that fetch their own
# source (coredns, hpm) and by VCLUSTER_SRC=upstream.
.PHONY: vcluster-src
vcluster-src:
	@mkdir -p "$(EMPTY_CONTEXT)"
	@echo "upsync/hpm vcluster source: $(VCLUSTER_SRC) ($(VCLUSTER_CONTEXT))"

.PHONY: build-all
build-all: build-syncer build-upsync build-hpm build-coredns ## Build every image incl. coredns (current FIPS=; coredns is always FIPS)

.PHONY: build-coredns
build-coredns: vcluster-src ## Build the CoreDNS FIPS image
	$(DOCKER_BUILD) -f $(FIPS_DIR)/Dockerfile.coredns \
	  --build-arg BUILDER_IMAGE=$(BUILDER_IMAGE) \
	  --build-arg VERSION=$(COREDNS_VERSION) \
	  $(BASE_IMAGE_ARG) \
	  -t $(COREDNS_IMAGE) $(EMPTY_CONTEXT)

.PHONY: build-hpm
build-hpm: vcluster-src ## Build the vcluster-hostpath-mapper image (FIPS=true|false)
	$(DOCKER_BUILD) -f $(FIPS_DIR)/Dockerfile.hpm \
	  --build-arg BUILDER_IMAGE=$(BUILDER_IMAGE) \
	  $(VCLUSTER_SRC_ARG) \
	  --build-arg VERSION=$(HPM_VERSION) \
	  --build-arg CRYPTO_LIB=$(CRYPTO_LIB) \
	  $(BASE_IMAGE_ARG) \
	  -t $(HPM_IMAGE) $(EMPTY_CONTEXT)

# --- push -------------------------------------------------------------------
.PHONY: push-all
push-all: push-syncer push-upsync push-hpm push-coredns ## Push every image incl. coredns (current FIPS=)

.PHONY: push-syncer push-upsync push-coredns push-hpm
push-syncer: ## Push the syncer image (FIPS=true|false)
	$(DOCKER) push $(SYNCER_IMAGE)
push-upsync: ## Push the upsync plugin image (FIPS=true|false)
	$(DOCKER) push $(UPSYNC_IMAGE)
push-coredns: ## Push the CoreDNS image
	$(DOCKER) push $(COREDNS_IMAGE)
push-hpm: ## Push the hostpath-mapper image (FIPS=true|false)
	$(DOCKER) push $(HPM_IMAGE)

.PHONY: docker-auth
# Registries the Dockerfiles pull base images from.
BASE_REGISTRIES := us-docker.pkg.dev us-central1-docker.pkg.dev
comma := ,
docker-auth: ## Configure the gcloud credential helper for $(REGISTRY) and the base-image registries
	gcloud auth configure-docker $(subst $() ,$(comma),$(sort $(REGISTRY) $(BASE_REGISTRIES))) --quiet

# --- scan / verify ----------------------------------------------------------
BULWARK  ?= bulwark
SEVERITY ?= critical,high

.PHONY: scan scan-all
scan: ## bulwark crit/high scan of syncer + upsync + hpm (current FIPS=)
	@set -euo pipefail; \
	for img in $(VCLUSTER_IMAGES); do \
	  echo "=== $$img ==="; \
	  $(BULWARK) image --severity $(SEVERITY) $$img; \
	done
scan-all: ## bulwark crit/high scan of every FIPS image incl. coredns
	@$(MAKE) --no-print-directory scan FIPS=true
	@set -euo pipefail; \
	for img in $(COREDNS_IMAGE); do \
	  echo "=== $$img ==="; \
	  $(BULWARK) image --severity $(SEVERITY) $$img; \
	done

# verify_binaries(want, image:binary pairs): want=fips asserts each binary is a
# GOFIPS140 build, want=nonfips asserts it is not. Works on distroless images.
define verify_binaries
	@set -euo pipefail; \
	rc=0; tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
	for spec in $(2); do \
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
	  if [ "$(1)" = fips ] && [ -n "$$fips" ]; then echo "ok    $$img $$bin ($$gover, $$fips)"; \
	  elif [ "$(1)" = nonfips ] && [ -z "$$fips" ]; then echo "ok    $$img $$bin ($$gover, no GOFIPS140)"; \
	  else echo "FAIL  $$img $$bin ($$gover, $${fips:-no GOFIPS140}, want $(1))"; rc=1; fi; \
	  rm -f "$$tmp/b"; \
	done; \
	exit $$rc
endef

VERIFY_TARGETS := $(SYNCER_IMAGE):/vcluster $(SYNCER_IMAGE):/usr/local/bin/kine \
  $(SYNCER_IMAGE):/usr/local/bin/helm $(UPSYNC_IMAGE):/plugin/plugin $(HPM_IMAGE):/vcluster-hpm

.PHONY: verify verify-all
verify: ## Check syncer + upsync + hpm binaries match the current FIPS= variant
	$(call verify_binaries,$(VARIANT),$(VERIFY_TARGETS))
verify-all: ## Assert every FIPS image's binaries are GOFIPS140 builds (incl. coredns)
	@$(MAKE) --no-print-directory verify FIPS=true
	$(call verify_binaries,fips,$(COREDNS_IMAGE):/coredns)

.PHONY: clean
clean: ## Remove the locally built tags (current FIPS=) and the fetched sources
	-$(DOCKER) rmi $(ALL_IMAGES) 2>/dev/null || true
	rm -rf "$(BUILD_DIR)"
