FROM  node:18.18.0 AS builder

WORKDIR /web
COPY ./VERSION .
COPY ./web .

# 安装依赖并修复 ajv 问题
RUN cd /web/default && npm install --legacy-peer-deps --force && \
    npm install ajv@^8.0.0 --save-dev --force
RUN cd /web/berry && npm install --legacy-peer-deps --force && \
    npm install ajv@^8.0.0 --save-dev --force
RUN cd /web/air && npm install --legacy-peer-deps --force && \
    npm install ajv@^8.0.0 --save-dev --force

# 构建前端
RUN cd /web/default && DISABLE_ESLINT_PLUGIN='true' REACT_APP_VERSION=$(cat /web/VERSION) npm run build
RUN cd /web/berry && DISABLE_ESLINT_PLUGIN='true' REACT_APP_VERSION=$(cat /web/VERSION) npm run build
RUN cd /web/air && DISABLE_ESLINT_PLUGIN='true' REACT_APP_VERSION=$(cat /web/VERSION) npm run build

FROM golang:alpine AS builder2

RUN apk add --no-cache \
    gcc \
    musl-dev \
    sqlite-dev \
    build-base

ENV GO111MODULE=on \
    CGO_ENABLED=1 \
    GOOS=linux

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=builder /web/build ./web/build

RUN go build -trimpath -ldflags "-s -w -X 'github.com/songquanpeng/one-api/common.Version=$(cat VERSION)' -linkmode external -extldflags '-static'" -o one-api

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder2 /build/one-api /

EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/one-api"]