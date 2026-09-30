FROM golang:1.25.0 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.22

RUN adduser -D -H appuser

WORKDIR /app

COPY --from=build /out/server .

USER appuser

ENTRYPOINT ["/app/server"]
