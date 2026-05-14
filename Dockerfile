FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/qwen2api ./cmd/qwen2api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/qwen2api /app/qwen2api
USER nonroot:nonroot
EXPOSE 5001
ENTRYPOINT ["/app/qwen2api"]
