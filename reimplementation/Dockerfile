FROM golang:1.27.4-bookworm AS build
ARG BUILD_VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY contracts ./contracts
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/specter-server ./cmd/server
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/specter-worker ./cmd/worker
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/specter-object-migrate ./cmd/object-migrate
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w -buildid= -X main.buildVersion=${BUILD_VERSION}" -o /out/specter-beacon ./cmd/beacon

FROM build AS test
COPY tests ./tests
COPY migrations ./migrations
RUN go test ./...

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl openssl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --create-home specter
WORKDIR /app
COPY --from=build /out/specter-server /app/specter-server
COPY --from=build /out/specter-worker /app/specter-worker
COPY --from=build /out/specter-object-migrate /app/specter-object-migrate
COPY --from=build /out/specter-beacon /app/specter-beacon
COPY migrations /app/migrations
USER 10001
EXPOSE 8080
ENTRYPOINT ["/app/specter-server"]
