.PHONY: all fmt fmt-write vet test build speaker android speech-models apk

all:
	./hack/make.sh all

fmt:
	./hack/make.sh fmt

fmt-write:
	./hack/make.sh fmt-write

vet:
	./hack/make.sh vet

test:
	./hack/make.sh test

build:
	./hack/make.sh build

speaker:
	./hack/make.sh speaker

android:
	./hack/make.sh android

speech-models:
	./hack/make.sh speech-models

apk:
	./hack/make.sh apk
