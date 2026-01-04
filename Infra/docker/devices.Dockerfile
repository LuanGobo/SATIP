FROM golang:1.25

RUN go install github.com/air-verse/air@latest

WORKDIR /app

COPY Apps/device_app/go.mod Apps/device_app/go.sum* ./

RUN go mod download

COPY Apps/device_app .

EXPOSE 8080

CMD ["air", "-c", ".air.toml"]