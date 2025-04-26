include scripts/common.mk

BINARY_NAME = mediamtx
VERSION = $(shell cat internal/core/VERSION)

OUT_DIR = $(SRC_DIR)/binaries
SYSROOTS = $(TMP_DIR)/sysroots

define SET_MUSL_ENV
	$(call SET_ENV,$(1))
	$(eval MUSL_CROSS_DIR = $(TMP_DIR)/musl-cross)
	$(eval CC_PREFIX = $(MUSL_CROSS_DIR)/$(ARCH)-linux-musl/bin/$(ARCH)-linux-musl-)
endef

define BUILD_MUSL
	$(eval ARCH = $(1))
	make musl_cross_build_$(ARCH)_linux
endef

define BUILD_MEDIAMTX_WITH_MUSL
	$(eval GOOS = $(1))
	$(eval GOARCH = $(2))
	$(eval GOARCH_OUT_DIR = $(OUT_DIR)/$(GOOS)-$(GOARCH)-musl)

	mkdir -p $(GOARCH_OUT_DIR)

	GOOS=$(GOOS) \
	GOARCH=$(GOARCH) \
	CC=$(CC_PREFIX)cc \
	CGO_ENABLED=1 \
	CGO_CFLAGS=$(CFLAGS) \
	CGO_LDFLAGS="$(LDFLAGS)" \
	go build -tags netgo \
		-ldflags '-linkmode external -extldflags "-Wl,-Bstatic -lc -lstdc++ -lm -lpthread -ldl -Wl,-Bdynamic -lssl -lcrypto"' \
		-o $(GOARCH_OUT_DIR)/$(BINARY_NAME)

	$(CC_PREFIX)strip --strip-unneeded $(GOARCH_OUT_DIR)/$(BINARY_NAME)

	cp mediamtx.yml LICENSE $(GOARCH_OUT_DIR)/
	tar -C $(GOARCH_OUT_DIR) -czf $(OUT_DIR)/$(BINARY_NAME)_$(VERSION)_$(GOOS)_$(GOARCH)_musl.tar.gz \
		--owner=0 --group=0 $(BINARY_NAME) mediamtx.yml LICENSE
endef

MEDIAMTX_PREPARE_MUSL_STAMP = $(STAMP_DIR)/mediamtx_prepare_musl.stamp

mediamtx_prepare_musl: $(MEDIAMTX_PREPARE_MUSL_STAMP)
$(MEDIAMTX_PREPARE_MUSL_STAMP):
	echo "Preparing mediamtx"
	mkdir -p $(OUT_DIR)
	mkdir -p $(SYSROOTS)/aarch64
	mkdir -p $(SYSROOTS)/x86_64
	mkdir -p $(STAMP_DIR)
	touch $@
.PHONY: mediamtx_prepare_musl

mediamtx_linux_amd64_musl: mediamtx_prepare_musl $(OUT_DIR)/linux-amd64-musl/$(BINARY_NAME)
$(OUT_DIR)/linux-amd64-musl/$(BINARY_NAME):
	$(call SET_MUSL_ENV,x86_64)
	$(call BUILD_MUSL,x86_64)
	$(call BUILD_OPENSSL,x86_64)
	$(call BUILD_LIBSRT,x86_64)
	$(call BUILD_MEDIAMTX_WITH_MUSL,linux,amd64,x86_64)
.PHONY: mediamtx_linux_amd64_musl

mediamtx_linux_arm64_musl: mediamtx_prepare_musl $(OUT_DIR)/linux-arm64-musl/$(BINARY_NAME)
$(OUT_DIR)/linux-arm64-musl/$(BINARY_NAME):
	$(call SET_MUSL_ENV,aarch64)
	$(call BUILD_MUSL,aarch64)
	$(call BUILD_OPENSSL,aarch64)
	$(call BUILD_LIBSRT,aarch64)
	$(call BUILD_MEDIAMTX_WITH_MUSL,linux,arm64,aarch64)
.PHONY: mediamtx_linux_arm64_musl

mediamtx_linux_musl_all: mediamtx_linux_amd64_musl mediamtx_linux_arm64_musl
.PHONY: mediamtx_linux_musl_all

mediamtx_clean_musl:
	rm -rf $(OUT_DIR)
	rm -f $(MEDIAMTX_PREPARE_MUSL_STAMP)
.PHONY: mediamtx_clean_musl
