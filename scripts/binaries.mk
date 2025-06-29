BINARY_NAME = mediamtx

define DOCKERFILE_BINARIES
FROM $(BASE_IMAGE) AS build-base
RUN apk add --no-cache zip make git tar build-base cmake pkgconfig openssl-dev
WORKDIR /s

# Build libsrt statically for all platforms
RUN git clone https://github.com/Haivision/srt.git /tmp/srt && \
    cd /tmp/srt && \
    git checkout v1.5.3 && \
    mkdir build && cd build && \
    cmake .. -DCMAKE_BUILD_TYPE=Release \
             -DENABLE_STATIC=ON \
             -DENABLE_SHARED=OFF \
             -DENABLE_APPS=OFF \
             -DENABLE_LOGGING=OFF \
             -DUSE_STATIC_LIBSTDCXX=ON \
             -DCMAKE_INSTALL_PREFIX=/usr/local && \
    make -j$(nproc) && \
    make install

COPY go.mod go.sum ./
RUN go mod download
COPY . ./
ENV CGO_ENABLED=1
ENV PKG_CONFIG_PATH=/usr/local/lib/pkgconfig
RUN rm -rf tmp binaries
RUN mkdir tmp binaries
RUN cp mediamtx.yml LICENSE tmp/
RUN go generate ./...

# Cross-compilation setup for different architectures
FROM build-base AS build-cross-base
RUN apk add --no-cache gcc-aarch64-linux-gnu gcc-arm-linux-gnueabihf gcc-x86_64-linux-gnu

# Build libsrt for each target architecture
FROM build-cross-base AS build-srt-linux-amd64
RUN cd /tmp/srt && rm -rf build && mkdir build && cd build && \
    CC=x86_64-linux-gnu-gcc CXX=x86_64-linux-gnu-g++ \
    cmake .. -DCMAKE_BUILD_TYPE=Release \
             -DENABLE_STATIC=ON \
             -DENABLE_SHARED=OFF \
             -DENABLE_APPS=OFF \
             -DENABLE_LOGGING=OFF \
             -DUSE_STATIC_LIBSTDCXX=ON \
             -DCMAKE_SYSTEM_NAME=Linux \
             -DCMAKE_SYSTEM_PROCESSOR=x86_64 \
             -DCMAKE_C_COMPILER=x86_64-linux-gnu-gcc \
             -DCMAKE_CXX_COMPILER=x86_64-linux-gnu-g++ \
             -DCMAKE_INSTALL_PREFIX=/usr/local/linux-amd64 && \
    make -j$(nproc) && \
    make install

FROM build-srt-linux-amd64 AS build-linux-amd64
ENV GOOS=linux GOARCH=amd64
ENV CC=x86_64-linux-gnu-gcc
ENV CXX=x86_64-linux-gnu-g++
ENV PKG_CONFIG_PATH=/usr/local/linux-amd64/lib/pkgconfig
ENV CGO_LDFLAGS="-static"
RUN go build -ldflags="-linkmode external -extldflags '-static'" -o "tmp/$(BINARY_NAME)"
RUN tar -C tmp -czf "binaries/$(BINARY_NAME)_$$(cat internal/core/VERSION)_linux_amd64.tar.gz" --owner=0 --group=0 "$(BINARY_NAME)" mediamtx.yml LICENSE

# Build libsrt for ARM64
FROM build-cross-base AS build-srt-linux-arm64
RUN cd /tmp/srt && rm -rf build && mkdir build && cd build && \
    CC=aarch64-linux-gnu-gcc CXX=aarch64-linux-gnu-g++ \
    cmake .. -DCMAKE_BUILD_TYPE=Release \
             -DENABLE_STATIC=ON \
             -DENABLE_SHARED=OFF \
             -DENABLE_APPS=OFF \
             -DENABLE_LOGGING=OFF \
             -DUSE_STATIC_LIBSTDCXX=ON \
             -DCMAKE_SYSTEM_NAME=Linux \
             -DCMAKE_SYSTEM_PROCESSOR=aarch64 \
             -DCMAKE_C_COMPILER=aarch64-linux-gnu-gcc \
             -DCMAKE_CXX_COMPILER=aarch64-linux-gnu-g++ \
             -DCMAKE_INSTALL_PREFIX=/usr/local/linux-arm64 && \
    make -j$(nproc) && \
    make install

FROM build-srt-linux-arm64 AS build-linux-arm64
ENV GOOS=linux GOARCH=arm64
ENV CC=aarch64-linux-gnu-gcc
ENV CXX=aarch64-linux-gnu-g++
ENV PKG_CONFIG_PATH=/usr/local/linux-arm64/lib/pkgconfig
ENV CGO_LDFLAGS="-static"
RUN go build -ldflags="-linkmode external -extldflags '-static'" -o "tmp/$(BINARY_NAME)"
RUN tar -C tmp -czf "binaries/$(BINARY_NAME)_$$(cat internal/core/VERSION)_linux_arm64.tar.gz" --owner=0 --group=0 "$(BINARY_NAME)" mediamtx.yml LICENSE

# Build libsrt for ARMv7
FROM build-cross-base AS build-srt-linux-armv7
RUN cd /tmp/srt && rm -rf build && mkdir build && cd build && \
    CC=arm-linux-gnueabihf-gcc CXX=arm-linux-gnueabihf-g++ \
    cmake .. -DCMAKE_BUILD_TYPE=Release \
             -DENABLE_STATIC=ON \
             -DENABLE_SHARED=OFF \
             -DENABLE_APPS=OFF \
             -DENABLE_LOGGING=OFF \
             -DUSE_STATIC_LIBSTDCXX=ON \
             -DCMAKE_SYSTEM_NAME=Linux \
             -DCMAKE_SYSTEM_PROCESSOR=arm \
             -DCMAKE_C_COMPILER=arm-linux-gnueabihf-gcc \
             -DCMAKE_CXX_COMPILER=arm-linux-gnueabihf-g++ \
             -DCMAKE_INSTALL_PREFIX=/usr/local/linux-armv7 && \
    make -j$(nproc) && \
    make install

FROM build-srt-linux-armv7 AS build-linux-armv7
ENV GOOS=linux GOARCH=arm GOARM=7
ENV CC=arm-linux-gnueabihf-gcc
ENV CXX=arm-linux-gnueabihf-g++
ENV PKG_CONFIG_PATH=/usr/local/linux-armv7/lib/pkgconfig
ENV CGO_LDFLAGS="-static"
RUN go build -ldflags="-linkmode external -extldflags '-static'" -o "tmp/$(BINARY_NAME)"
RUN tar -C tmp -czf "binaries/$(BINARY_NAME)_$$(cat internal/core/VERSION)_linux_armv7.tar.gz" --owner=0 --group=0 "$(BINARY_NAME)" mediamtx.yml LICENSE

FROM build-srt-linux-armv7 AS build-linux-armv6
ENV GOOS=linux GOARCH=arm GOARM=6
ENV CC=arm-linux-gnueabihf-gcc
ENV CXX=arm-linux-gnueabihf-g++
ENV PKG_CONFIG_PATH=/usr/local/linux-armv7/lib/pkgconfig
ENV CGO_LDFLAGS="-static"
RUN go build -ldflags="-linkmode external -extldflags '-static'" -o "tmp/$(BINARY_NAME)"
RUN tar -C tmp -czf "binaries/$(BINARY_NAME)_$$(cat internal/core/VERSION)_linux_armv6.tar.gz" --owner=0 --group=0 "$(BINARY_NAME)" mediamtx.yml LICENSE

# Note: Windows and macOS builds are disabled for now due to CGO cross-compilation complexity
# They would require additional toolchains and more complex setup

FROM $(BASE_IMAGE)
COPY --from=build-linux-amd64 /s/binaries /s/binaries
COPY --from=build-linux-armv6 /s/binaries /s/binaries
COPY --from=build-linux-armv7 /s/binaries /s/binaries
COPY --from=build-linux-arm64 /s/binaries /s/binaries
endef
export DOCKERFILE_BINARIES

binaries:
	echo "$$DOCKERFILE_BINARIES" | docker build . -f - \
	-t temp
	docker run --rm -v "$(shell pwd):/out" \
	temp sh -c "rm -rf /out/binaries && cp -r /s/binaries /out/"
	sudo chown -R $(shell id -u):$(shell id -g) binaries
