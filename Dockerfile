FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bot ./cmd/api

FROM alpine:3

RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /bot /bot

USER 10001:10001
ENTRYPOINT ["/bot"]
