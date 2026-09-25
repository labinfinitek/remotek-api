# Immagine dell'API di Remotek costruita dal sorgente, in tre stadi: binario
# Go, pannello rustdesk-api-web, immagine finale. Le immagini di base sono
# fissate per digest: per aggiornarne una si cambiano tag e digest insieme.
#
#   docker build --build-arg VERSION=<versione> \
#     --build-arg REVISION="$(git rev-parse HEAD)" -t remotek-api .

# --- Binario Go ---
# Il driver sqlite (mattn/go-sqlite3) e' C e vuole CGO. Il binario si linka
# statico contro la glibc dell'immagine golang, che ha gia' gcc: nessun
# pacchetto da installare, e il binario gira su qualunque base. I tag tolgono
# le parti della glibc che non funzionano statiche: netgo e osusergo (DNS e
# utenti in Go puro), sqlite_omit_load_extension (niente dlopen); timetzdata
# mette il database dei fusi nel binario, cosi' TZ funziona senza tzdata.
FROM docker.io/library/golang:1.26.8-trixie@sha256:bdca99a00bc16590cb1a0bb4e698f5fc5d6a64e4d5eef13d9f18a0ee08e5fa65 AS api
ENV CGO_ENABLED=1 GOFLAGS=-mod=readonly GOTOOLCHAIN=local
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -trimpath -buildvcs=false \
      -tags 'netgo osusergo sqlite_omit_load_extension timetzdata' \
      -ldflags '-s -w -linkmode=external -extldflags=-static' \
      -o /out/apimain ./cmd \
 && if readelf -d /out/apimain | grep -q NEEDED; then \
      echo "apimain non e' statico" >&2; exit 1; \
    fi

# --- Pannello ---
# rustdesk-api-web di upstream compilato dal sorgente a un commit fissato
# (sha completo, mai un branch), con npm ci sul package-lock.json di quel
# commit. 3998c2a e' l'ultimo commit di master (2025-08-31), quello che
# impacchettava rustdesk-api v2.7. Il lockfile di upstream risolve tutto su
# registry.npmmirror.com: si scarica dal registry ufficiale, e l'integrity
# sha512 di ogni pacchetto nel lockfile garantisce lo stesso contenuto. Gli
# script di installazione non servono alla build (esbuild, vue-demi,
# protobufjs, fsevents) e non si eseguono.
FROM docker.io/library/node:24.21.0-trixie-slim@sha256:8ec5d7557396cfe32d21c3f9c13072355ceab22b584578ca4bb28af31120cffe AS pannello
ARG PANNELLO_COMMIT=3998c2a9213fcd047252776d0f0db33e6717026c
ADD --keep-git-dir=false https://github.com/lejianwen/rustdesk-api-web.git#${PANNELLO_COMMIT} /pannello
WORKDIR /pannello
RUN case "$PANNELLO_COMMIT" in \
      *[!0-9a-f]*) echo "PANNELLO_COMMIT non e' uno sha" >&2; exit 1 ;; \
    esac \
 && [ "${#PANNELLO_COMMIT}" -eq 40 ] \
 && npm ci --registry=https://registry.npmjs.org/ \
      --replace-registry-host=always --ignore-scripts --no-audit --no-fund \
 && npm run build

# --- Immagine finale ---
FROM docker.io/library/alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.title="remotek-api" \
      org.opencontainers.image.description="API di Remotek, fork di rustdesk-api v2.7" \
      org.opencontainers.image.source="https://github.com/labinfinitek/remotek-api" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.licenses="MIT"
RUN addgroup -S -g 10001 remotek \
 && adduser -S -D -H -h /app -s /sbin/nologin -G remotek -u 10001 remotek
WORKDIR /app
COPY --from=api /out/apimain ./apimain
COPY conf/ ./conf/
COPY resources/ ./resources/
COPY --from=pannello /pannello/dist/ ./resources/admin/
# /api/version legge resources/version (service.AppService); l'a capo finale
# e' quello dell'immagine di v2.7.
RUN echo "$VERSION" > resources/version \
 && mkdir -p data runtime \
 && chown remotek:remotek data runtime
USER 10001:10001
VOLUME /app/data
EXPOSE 21114
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD ["wget", "-q", "-O", "/dev/null", "http://127.0.0.1:21114/api/version"]
CMD ["./apimain"]
