# Nexus - 高性能反向代理和负载均衡器

**Nexus** 是一个用 Go 语言编写的轻量级、高性能的反向代理和负载均衡器。 它旨在提供快速、可靠和可扩展的解决方案，用于管理和路由网络流量。

## 特性

*   **高性能**:  使用 Go 语言和高效的网络编程技术构建，提供卓越的性能和低延迟。
*   **负载均衡**:  支持多种负载均衡算法 (例如轮询、加权轮询、IP Hash 等)，可以根据后端服务器的健康状况和负载情况智能地分发请求。
*   **健康检查**:  内置健康检查机制，定期检测后端服务器的可用性，自动剔除不健康的服务器，确保服务的稳定性和可靠性。
*   **灵活配置**:  使用 YAML 配置文件进行管理，易于配置和维护。 支持动态配置更新，无需重启服务即可应用新的配置。
*   **可扩展性**:  模块化设计，易于扩展和定制。 可以根据需求添加新的功能模块，例如认证、限流、监控等。
*   **易于部署**:  编译成单个可执行文件，部署简单方便。 支持 Docker 部署。
*   **gRPC 支持**:  支持 gRPC 协议的反向代理和负载均衡。

## 快速开始

### 前提条件

*   Go 1.16 或更高版本
*   Protocol Buffer 编译器 (protoc)
*   protoc-gen-go 和 protoc-gen-go-grpc 插件 (用于 gRPC 支持)

### 安装

1.  **克隆仓库:**

    ```bash
    git clone https://github.com/yourusername/nexus.git
    cd nexus
    ```

2.  **构建项目:**

    ```bash
    go build -o nexus cmd/main.go
    ```

    这将在当前目录下生成可执行文件 `nexus`。

### 配置

1.  **复制配置文件:**

    复制 `configs/config.yaml` 文件到您希望运行 `nexus` 的目录，例如当前目录。

    ```bash
    cp configs/config.yaml ./config.yaml
    ```

2.  **编辑配置文件:**

    打开 `config.yaml` 文件，根据您的需求修改配置。  配置文件详细说明请参考 [配置详解](#配置详解) 章节。

    ```yaml
    # config.yaml 示例

    proxy:
      listen_address: ":8080"  # 监听地址
      backend_servers:        # 后端服务器列表
        - address: "192.168.1.100:8081"
          weight: 10
        - address: "192.168.1.101:8081"
          weight: 5
      load_balancer: "round_robin" # 负载均衡算法，可选: round_robin, weighted_round_robin, ip_hash
      health_check:
        enabled: true           # 是否启用健康检查
        interval: "5s"          # 健康检查间隔
        timeout: "2s"           # 健康检查超时时间
        path: "/health"         # 健康检查路径 (HTTP) 或 gRPC 服务方法名 (gRPC)
    ```

### 运行

1.  **启动 Nexus:**

    ```bash
    ./nexus -config config.yaml
    ```

    或者，如果您将配置文件放在默认位置 (`./config.yaml` 或 `/etc/nexus/config.yaml`)，则可以直接运行：

    ```bash
    ./nexus
    ```

2.  **访问代理服务:**

    现在您可以通过配置的监听地址 (例如 `http://localhost:8080`) 访问 Nexus 反向代理服务了。  请求将被负载均衡到后端服务器。

## 配置详解

`config.yaml` 文件用于配置 Nexus 反向代理和负载均衡器的行为。  以下是配置文件的详细说明：

```yaml
proxy:
  listen_address: ":8080" # (必填) 监听地址，例如 ":8080", "0.0.0.0:80", "[::]:8080"
  backend_servers: # (必填) 后端服务器列表
    - address: "192.168.1.100:8081" # 后端服务器地址，格式为 "host:port"
      weight: 10 # (可选) 权重，用于加权轮询负载均衡，默认为 1
    - address: "192.168.1.101:8081"
      weight: 5
  load_balancer: "round_robin" # (可选) 负载均衡算法，可选值:
# - "round_robin": 轮询 (默认)
# - "weighted_round_robin": 加权轮询
# - "ip_hash": IP Hash
health_check: # (可选) 健康检查配置
enabled: true # 是否启用健康检查，默认为 false
interval: "5s" # 健康检查间隔，例如 "5s", "1m", "300ms"，默认为 "5s"
timeout: "2s" # 健康检查超时时间，例如 "2s", "1s", "500ms"，默认为 "2s"
path: "/health" # 健康检查路径 (HTTP) 或 gRPC 服务方法名 (gRPC)，默认为 "/health"
# - HTTP 健康检查: Nexus 将发送 HTTP GET 请求到后端服务器的 /health 路径。
# - gRPC 健康检查: Nexus 将调用 gRPC 健康检查服务的 Check 方法，方法名为 path 指定的值。
protocol: "http" # 健康检查协议，可选值: "http", "grpc"，默认为 "http"
logger: # (可选) 日志配置
level: "info" # 日志级别，可选值: "debug", "info", "warn", "error", "fatal"，默认为 "info"
format: "text" # 日志格式，可选值: "text", "json"，默认为 "text"
output: "stdout" # 日志输出目标，可选值: "stdout", "stderr", "file"，默认为 "stdout"
filename: "nexus.log" # 当 output 为 "file" 时，指定日志文件路径，默认为 "nexus.log" (仅当 output 为 "file" 时生效)
```

## 目录结构

```
nexus/
├── .gitignore
├── cmd/                    # 包含了项目的可执行文件
│   └── main.go             # 主程序入口
├── configs/
│   └── config.yaml         # 配置文件，用于配置代理服务器
├── internal/
│   ├── balancer.go         # 负载均衡器实现
│   ├── config.go           # 配置管理
│   ├── healthcheck.go      # 健康检查实现
│   ├── logger.go           # 日志实现
│   └── proxy.go            # 代理实现
├── pb/                     # 包含了protobuf定义和生成的代码
│   ├── nexus.pb.go
│   └── nexus_grpc.pb.go
├── test/
│   ├── balancer_test.go    # 负载均衡器测试
│   ├── benchmark_test.go   # 性能基准测试
│   ├── config_test.go      # 配置加载测试
│   ├── healthcheck_test.go # 健康检查测试
│   ├── integration_test.go # 集成测试
│   ├── logger_test.go      # 日志记录测试
│   ├── proxy_test.go       # 反向代理测试
│   └── stress_test.go      # 压力测试
├── go.mod
└── go.sum
```

## 使用示例

### HTTP 反向代理和负载均衡

配置 `config.yaml` 文件如下：
```yml
proxy:
  listen_address: ":8080"
  backend_servers:
    - address: "192.168.1.100:8081"
    - address: "192.168.1.101:8081"
  load_balancer: "round_robin"
  health_check:
    enabled: true
path: "/health"
protocol: "http"
```

启动 Nexus 后，所有发送到 `http://localhost:8080` 的 HTTP 请求将被轮询负载均衡到 `192.168.1.100:8081` 和 `192.168.1.101:8081` 这两台后端服务器。  Nexus 会定期检查后端服务器的 `/health` 路径，确保只将请求发送到健康的服务器。

### gRPC 反向代理和负载均衡

配置 `config.yaml` 文件如下：
```yaml
proxy:
  listen_address: ":8080"
  backend_servers:
    - address: "192.168.1.100:8081"
    - address: "192.168.1.101:8081"
  load_balancer: "weighted_round_robin"
  health_check:
    enabled: true
path: "grpc.health.v1.Health/Check" # gRPC 健康检查服务方法名
protocol: "grpc"
```

启动 Nexus 后，所有发送到 `localhost:8080` 的 gRPC 请求将被加权轮询负载均衡到后端 gRPC 服务器。  Nexus 会使用 gRPC 健康检查服务 (`grpc.health.v1.Health/Check`) 检查后端服务器的健康状态。

## 贡献

欢迎任何形式的贡献！  如果您想为此项目做出贡献，请遵循以下步骤：

1.  Fork 本仓库。
2.  创建您的 Feature 分支 (`git checkout -b feature/your-feature`)。
3.  提交您的更改 (`git commit -am 'Add some feature'`)。
4.  Push 到 Feature 分支 (`git push origin feature/your-feature`)。
5.  创建新的 Pull Request。

请确保您的代码风格与项目现有代码保持一致，并添加相应的单元测试。

## 许可证

MIT
