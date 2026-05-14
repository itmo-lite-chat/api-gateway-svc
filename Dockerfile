FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api-gateway-svc ./cmd

FROM alpine:3.22

COPY --from=build /out/api-gateway-svc /usr/local/bin/api-gateway-svc

EXPOSE 8080

ENTRYPOINT ["api-gateway-svc"]
