REGISTRY ?= quay.io
REMOTE ?= osd-addons
REPO ?= mcg-osd
TAG ?= harness

all: build push

# the harness image is used by osde2e runners to run the tests and generate a
# junit output that would be parsed further to determine the status of the
# test run.
build:
	# build the harness image.
	@docker build . -t $(REGISTRY)/$(REMOTE)/$(REPO):$(TAG) -f harness.Dockerfile

push:
	# push the harness image to official remote.
	@docker push $(REGISTRY)/$(REMOTE)/$(REPO):$(TAG)
