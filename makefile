NAME := mute-service
OWNER := byuoitav
PKG := github.com/${OWNER}/${NAME}
DOCKER_URL := docker.pkg.github.com

# version:
# use the git tag, if this commit
# doesn't have a tag, use the git hash
COMMIT_HASH := $(shell git rev-parse --short HEAD)
TAG := $(shell git rev-parse --short HEAD)
ifneq ($(shell git describe --exact-match --tags HEAD 2> /dev/null),)
	TAG = $(shell git describe --exact-match --tags HEAD)
endif

PRD_TAG_REGEX := "v[0-9]+\.[0-9]+\.[0-9]+"
DEV_TAG_REGEX := "v[0-9]+\.[0-9]+\.[0-9]+-.+"

# go stuff
PKG_LIST := $(shell go list ${PKG}/...)

.PHONY: all deps build test test-cov clean

all: clean build

test:
	@go test -v ${PKG_LIST}

test-cov:
	@go test -coverprofile=coverage.txt -covermode=atomic ${PKG_LIST}

lint:
	@golangci-lint run --tests=false

deps:
	@echo Downloading backend dependencies...
	@go mod download

build: deps
	@mkdir -p dist

	@echo
	@echo Building central monitoring for linux-amd64...
	@env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o ./dist/${NAME}-central-linux-amd64 ${BUILD_PKG}

	@echo
	@echo Building local monitoring for linux-arm...
	@env CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -v -o ./dist/${NAME}-local-linux-arm ${BUILD_PKG}

	@echo
	@echo Build output is located in ./dist/.

docker: clean build
ifeq (${COMMIT_HASH}, ${TAG})
	@echo Building central dev container with tag ${COMMIT_HASH}

	@echo Building container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-central-dev:${COMMIT_HASH}
	@docker build -f dockerfile --build-arg NAME=${NAME}-central-linux-amd64 -t ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-central-dev:${COMMIT_HASH} dist

	@echo Building local dev container with tag ${COMMIT_HASH}

	@echo Building container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-local-dev:${COMMIT_HASH}
	@docker build -f dockerfile --build-arg NAME=${NAME}-local-linux-arm -t ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-local-dev:${COMMIT_HASH} dist

else ifneq ($(shell echo ${TAG} | grep -x -E ${DEV_TAG_REGEX}),)
	@echo Building central dev container with tag ${TAG}

	@echo Building container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-central-dev:${TAG}
	@docker build -f dockerfile --build-arg NAME=${NAME}-central-linux-amd64 -t ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-central-dev:${TAG} dist

	@echo Building local dev container with tag ${TAG}

	@echo Building container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-local-dev:${TAG}
	@docker build -f dockerfile --build-arg NAME=${NAME}-local-linux-arm -t ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-local-dev:${TAG} dist

else ifneq ($(shell echo ${TAG} | grep -x -E ${PRD_TAG_REGEX}),)
	@echo Building central prd container with tag ${TAG}

	@echo Building container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-central:${TAG}
	@docker build -f dockerfile --build-arg NAME=${NAME}-central-linux-amd64 -t ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-central:${TAG} dist

	@echo Building local prd container with tag ${TAG}

	@echo Building container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-local:${TAG}
	@docker build -f dockerfile --build-arg NAME=${NAME}-local-linux-arm -t ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-local:${TAG} dist

endif

deploy: docker
	@echo Logging into Github Package Registry
	@docker login ${DOCKER_URL} -u ${DOCKER_USERNAME} -p ${DOCKER_PASSWORD}

ifeq (${COMMIT_HASH}, ${TAG})
	@echo Pushing central dev container with tag ${COMMIT_HASH}

	@echo Pushing container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-central-dev:${COMMIT_HASH}
	@docker push ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-central-dev:${COMMIT_HASH}

	@echo Pushing local dev container with tag ${COMMIT_HASH}

	@echo Pushing container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-local-dev:${COMMIT_HASH}
	@docker push ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-local-dev:${COMMIT_HASH}

else ifneq ($(shell echo ${TAG} | grep -x -E ${DEV_TAG_REGEX}),)
	@echo Pushing central dev container with tag ${TAG}

	@echo Pushing container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-central-dev:${TAG}
	@docker push ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-central-dev:${TAG}

	@echo Pushing local dev container with tag ${TAG}

	@echo Pushing container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-local-dev:${TAG}
	@docker push ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-local-dev:${TAG}

else ifneq ($(shell echo ${TAG} | grep -x -E ${PRD_TAG_REGEX}),)
	@echo Pushing central prd container with tag ${TAG}

	@echo Pushing container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-central:${TAG}
	@docker push ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-central:${TAG}

	@echo Pushing local prd container with tag ${TAG}

	@echo Pushing container ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-local:${TAG}
	@docker push ${DOCKER_URL}/${OWNER}/${NAME}/${NAME}-local:${TAG}

endif

clean:
	@go clean
	@rm -rf dist/