SRC_DIR ?= $(shell pwd)
TMP_DIR ?= $(SRC_DIR)/tmp
PARALLEL_JOBS ?= $(shell nproc)
STAMP_DIR = $(TMP_DIR)/stamps

define SET_ENV
	$(eval ARCH = $(1))

	$(eval CFLAGS = -I$(SYSROOTS)/$(ARCH)/include)
	$(eval CXXFLAGS = -I$(SYSROOTS)/$(ARCH)/include)
	$(eval LDFLAGS = -L$(SYSROOTS)/$(ARCH)/lib -L$(SYSROOTS)/$(ARCH)/lib64)
	$(eval SYSROOT = $(SYSROOTS)/$(ARCH))
endef

define BUILD_LIBSRT
	$(call SET_ENV,$(1))
	$(eval TARGET = $(1))
	+make \
		TARGET=$(TARGET) \
		CC_PREFIX=$(CC_PREFIX) \
		INSTALL_DIR=$(SYSROOT) \
		CFLAGS="$(CFLAGS)" \
		CXXFLAGS="$(CXXFLAGS)" \
		LDFLAGS="$(LDFLAGS)" \
		libsrt_build
endef

define BUILD_OPENSSL
	$(call SET_ENV,$(1))
	$(eval TARGET = linux-$(1))
	+make \
		TARGET=$(TARGET) \
		CC_PREFIX=$(CC_PREFIX) \
		INSTALL_DIR=$(SYSROOT) \
		CFLAGS="$(CFLAGS)" \
		CXXFLAGS="$(CXXFLAGS)" \
		LDFLAGS="$(LDFLAGS)" \
		openssl_build
endef