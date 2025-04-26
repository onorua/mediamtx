include scripts/common.mk

OPENSSL_VERSION ?= 3.5.0
OPENSSL_BASE_URL = https://www.openssl.org/source
OPENSSL_DIR = $(TMP_DIR)/openssl
OPENSSL_SRC_DIR = $(OPENSSL_DIR)/$(OPENSSL_VERSION)
OPENSSL_TAR = $(OPENSSL_DIR)/openssl-$(OPENSSL_VERSION).tar.gz
OPENSSL_STAMP = $(STAMP_DIR)/openssl-$(OPENSSL_VERSION)-$(TARGET).stamp

define build_openssl_def
	$(eval TARGET = $(1))
	$(eval CC_PREFIX = $(2))
	$(eval INSTALL_DIR = $(3))

	@echo "=========================="
	@echo "Building OpenSSL $(OPENSSL_VERSION) for $(TARGET)"
	@echo "=========================="
	@echo "TARGET: $(TARGET)"
	@echo "CC_PREFIX: $(CC_PREFIX)"
	@echo "INSTALL_DIR: $(INSTALL_DIR)"
	@echo "CFLAGS: $(CFLAGS)"
	@echo "CXXFLAGS: $(CXXFLAGS)"
	@echo "LDFLAGS: $(LDFLAGS)"
	@echo "=========================="

	rm -rf $(OPENSSL_SRC_DIR)
	mkdir -p $(OPENSSL_SRC_DIR)
	tar -xzf $(OPENSSL_TAR) -C $(OPENSSL_SRC_DIR) --strip-components=1
	(cd $(OPENSSL_SRC_DIR) && \
	./Configure $(TARGET) no-tests --cross-compile-prefix=$(CC_PREFIX) --prefix=$(INSTALL_DIR) && \
	make -j$(PARALLEL_JOBS) && \
	make install_sw)
endef

$(OPENSSL_TAR):
	mkdir -p $(OPENSSL_DIR)
	curl -L $(OPENSSL_BASE_URL)/openssl-$(OPENSSL_VERSION).tar.gz -o $(OPENSSL_TAR)

openssl_build: $(OPENSSL_TAR) $(OPENSSL_STAMP) 
$(OPENSSL_STAMP):
	$(call build_openssl_def,$(TARGET),$(CC_PREFIX),$(INSTALL_DIR))
	@touch $@
.PHONY: openssl_build

openssl_clean:
	rm -rf $(OPENSSL_DIR)
	rm -f $(STAMP_DIR)/openssl-*
.PHONY: openssl_clean
