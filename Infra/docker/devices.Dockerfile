FROM golang:1.25

# Air (mais robusto)
RUN go install github.com/air-verse/air@latest

WORKDIR /app

# Copia somente go.mod/go.sum primeiro para cache
COPY Apps/device_app/go.mod Apps/device_app/go.sum* ./
RUN go mod download

# Copia o restante
COPY Apps/device_app ./

CMD ["air", "-c", ".air.toml"]
