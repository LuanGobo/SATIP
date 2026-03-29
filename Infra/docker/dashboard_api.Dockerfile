# Estágio 1: Build do Simulador
FROM golang:1.23-alpine AS sim-builder
WORKDIR /app
COPY Apps/device_app/go.mod Apps/device_app/go.sum* ./
RUN go mod download
COPY Apps/device_app ./
RUN go build -o simulator cmd/simulator/main.go

# Estágio 2: Build da API
FROM golang:1.23-alpine AS api-builder
WORKDIR /app
COPY Apps/dashboard/api/go.mod ./
# Se houver go.sum, copie também
RUN go mod download
COPY Apps/dashboard/api ./
RUN go build -o dashboard-api main.go

# Estágio Final
FROM alpine:latest
RUN apk add --no-cache libc6-compat
WORKDIR /app
COPY --from=sim-builder /app/simulator .
COPY --from=sim-builder /app/cmd/simulator/plant.yaml ./cmd/simulator/plant.yaml
COPY --from=api-builder /app/dashboard-api .

EXPOSE 8080
CMD ["./dashboard-api"]
