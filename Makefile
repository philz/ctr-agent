.PHONY: build clean

# Get git configuration or use defaults
GIT_USER_NAME := $(shell git config --get user.name || echo "Container User")
GIT_USER_EMAIL := $(shell git config --get user.email || echo "user@example.com")

# Docker image tag expected by run-in-container.sh
IMAGE_TAG := container-agent:dev

build:
	@echo "Building Docker image with git user: $(GIT_USER_NAME) <$(GIT_USER_EMAIL)>"
	docker build --progress=plain \
		--build-arg GIT_USER_NAME="$(GIT_USER_NAME)" \
		--build-arg GIT_USER_EMAIL="$(GIT_USER_EMAIL)" \
		--mount=type=cache,target=/root/.cache/go-build \
		--mount=type=cache,target=/go/pkg/mod \
		-t $(IMAGE_TAG) .

clean:
	docker rmi $(IMAGE_TAG) || true
