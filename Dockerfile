# Builds mergelock and runs its MCP server on stdio.
#   docker build -t mergelock .
#   docker run -i --rm -v "$PWD":/repo mergelock          # MCP server for the mounted repo
#   docker run --rm -v "$PWD":/repo mergelock check my-branch main --format text
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=1 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /mergelock ./cmd/mergelock

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && git config --system --add safe.directory '*'
COPY --from=build /mergelock /usr/local/bin/mergelock
WORKDIR /repo
ENTRYPOINT ["mergelock"]
CMD ["serve"]
