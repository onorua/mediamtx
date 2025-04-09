include scripts/common.mk

MUSL_CROSS_VERSION ?= master
MUSL_CROSS_TARBALL_URL = https://github.com/richfelker/musl-cross-make/archive/refs/heads/$(MUSL_CROSS_VERSION).tar.gz
MUSL_CROSS_TARBALL_FILE = $(MUSL_CROSS_DIR)/$(MUSL_CROSS_VERSION).tar.gz
MUSL_CROSS_DIR = $(TMP_DIR)/musl-cross
MUSL_CROSS_SRC_DIR = $(MUSL_CROSS_DIR)/$(MUSL_CROSS_VERSION)

define build_musl_cross
	$(eval ARCH = $(1))
	$(eval OS = $(2))
	$(eval ABI = $(3))
	$(eval OUTPUT = $(MUSL_CROSS_DIR)/$(1)-$(2)-musl$(3))

	@echo "Building musl cross compiler for os=\"$(OS)\" arch=\"$(ARCH)\" abi=\"$(ABI)\" output=\"$(OUTPUT)\""
	( \
		mkdir -p $(MUSL_CROSS_SRC_DIR); \
		tar -xz -f $(MUSL_CROSS_TARBALL_FILE) -C $(MUSL_CROSS_SRC_DIR) --strip-components=1 ; \
		echo "TARGET = $(ARCH)-$(OS)-musl$(ABI)" > $(MUSL_CROSS_SRC_DIR)/config.mak; \
		$(MAKE) -C $(MUSL_CROSS_SRC_DIR) -j$(PARALLEL_JOBS); \
		$(MAKE) -C $(MUSL_CROSS_SRC_DIR) install OUTPUT=$(OUTPUT); \
	)
endef

.PHONY: musl_cross_build_x86_64_linux musl_cross_build_aarch64_linux musl_cross_build_all musl_cross_download musl_cross_clean

musl_cross_build_x86_64_linux: musl_cross_download $(MUSL_CROSS_DIR)/x86_64_linux_musl
$(MUSL_CROSS_DIR)/x86_64_linux_musl: 
	$(call build_musl_cross,x86_64,linux,)

musl_cross_build_aarch64_linux: musl_cross_download $(MUSL_CROSS_DIR)/aarch64_linux_musl
$(MUSL_CROSS_DIR)/aarch64_linux_musl:
	$(call build_musl_cross,aarch64,linux,)

musl_cross_build_all: musl_cross_build_x86_64_linux musl_cross_build_aarch64_linux

musl_cross_clean:
	rm -rf $(MUSL_CROSS_DIR)

musl_cross_download: $(MUSL_CROSS_TARBALL_FILE)
$(MUSL_CROSS_TARBALL_FILE):
	mkdir -p $(MUSL_CROSS_DIR)
	curl -L $(MUSL_CROSS_TARBALL_URL) -o $(MUSL_CROSS_TARBALL_FILE)


