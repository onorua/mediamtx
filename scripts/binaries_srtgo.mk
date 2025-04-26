include scripts/common.mk

BINARY_NAME = mediamtx
VERSION = $(shell cat internal/core/VERSION)

OUT_DIR = $(SRC_DIR)/binaries
SYSROOTS = $(TMP_DIR)/sysroots

define BUILD_MEDIAMTX
	$(call SET_ENV,$(3))

	$(eval GOOS = $(1))
	$(eval GOARCH = $(2))
	$(eval GOARCH_OUT_DIR = $(OUT_DIR)/$(GOOS)-$(GOARCH))

	mkdir -p $(GOARCH_OUT_DIR)

	GOOS=$(GOOS) \
	GOARCH=$(GOARCH) \
	CC=$(CC_PREFIX)gcc \
	CGO_ENABLED=1 \
	CGO_CFLAGS=$(CFLAGS) \
	CGO_LDFLAGS="$(LDFLAGS) $(SYSROOT)/lib/libsrt.a -lcrypto -lssl -lstdc++ -lm" \
	go build -o $(GOARCH_OUT_DIR)/$(BINARY_NAME)

	$(CC_PREFIX)strip --strip-unneeded $(GOARCH_OUT_DIR)/$(BINARY_NAME)

	cp mediamtx.yml LICENSE $(GOARCH_OUT_DIR)/
	tar -C $(GOARCH_OUT_DIR) -czf $(OUT_DIR)/$(BINARY_NAME)_$(VERSION)_$(GOOS)_$(GOARCH).tar.gz \
		--owner=0 --group=0 $(BINARY_NAME) mediamtx.yml LICENSE
endef

MEDIAMTX_PREPARE_STAMP = $(STAMP_DIR)/mediamtx_prepare.stamp

mediamtx_prepare: $(MEDIAMTX_PREPARE_STAMP)
$(MEDIAMTX_PREPARE_STAMP):
	echo "Preparing mediamtx"
	mkdir -p $(OUT_DIR)
	mkdir -p $(SYSROOTS)/aarch64
	mkdir -p $(SYSROOTS)/x86_64
	mkdir -p $(STAMP_DIR)
	touch $@
.PHONY: mediamtx_prepare

mediamtx_linux_amd64: mediamtx_prepare $(OUT_DIR)/linux-amd64/$(BINARY_NAME)
$(OUT_DIR)/linux-amd64/$(BINARY_NAME):
	$(call BUILD_LIBSRT,x86_64)
	$(call BUILD_MEDIAMTX,linux,amd64,x86_64)
.PHONY: mediamtx_linux_amd64

mediamtx_linux_arm64: mediamtx_prepare $(OUT_DIR)/linux-arm64/$(BINARY_NAME)
$(OUT_DIR)/linux-arm64/$(BINARY_NAME):
	$(eval CC_PREFIX = aarch64-linux-gnu-)
	$(call BUILD_LIBSRT,aarch64)
	$(call BUILD_MEDIAMTX,linux,arm64,aarch64)
.PHONY: mediamtx_linux_arm64

mediamtx_linux_all: mediamtx_linux_amd64 mediamtx_linux_arm64
.PHONY: mediamtx_linux_all

mediamtx_clean:
	rm -rf $(OUT_DIR)
	rm -f $(MEDIAMTX_PREPARE_STAMP)
