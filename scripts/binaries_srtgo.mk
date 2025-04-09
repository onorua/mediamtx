BINARY_NAME = mediamtx

TMP_DIR = $(shell pwd)/tmp
MEDIAMTX_DIR = ${TMP_DIR}/mediamtx
MEDIAMTX_SRC_DIR = $(shell pwd)
MEDIAMTX_ARTIFACTS_DIR = ${MEDIAMTX_DIR}/artifacts

define build_binary_linux
	$(eval GOOS = linux)
	$(eval GOARCH = $(1))
	$(eval CC_PREFIX = $(2))

	GOOS=$(GOOS) \
	GOARCH=$(GOARCH) \
	CGO_ENABLED=1 \
	CC=$(CC_PREFIX)gcc \
	CGO_CFLAGS="-I$(LIBSRT_ARTIFACTS_DIR)/linux-$(GOARCH)/include" \
	CGO_LDFLAGS="-L$(LIBSRT_ARTIFACTS_DIR)/linux-$(GOARCH)/lib/ $(LIBSRT_ARTIFACTS_DIR)/linux-$(GOARCH)/lib/libsrt.a -lcrypto -lssl -lstdc++ -lm" \
	go build -o $(MEDIAMTX_ARTIFACTS_DIR)/linux-$(GOARCH)/$(BINARY_NAME)
endef

define package_binary
	mkdir -p $(MEDIAMTX_SRC_DIR)/binaries
	cp $(MEDIAMTX_SRC_DIR)/mediamtx.yml $(MEDIAMTX_ARTIFACTS_DIR)/linux-$(1)
	cp $(MEDIAMTX_SRC_DIR)/LICENSE $(MEDIAMTX_ARTIFACTS_DIR)/linux-$(1)
	tar -C $(MEDIAMTX_ARTIFACTS_DIR)/linux-$(1) -cz \
		-f "$(MEDIAMTX_SRC_DIR)/binaries/$(BINARY_NAME)_$(shell cat $(MEDIAMTX_SRC_DIR)/internal/core/VERSION)_linux_$(1).tar.gz" \
		--owner=0 --group=0 \
		$(BINARY_NAME) mediamtx.yml LICENSE
endef

.PHONY: go_generate binary_linux_amd64 binary_linux_arm64

go_generate:
	go generate ./...

binary_linux_amd64: libsrt_linux_amd64
	$(call build_binary_linux,amd64,)
	$(call package_binary,amd64)

binary_linux_arm64: libsrt_linux_arm64
	$(call build_binary_linux,arm64,aarch64-linux-gnu-)
	$(call package_binary,arm64)
