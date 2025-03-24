TMP_DIR ?= $(shell pwd)/tmp

LIBSRT_DIR = $(TMP_DIR)/libsrt
LIBSRT_SRC_DIR = $(LIBSRT_DIR)/$(LIBSRT_VERSION)
LIBSRT_ARTIFACTS_DIR = $(LIBSRT_DIR)/artifacts
LIBSRT_TAR = $(LIBSRT_DIR)/libsrt.tar.gz
LIBSRT_VERSION ?= 1.5.4
LIBSRT_URL = https://github.com/Haivision/srt/archive/refs/tags/v$(LIBSRT_VERSION).tar.gz
LIBSRT_CMAKE_CONF_OPTS += "	\
	-DENABLE_STDCXX_SYNC=ON \
	-DENABLE_ENCRYPTION=ON \
	-DENABLE_BONDING=ON \
	-DENABLE_SHOW_PROJECT_CONFIG=ON \
	"

# $(call build_libsrt,CC_PREFIX,CMAKE_ADDITIONAL_OPTS,BUILD_DIR,INSTALL_DIR)
define build_libsrt
	cmake \
		$(LIBSRT_CMAKE_CONF_OPTS) \
		-DCMAKE_C_COMPILER=$(1)gcc \
		-DCMAKE_CXX_COMPILER=$(1)g++ \
		-DCMAKE_INSTALL_PREFIX=$(4) \
		$(2) \
		-B $(3) \
		-S $(LIBSRT_SRC_DIR)
	cmake --build $(3) --target install -- -j`nproc`
endef

.PHONY: libsrc_src libsrt_linux_amd64 libsrt_linux_arm64 libsrt_linux_armv6 libsrt_linux_armv7 libsrt_linux

$(LIBSRT_SRC_DIR):
	mkdir -p $(TMP_DIR) $(LIBSRT_DIR) $(LIBSRT_SRC_DIR)
	curl -L -o $(LIBSRT_TAR) $(LIBSRT_URL)
	tar -xzf $(LIBSRT_TAR) -C $(LIBSRT_SRC_DIR) --strip-components=1
	rm $(LIBSRT_TAR)

libsrc_src: $(LIBSRT_SRC_DIR)

libsrt_linux_amd64: libsrc_src
	$(eval BUILD_DIR = $(LIBSRT_DIR)/build-amd64)
	$(call build_libsrt,,,$(BUILD_DIR),$(LIBSRT_ARTIFACTS_DIR)/linux-amd64)

libsrt_linux_arm64: libsrc_src
	$(eval BUILD_DIR = $(LIBSRT_DIR)/build-arm64)
	$(eval CC_PREFIX = aarch64-linux-gnu-)
	$(eval CMAKE_ADDITIONAL_OPTS = -DCMAKE_SYSTEM_PROCESSOR=aarch64)
	$(call build_libsrt,$(CC_PREFIX),$(CMAKE_ADDITIONAL_OPTS),$(BUILD_DIR),$(LIBSRT_ARTIFACTS_DIR)/linux-arm64)

# libsrt_linux_armv6: libsrc_src
# 	$(eval BUILD_DIR = $(LIBSRT_DIR)/build_armv6)
# 	$(eval CC_PREFIX = aarch64-linux-gnu-)
# 	$(eval CMAKE_ADDITIONAL_OPTS = -DCMAKE_SYSTEM_PROCESSOR=armv6)
# 	$(call build_libsrt,$(CC_PREFIX),$(CMAKE_ADDITIONAL_OPTS),$(BUILD_DIR),$(LIBSRT_DIR)/artifacts/armv6)

# libsrt_linux_armv7: libsrc_src
# 	$(eval BUILD_DIR = $(LIBSRT_DIR)/build_armv7)
# 	$(eval CC_PREFIX = aarch64-linux-gnu-)
# 	$(eval CMAKE_ADDITIONAL_OPTS = -DCMAKE_SYSTEM_PROCESSOR=armv7)
# 	$(call build_libsrt,$(CC_PREFIX),$(CMAKE_ADDITIONAL_OPTS),$(BUILD_DIR),$(LIBSRT_DIR)/artifacts/armv7)

libsrt_linux: libsrt_linux_amd64 libsrt_linux_arm64
