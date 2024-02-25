# syntax=docker/dockerfile:1

FROM golang:1.21.3-alpine3.18

WORKDIR /app
COPY . ./
WORKDIR /app/api_rest
RUN CGO_ENABLED=0 GOOS=linux go build -o /api_rest
CMD ["/api_rest"]
