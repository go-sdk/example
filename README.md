# go-sdk example

这是一个简易 Go 应用模板，集中演示以下三个基础包的组合方式：

- `github.com/go-sdk/core`：配置、日志、生命周期、命令行、ID 和错误处理。
- `github.com/go-sdk/database`：PostgreSQL、GORM 日志、软删除和版本迁移。
- `github.com/go-sdk/server`：单端口 gRPC/Gateway、统一响应、JWT、参数校验和 Recovery。

模板包含用户、角色、权限 RBAC，以及文件上传、下载和用户头像替换接口。
Proto 中使用的 server 方法和字段选项直接依赖 `buf.build/go-sdk/server`。

## 环境要求

- Go 1.27+
- Buf 1.73+
- Docker 和 Docker Compose

本地生成 Proto 还需要 `protoc-gen-go`、`protoc-gen-go-grpc`、
`protoc-gen-grpc-gateway` 和 `protoc-gen-openapiv2`，可通过 `make prepare` 安装。

## 快速启动

复制环境变量模板并把三个占位值替换为随机的本地开发凭据：

```bash
cp .env.example .env
docker compose up --build
```

应用和 PostgreSQL 由 Compose 管理，API 默认监听 `http://127.0.0.1:8080`。PostgreSQL 数据和
上传文件分别保存在命名卷 `postgres-data` 和 `app-data` 中。

首次启动会执行数据库迁移，创建内置权限和 `admin` 角色，并根据以下配置创建管理员：

- 用户名：`bootstrap.username`，默认 `admin`
- 邮箱：`bootstrap.email`
- 密码：环境变量 `APP__BOOTSTRAP__PASSWORD`

管理员已存在时不会覆盖密码或其他资料。

## API 模型

普通业务接口定义在 `proto/app/v1`，同时支持原生 gRPC 和 HTTP JSON：

| 模块     | HTTP 路径                       | 权限                     |
|----------|---------------------------------|--------------------------|
| 登录     | `POST /api/v1/auth/login`       | 匿名                     |
| 用户     | `/api/v1/users`                 | `users.read/write`       |
| 角色     | `/api/v1/roles`                 | `roles.read/write`       |
| 权限     | `/api/v1/permissions`           | `permissions.read/write` |
| 上传     | `POST /api/v1/files`            | `files.write`            |
| 下载     | `GET /api/v1/files/{id}`        | `files.read`             |
| 替换头像 | `PUT /api/v1/users/{id}/avatar` | `users.write`            |

Gateway 成功响应由 server 统一包装：

```json
{"data":{"id":"123"}}
```

文件接口使用 `multipart/form-data`，文件字段名固定为 `file`。默认最大请求大小为 10 MiB。

登录示例：

```bash
curl -sS http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  --data "{\"username\":\"admin\",\"password\":\"${BOOTSTRAP_ADMIN_PASSWORD}\"}"
```

上传示例，其中 `ACCESS_TOKEN` 是登录接口返回的 JWT：

```bash
curl -sS http://127.0.0.1:8080/api/v1/files \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -F 'file=@./example.png'
```

生成的 Swagger 2.0 文档位于 `openapi/openapi.swagger.yaml`，其 200 响应的 schema 描述
`data` 字段内的消息结构。特殊文件接口由 `standard.Server.HandlePath` 注册，不在生成文档内。

## 配置

默认配置位于 `config.yaml`，也可以通过 `CONFIG_PATH` 指定另一份 YAML 或 JSON 配置文件。
`core/config` 使用 `APP__` 前缀和双下划线覆盖嵌套配置：

| 配置                       | 环境变量                         | 说明           |
|----------------------------|----------------------------------|----------------|
| `server.address`           | `APP__SERVER__ADDRESS`           | 监听地址       |
| `database.dsn`             | `APP__DATABASE__DSN`             | PostgreSQL DSN |
| `auth.jwt_secret`          | `APP__AUTH__JWT_SECRET`          | 至少 32 个字符 |
| `auth.expires_in`          | `APP__AUTH__EXPIRES_IN`          | Go duration    |
| `storage.root`             | `APP__STORAGE__ROOT`             | 文件存储目录   |
| `storage.max_upload_bytes` | `APP__STORAGE__MAX_UPLOAD_BYTES` | 上传请求上限   |
| `bootstrap.password`       | `APP__BOOTSTRAP__PASSWORD`       | 首次管理员密码 |

不要把真实 DSN、JWT 密钥或管理员密码写入 `config.yaml` 或提交到 Git。

## 开发命令

```bash
make generate        # Buf lint，并生成 Protobuf、Gateway 和 OpenAPI
make lint            # go mod tidy 和 golangci-lint
make build           # 纯编译到 bin/app
```

`make generate` 会重新创建 `gen/` 和 `openapi/`。生成代码和协议必须在同一提交中保持同步。

## 安全与边界

- JWT 使用 HS256，生产环境必须注入足够随机的独立密钥并使用 TLS 入口。
- 密码使用 bcrypt 保存，Proto 将密码和 Token 标记为敏感字段，登录方法跳过 Payload 日志。
- RBAC 在每次请求时查询数据库，权限关系修改立即生效。
- 文件名不会直接作为磁盘路径；磁盘使用随机名称，原始文件名只作为元数据保存。
- 当前文件存储适合单机模板。多副本部署应替换为对象存储，并补充恶意文件检测、配额和清理任务。
- 自动迁移和静态编译不能代替真实 PostgreSQL、鉴权和文件上传的运行时验证。
