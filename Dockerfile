FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go run github.com/a-h/templ/cmd/templ@v0.3.1020 generate && CGO_ENABLED=0 go build -o /railnow ./cmd/server
FROM alpine:3.21
WORKDIR /app
COPY --from=build /railnow ./railnow
COPY --from=build /app/data ./data
COPY --from=build /app/db/migrations ./db/migrations
COPY --from=build /app/public ./public
EXPOSE 8080
ENV PORT=8080
CMD ["./railnow"]
