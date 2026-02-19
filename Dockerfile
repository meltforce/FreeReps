# Stage 1: Build frontend
FROM node:22-alpine AS frontend
WORKDIR /build
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.23-alpine AS backend
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/dist ./web/dist
RUN CGO_ENABLED=0 go build -o freereps ./cmd/freereps

# Stage 3: Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /build/freereps .
COPY --from=backend /build/migrations ./migrations
EXPOSE 8080
ENTRYPOINT ["./freereps"]
