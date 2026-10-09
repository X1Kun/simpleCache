.PHONY: test race build vet check test-operator fmt-check test-ci test-operator-ci compose-config generated-check k8s-render
.PHONY: kind-up kind-smoke kind-delete

# Prevent Go from invoking Git for automatic version stamping.
export GOFLAGS := $(GOFLAGS) -buildvcs=false

test:
	cd geecache-engine && go test ./...
race:
	cd geecache-engine && go test -race ./...
build:
	cd geecache-engine && go build -o /dev/null ./cmd/simplecache
	cd simplecache-operator && go build -o /dev/null ./cmd
vet:
	cd geecache-engine && go vet ./...
	cd simplecache-operator && go vet ./cmd/... ./api/... ./internal/...
# Operator tests use a temporary API server and etcd, not a deployed cluster.
test-operator:
	$(MAKE) -C simplecache-operator test

fmt-check:
	test -z "$$(gofmt -l geecache-engine/cmd geecache-engine/internal simplecache-operator/api simplecache-operator/cmd simplecache-operator/internal simplecache-operator/test)"

test-ci:
	cd geecache-engine && go test -race -count=1 -timeout=3m -coverprofile=cover.out ./...

test-operator-ci:
	$(MAKE) -C simplecache-operator test-ci

compose-config:
	docker compose -f geecache-engine/docker-compose.yml config --quiet

generated-check:
	bash scripts/check-generated.sh

k8s-render:
	$(MAKE) -C simplecache-operator kustomize
	simplecache-operator/bin/kustomize build simplecache-operator/config/default >/dev/null
	simplecache-operator/bin/kustomize build simplecache-operator/config/samples >/dev/null
	simplecache-operator/bin/kustomize build deploy/kind/operator >/dev/null
# Version-control operations belong to the user.
check: fmt-check test race build vet

kind-up:
	bash scripts/kind/up.sh

kind-smoke:
	bash scripts/kind/smoke.sh

kind-delete:
	bash scripts/kind/delete.sh
