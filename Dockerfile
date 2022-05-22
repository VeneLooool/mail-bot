FROM golang:1.18.2-alpine3.15
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o server ./cmd/server


FROM alpine:latest
WORKDIR /app
COPY --from=0 /app/server ./ /
COPY --from=0 /app/config ./config/
EXPOSE 8080
EXPOSE 8081
ENTRYPOINT ["./server"]
