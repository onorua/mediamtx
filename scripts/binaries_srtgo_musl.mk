BINARY_NAME = mediamtx
VERSION = $(shell cat internal/core/VERSION)

OUT_DIR = $(SRC_DIR)/binaries
SYSROOTS = $(TMP_DIR)/sysroots
STAMP_DIR = $(TMP_DIR)/stamps

define SET_MUSL_ENV
	$(eval ARCH = $(1))

	$(eval CC_PREFIX = $(MUSL_CROSS_DIR)/$(ARCH)-linux-musl/bin/$(ARCH)-linux-musl-)
	$(eval CFLAGS = -I$(SYSROOTS)/$(ARCH)/include)
	$(eval CXXFLAGS = -I$(SYSROOTS)/$(ARCH)/include)
	$(eval LDFLAGS = -L$(SYSROOTS)/$(ARCH)/lib -L$(SYSROOTS)/$(ARCH)/lib64)
	$(eval SYSROOT = $(SYSROOTS)/$(ARCH))

	@(echo "========================="; \
	echo "CC_PREFIX: $(CC_PREFIX)"; \
	echo "CFLAGS: $(CFLAGS)"; \
	echo "CXXFLAGS: $(CXXFLAGS)"; \
	echo "LDFLAGS: $(LDFLAGS)"; \
	echo "SYSROOT: $(SYSROOT)"; \
	echo "=========================")
endef

define BUILD_LIBSRT_WITH_MUSL
	$(call SET_MUSL_ENV,$(1))
	$(call build_libsrt,$(ARCH),$(CC_PREFIX),$(SYSROOT))
endef

define BUILD_OPENSSL_WITH_MUSL
	$(call SET_MUSL_ENV,$(1))
	$(eval TARGET = linux-$(ARCH))
	$(call BUILD_OPENSSL,$(TARGET),$(CC_PREFIX),$(SYSROOT))
endef

define BUILD_MEDIAMTX_MUSL
	$(eval GOOS = $(1))
	$(eval GOARCH = $(2))
	$(call SET_MUSL_ENV,$(3))

	$(eval GOARCH_OUT_DIR = $(OUT_DIR)/$(GOOS)-$(GOARCH))

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

	$(MUSL_CROSS_DIR)/$(ARCH)-linux-musl/bin/$(ARCH)-linux-musl-strip \
		--strip-unneeded $(GOARCH_OUT_DIR)/$(BINARY_NAME)

	cp mediamtx.yml LICENSE $(GOARCH_OUT_DIR)/
	tar -C $(GOARCH_OUT_DIR) -czf $(OUT_DIR)/$(BINARY_NAME)_$(VERSION)_$(GOOS)_$(GOARCH).tar.gz \
		--owner=0 --group=0 $(BINARY_NAME) mediamtx.yml LICENSE
endef


mediamtx_prepare: musl_cross_build_all
	@echo "Preparing mediamtx"
	mkdir -p $(OUT_DIR)
	mkdir -p $(SYSROOTS)/aarch64
	mkdir -p $(SYSROOTS)/x86_64
	mkdir -p $(STAMP_DIR)
.PHONY: mediamtx_prepare

build_openssl_with_musl: openssl_download $(STAMP_DIR)/openssl_$(OPENSSL_VERSION).stamp
$(STAMP_DIR)/openssl_$(OPENSSL_VERSION).stamp: openssl_download
	$(call BUILD_OPENSSL_WITH_MUSL,x86_64)
	$(call BUILD_OPENSSL_WITH_MUSL,aarch64)
	@touch $@

build_libsrt_with_musl: build_openssl_with_musl libsrt_download $(STAMP_DIR)/libsrt_$(LIBSRT_VERSION).stamp
$(STAMP_DIR)/libsrt_$(LIBSRT_VERSION).stamp: 
	$(call BUILD_LIBSRT_WITH_MUSL,x86_64)
	$(call BUILD_LIBSRT_WITH_MUSL,aarch64)
	@touch $@

mediamtx_linux_amd64: $(OUT_DIR)/linux-amd64/$(BINARY_NAME)
$(OUT_DIR)/linux-amd64/$(BINARY_NAME): mediamtx_prepare build_openssl_with_musl build_libsrt_with_musl
	$(call BUILD_MEDIAMTX_MUSL,linux,amd64,x86_64)
.PHONY: mediamtx_linux_amd64

mediamtx_linux_arm64: $(OUT_DIR)/linux-arm64/$(BINARY_NAME)
$(OUT_DIR)/linux-arm64/$(BINARY_NAME): mediamtx_prepare build_openssl_with_musl build_libsrt_with_musl
	$(call BUILD_MEDIAMTX_MUSL,linux,arm64,aarch64)
.PHONY: mediamtx_linux_arm64

mediamtx_linux_all: mediamtx_linux_amd64 mediamtx_linux_arm64
.PHONY: mediamtx_linux_all

mediamtx_clean:
	rm -rf $(SYSROOTS)
	rm -rf $(OUT_DIR)
