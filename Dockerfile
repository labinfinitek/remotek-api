# Immagine dell'API di Remotek costruita dal sorgente, in tre stadi:
# binario Go, build del pannello (pannello/), immagine finale. Le immagini
# di base sono fissate per digest: per aggiornarne una si cambiano tag e
# digest insieme.
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
FROM docker.io/library/golang:1.26.10-trixie@sha256:909771fef53e3dcb0950255f0c9a8b4728c3b841e61509c7b29b098de7ee02d4 AS api
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
# Sorgente in pannello/: rustdesk-api-web di lejianwen al commit 3998c2a,
# con le nostre modifiche (ADR-0027, pannello/README.md). npm ci prima di
# copiare il resto del sorgente: cambiare un file del pannello non riscarica
# i pacchetti. Il lockfile risolve tutto sul registry ufficiale, e la build
# si ferma se un `resolved` punta altrove (un mirror non passa inosservato);
# l'integrity sha512 di ogni pacchetto garantisce il contenuto, e --registry
# lo dice anche a npm audit. Node e' quello di pannello/.nvmrc, e la build si
# ferma se l'immagine ne ha un altro. Un avviso HIGH o CRITICAL di npm audit
# ferma la build (REGOLE sez. 9): si aggiorna il pacchetto, non si salta
# l'audit. Gli script di installazione non servono alla build (esbuild,
# vue-demi, fsevents, @parcel/watcher) e non si eseguono.
FROM docker.io/library/node:24.21.0-trixie-slim@sha256:8ec5d7557396cfe32d21c3f9c13072355ceab22b584578ca4bb28af31120cffe AS pannello
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
WORKDIR /pannello
COPY pannello/package.json pannello/package-lock.json pannello/.nvmrc ./
RUN test "$(node --version)" = "v$(cat .nvmrc)" \
      || { echo "node $(node --version), .nvmrc chiede v$(cat .nvmrc)" >&2; exit 1; } \
 && if grep '"resolved":' package-lock.json | grep -vF '"resolved": "https://registry.npmjs.org/' >&2; then \
      echo "package-lock.json: resolved fuori da https://registry.npmjs.org/" >&2; exit 1; \
    fi \
 && npm ci --registry=https://registry.npmjs.org/ --ignore-scripts --no-fund \
 && npm audit --audit-level=high --registry=https://registry.npmjs.org/
COPY pannello/ ./
# Titolo statico del pannello: nel sorgente e' Remotek, BRAND_NAME lo
# cambia; quello vero lo mette il pannello leggendo
# /api/admin/config/admin. Logo e favicon vengono da /brand/, che l'API
# serve da brand.dir, e non dipendono da BRAND_NAME. La build si ferma se
# una delle due righe non c'e' piu'.
ARG BRAND_NAME=Remotek
RUN case "$BRAND_NAME" in \
      ""|*[!A-Za-z0-9._\ -]*) echo "BRAND_NAME: solo lettere, cifre, spazi, . _ -" >&2; exit 1 ;; \
    esac \
 && titolo() { grep -qF "$2" "$1" && sed -i "s|$2|$3|" "$1" && grep -qF "$3" "$1" \
      || { echo "$1: manca la riga del titolo: $2" >&2; return 1; }; } \
 && titolo index.html '<title>Remotek</title>' "<title>$BRAND_NAME</title>" \
 && titolo src/store/app.js "title: 'Remotek'," "title: '$BRAND_NAME',"
# Prima del build: it.json ed en.json allineati e nessuna chiave usata che
# manchi.
RUN node scripts/verifica-lingue.mjs \
 && npm run build

# --- Immagine finale ---
FROM docker.io/library/alpine:3.24.3@sha256:b3413572228e34bd1c9245f5565e6cd127005655e8a7899aef33d8e83255c9e6
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
# Il pannello e' MIT: la sua licenza va con i suoi file (pannello/README.md).
COPY pannello/LICENSE ./resources/admin/LICENSE
# I caratteri del pannello (Inter, Space Grotesk) sono OFL-1.1: la licenza
# va con i file dei caratteri.
COPY --from=pannello /pannello/node_modules/@fontsource/inter/LICENSE ./resources/admin/LICENSE-inter
COPY --from=pannello /pannello/node_modules/@fontsource/space-grotesk/LICENSE ./resources/admin/LICENSE-space-grotesk
# /api/version legge resources/version (service.AppService); l'a capo finale
# e' quello dell'immagine di v2.7.
RUN echo "$VERSION" > resources/version \
 && mkdir -p data \
 && chown remotek:remotek data
USER 10001:10001
VOLUME /app/data
EXPOSE 21114
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
  CMD ["wget", "-q", "-O", "/dev/null", "http://127.0.0.1:21114/api/version"]
CMD ["./apimain"]
