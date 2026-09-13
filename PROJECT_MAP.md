# 项目地图

## 目录结构

```text
example/
├── cmd/app/main.go                    命令入口、依赖装配和生命周期
├── gen/                               Protobuf、gRPC 与 Gateway 生成代码
├── internal/auth/                     JWT 签发和 RBAC 权限拦截器
├── internal/config/                   配置加载与启动参数校验
├── internal/migration/                数据表迁移、内置权限和管理员初始化
├── internal/model/                    用户、角色、权限、关联表和文件模型
├── internal/service/                  Proto 服务实现
├── internal/transport/http/           上传、下载和头像替换接口
├── openapi/openapi.swagger.yaml       生成的 HTTP API 文档
├── proto/app/v1/                      业务服务和消息定义
├── proto/common/v1/                   分页、元数据和 OpenAPI 公共定义
├── config.yaml                        非秘密默认配置
├── docker-compose.yaml                应用和 PostgreSQL 编排
├── Dockerfile                         两阶段应用镜像
└── Makefile                           生成、检查和编译入口
```

Proto 直接通过 `buf.build/go-sdk/server` 引用 server 的方法与字段选项，生成代码使用
`github.com/go-sdk/server/options` 提供的运行时扩展描述符，不在本仓库维护副本。

## 启动链路

```text
cmd/app serve
  -> core/config 加载 config.yaml 和 APP__ 环境变量
  -> core/logx 设置进程日志字段
  -> database/dbx.Open 按需打开 PostgreSQL 并注册关闭函数
  -> database/migrate 注册带 advisory lock 的迁移初始化函数
  -> 初始化内置权限、admin 角色和首次管理员
  -> server/standard.New 注册 gRPC、Gateway、JWT、RBAC 和错误转换
  -> 注册额外文件 HTTP 路由
  -> core/lifex.Init 按顺序执行迁移、初始化和 Server.Start
  -> core/lifex.Wait 处理信号并逆序释放 Server 和数据库
```

## 请求链路

普通 HTTP 请求：

```text
HTTP /api/v1/*
  -> grpc-gateway
  -> server 请求上下文、日志、JWT、Protovalidate
  -> 应用 RBAC interceptor 查询 user_roles 和 role_permissions
  -> internal/service
  -> GORM with context
  -> database logger 继承 trace-id、span-id 和 depth
```

原生 gRPC 请求经过相同的 interceptor 链。登录方法通过 Proto 选项跳过 JWT 和 Payload
日志；密码和返回 Token 还标记为敏感字段。

文件请求：

```text
HandlePath
  -> server HTTP 请求上下文、JWT、访问日志和 Recovery
  -> FileHandler 显式检查 files.read、files.write 或 users.write
  -> 文件系统保存随机文件名
  -> files 表保存元数据
```

头像替换先保存新文件，再在事务中创建文件记录并切换 `users.avatar_file_id`。事务失败时删除
新文件；事务成功后尽力软删除并清理旧头像。异常退出可能留下孤立文件，生产系统应增加定期
清理任务或改用具备事务补偿能力的对象存储。

## 数据关系

```text
users --< user_roles >-- roles --< role_permissions >-- permissions
  |
  +-- avatar_file_id -------------------------------> files
```

用户、角色、权限和文件使用 `dbx.Metadata`，删除为毫秒时间戳软删除；两个关联表使用复合主键。

## 权限约定

内置权限包括：

- `users.read`、`users.write`
- `roles.read`、`roles.write`
- `permissions.read`、`permissions.write`
- `files.read`、`files.write`

每个受保护 RPC 声明所需权限。RBAC 拦截器按请求实时查询数据库，因此角色或权限关系修改后
立即生效，无需等待 JWT 过期。JWT 只携带用户 ID 和用户名，不缓存权限列表。

## 数据库迁移

迁移 `20260913_010000_01_create_rbac_tables` 创建全部业务表，`20260913_020000_02_add_rbac_reverse_indexes`
为 `user_roles(role_id)` 和 `role_permissions(permission_id)` 补充反向索引。迁移在服务监听端口前执行，
并由 `database/dbx/migrate` 使用 PostgreSQL session-level advisory lock 防止多个副本并发迁移。
