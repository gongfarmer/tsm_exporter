# Needs to be defined before including Makefile.common to auto-generate targets
DOCKER_ARCHS ?= amd64 ppc64le
DOCKER_REPO	 ?= treydock

include Makefile.common

DOCKER_IMAGE_NAME ?= tsm_exporter

coverage:
	go test -race -coverpkg=./... -coverprofile=coverage.txt -covermode=atomic ./...
