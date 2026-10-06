.PHONY: test race build vet check test-operator

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
# Operator retains the original envtest suite in stage 1.
test-operator:
	$(MAKE) -C simplecache-operator test
# Version-control operations belong to the user.
check: test race build vet
