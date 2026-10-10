FROM golang:1.26

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /usr/local/bin/api ./cmd/api

EXPOSE 8080
CMD ["api"]
