.PHONY: all
all: container plugin

.PHONY: container
container:
	docker build . -t quay.io/ellorent/kubevirt-passt-binding

.PHONY: push
push: container
	docker push quay.io/ellorent/kubevirt-passt-binding

.PHONY: plugin
plugin:
	go build -o kubevirt-passt-binding ./cmd/cni/

.PHONY: cluster-sync
cluster-sync: plugin push
	cluster/sync.sh

