# Build frontend
FROM node:22-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --omit=dev=false
COPY frontend/ ./
RUN npm run build

# Build backend
FROM golang:1.25-alpine AS backend-build
WORKDIR /app
RUN apk add --no-cache git
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend-build /app/frontend/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /collaborator ./cmd/server

# Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 1000 collaborator \
    && adduser -S -u 1000 -G collaborator -h /home/collaborator collaborator
WORKDIR /app
COPY --from=backend-build /collaborator /app/collaborator
COPY --from=backend-build /app/web/dist /app/web/dist
RUN chown -R collaborator:collaborator /app
USER collaborator
EXPOSE 53/udp 53/tcp 80/tcp 443/tcp 25/tcp 587/tcp
ENTRYPOINT ["/app/collaborator"]
