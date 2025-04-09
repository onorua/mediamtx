include scripts/common.mk

OPENSSL_VERSION ?= 3.5.0
OPENSSL_BASE_URL = https://www.openssl.org/source
OPENSSL_DIR = $(TMP_DIR)/openssl
OPENSSL_SRC_DIR = $(OPENSSL_DIR)/$(OPENSSL_VERSION)
OPENSSL_TAR = $(OPENSSL_DIR)/openssl-$(OPENSSL_VERSION).tar.gz

define BUILD_OPENSSL
	$(eval TARGET = $(1))
	$(eval CC_PREFIX = $(2))
	$(eval INSTALL_DIR = $(3))

	@echo "Building OpenSSL $(OPENSSL_VERSION) for $(TARGET)"
	rm -rf $(OPENSSL_SRC_DIR)
	mkdir -p $(OPENSSL_SRC_DIR)
	tar -xzf $(OPENSSL_TAR) -C $(OPENSSL_SRC_DIR) --strip-components=1
	(cd $(OPENSSL_SRC_DIR) && \
	./Configure $(TARGET) no-tests --cross-compile-prefix=$(CC_PREFIX) --prefix=$(INSTALL_DIR) && \
	make -j$(PARALLEL_JOBS) && \
	make install_sw)
endef

.PHONY: openssl_build openssl_download openssl_clean

openssl_clean:
	rm -rf $(OPENSSL_DIR)

openssl_download: $(OPENSSL_TAR)
$(OPENSSL_TAR):
	mkdir -p $(OPENSSL_DIR)
	curl -L $(OPENSSL_BASE_URL)/openssl-$(OPENSSL_VERSION).tar.gz -o $(OPENSSL_TAR)
