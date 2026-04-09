FROM golang:1.25 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /st8d ./cmd/st8d

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /st8d /usr/local/bin/st8d

VOLUME ["/data"]
EXPOSE 8748

ENTRYPOINT ["/usr/local/bin/st8d"]
CMD ["--listen=:8748", "--state-dir=/data"]
