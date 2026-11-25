FROM golang:1.27.4-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY contracts ./contracts
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/specter-server ./cmd/server
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/specter-worker ./cmd/worker

FROM build AS test
RUN go test ./...

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home specter
WORKDIR /app
COPY --from=build /out/specter-server /app/specter-server
COPY --from=build /out/specter-worker /app/specter-worker
COPY migrations /app/migrations
USER 10001
EXPOSE 8080
ENTRYPOINT ["/app/specter-server"]
