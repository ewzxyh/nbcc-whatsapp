FROM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /nbcc .
RUN mkdir /data && chown 65532:65532 /data

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /nbcc /nbcc
COPY --from=build --chown=65532:65532 /data /data
COPY --from=web /web/dist /web
USER 65532:65532
ENV STATE_PATH=/data/state.json
ENV WEB_DIR=/web
EXPOSE 8080
ENTRYPOINT ["/nbcc"]
