include scripts/common.mk

LIBSRT_VERSION ?= 1.5.4
LIBSRT_DIR = $(TMP_DIR)/libsrt
LIBSRT_SRC_DIR = $(LIBSRT_DIR)/$(LIBSRT_VERSION)
LIBSRT_TAR = $(LIBSRT_DIR)/v$(LIBSRT_VERSION).tar.gz
LIBSRT_URL = https://github.com/Haivision/srt/archive/refs/tags/v$(LIBSRT_VERSION).tar.gz
LIBSRT_STAMP = $(STAMP_DIR)/libsrt-$(LIBSRT_VERSION)-$(TARGET).stamp

LIBSRT_CMAKE_CONF_OPTS += \
	-DENABLE_STDCXX_SYNC=ON \
	-DENABLE_ENCRYPTION=ON \
	-DENABLE_BONDING=ON \
	-DENABLE_SHOW_PROJECT_CONFIG=ON \
	-DENABLE_SHARED=OFF \
	-DENABLE_STATIC=ON \
	-DENABLE_LOGGING=ON \
	-DENABLE_HEAVY_LOGGING=ON

define build_libsrt_def
	$(eval TARGET = $(1))
	$(eval CC_PREFIX = $(2))
	$(eval INSTALL_DIR = $(3))
	
	$(eval BUILD_DIR = $(LIBSRT_SRC_DIR)/build-$(TARGET))

	rm -rf $(LIBSRT_SRC_DIR)
	mkdir -p $(LIBSRT_SRC_DIR)
	tar -xzf $(LIBSRT_TAR) -C $(LIBSRT_SRC_DIR) --strip-components=1

	mkdir -p $(BUILD_DIR)

	@echo "=========================="
	@echo "Building libsrt $(LIBSRT_VERSION) for $(TARGET)"
	@echo "=========================="
	@echo "TARGET: $(TARGET)"
	@echo "CC_PREFIX: $(CC_PREFIX)"
	@echo "INSTALL_DIR: $(INSTALL_DIR)"
	@echo "BUILD_DIR: $(BUILD_DIR)"
	@echo "CFLAGS: $(CFLAGS)"
	@echo "CXXFLAGS: $(CXXFLAGS)"
	@echo "LDFLAGS: $(LDFLAGS)"
	@echo "LIBSRT_CMAKE_CONF_OPTS: $(LIBSRT_CMAKE_CONF_OPTS)"
	@echo "=========================="

	(cmake \
		-DCMAKE_C_COMPILER=$(CC_PREFIX)gcc \
		-DCMAKE_CXX_COMPILER=$(CC_PREFIX)g++ \
		-DCMAKE_INSTALL_PREFIX=$(INSTALL_DIR) \
		-DCMAKE_SYSTEM_PROCESSOR=$(TARGET) \
		-DCMAKE_INSTALL_DO_STRIP=ON \
		$(if $(CFLAGS),-DCMAKE_C_FLAGS="$(CFLAGS)") \
		$(if $(CXXFLAGS),-DCMAKE_CXX_FLAGS="$(CXXFLAGS)") \
		$(if $(LDFLAGS),-DCMAKE_EXE_LINKER_FLAGS="$(LDFLAGS)") \
		$(if $(LDFLAGS),-DCMAKE_SHARED_LINKER_FLAGS="$(LDFLAGS)") \
		$(LIBSRT_CMAKE_CONF_OPTS) \
		-B $(BUILD_DIR) \
		-S $(LIBSRT_SRC_DIR); \
	cmake --build $(BUILD_DIR) --target install -- -j$(PARALLEL_JOBS))
endef

$(LIBSRT_TAR):
	mkdir -p $(TMP_DIR) $(LIBSRT_DIR)
	curl -L -o $(LIBSRT_TAR) $(LIBSRT_URL)

libsrt_clean:
	rm -rf $(LIBSRT_DIR)
	rm -f $(STAMP_DIR)/libsrt-*
.PHONY: libsrt_clean

libsrt_build: $(LIBSRT_TAR) $(LIBSRT_STAMP)
$(LIBSRT_STAMP):
	$(call build_libsrt_def,$(TARGET),$(CC_PREFIX),$(INSTALL_DIR))
	@touch $@
.PHONY: libsrt_build
