# 子 agent 容器镜像(runner=docker 时用)。多阶段:编译 my-agent → 塞进精简 base。
# 构建: docker build -t my-agent:latest .
# DockerRunner 会以 `/app/my-agent subagent --role .. --task ..` 调起它。
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/my-agent .

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/my-agent /app/my-agent
ENTRYPOINT []
