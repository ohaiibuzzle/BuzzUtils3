FROM golang:1.27-trixie AS builder
RUN apt-get update && apt-get install -y --no-install-recommends unzip pkg-config && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
COPY scripts/install-libdave.sh scripts/
# libdave (voice encryption), into ~/.local with a pkg-config file for cgo
RUN go mod download && sh scripts/install-libdave.sh
ENV PKG_CONFIG_PATH=/root/.local/lib/pkgconfig
COPY . .
RUN CGO_ENABLED=1 go build -o buzzutils3 ./src/main.go

FROM debian:trixie-slim
ARG TARGETARCH
# ffmpeg encodes the music; yt-dlp finds it, and needs Deno for YouTube. yt-dlp is
# fetched fresh on every build, as it has to keep up with YouTube's changes.
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg ca-certificates curl unzip && \
    case "$TARGETARCH" in \
        arm64) YTDLP=yt-dlp_linux_aarch64; DENO=aarch64-unknown-linux-gnu ;; \
        *) YTDLP=yt-dlp_linux; DENO=x86_64-unknown-linux-gnu ;; \
    esac && \
    curl -fsSL -o /usr/local/bin/yt-dlp "https://github.com/yt-dlp/yt-dlp/releases/latest/download/$YTDLP" && \
    chmod +x /usr/local/bin/yt-dlp && \
    curl -fsSL -o /tmp/deno.zip "https://github.com/denoland/deno/releases/latest/download/deno-$DENO.zip" && \
    unzip -o /tmp/deno.zip -d /usr/local/bin && rm /tmp/deno.zip && \
    apt-get purge -y curl unzip && apt-get autoremove -y && rm -rf /var/lib/apt/lists/*
COPY --from=builder /root/.local/lib/libdave.so /usr/local/lib/
RUN ldconfig

WORKDIR /app
COPY --from=builder /src/buzzutils3 .
# yt-dlp keeps a cache of YouTube's player code; the nonroot user has no home
ENV XDG_CACHE_HOME=/tmp/cache
USER 65532:65532
VOLUME /app/runtime
CMD ["./buzzutils3"]
