# 仓库协作规范

## 项目定位

本仓库是 `go-sdk/core`、`go-sdk/database` 和 `go-sdk/server` 的简易应用模板，提供
Proto 驱动的用户、角色、权限 API，以及额外的文件上传和头像替换 HTTP 接口。

## 修改前检查

每次修改前必须依次阅读：

1. `AGENTS.md`
2. `PROJECT_MAP.md`
3. `README.md`

同时检查 `git status --short`，区分暂存区、工作区和未跟踪文件，不覆盖或混入无关修改。

## 设计约束

- 普通业务接口以 `proto/` 为唯一协议来源，通过 gRPC Gateway 暴露 HTTP API。
- 只有 multipart 上传、文件下载等不适合 Proto JSON 映射的接口使用
  `standard.Server.HandlePath`。
- 新增受保护的 RPC 必须声明 `(server.options.method).permissions`；匿名 RPC 必须显式声明
  `skip_auth`。
- 密码、Token 等敏感字段必须使用 `(server.options.field).sensitive` 或为整个方法设置
  `skip_log`，不得写入日志。
- `dbx.Open` 不隐式迁移；迁移统一放在 `internal/migration`，ID 遵循
  `YYYYMMDD_HHMMSS_NN_description`。
- 模型主键使用 `core/seq` 生成，并复用 `dbx.Metadata` 的审计字段和软删除行为。
- 文件只在配置的存储根目录内按随机名称保存，数据库不保存文件内容或绝对路径。
- 不提交 `.env`、真实 DSN、JWT 密钥、管理员密码或其他秘密。

## 代码规范

- 遵循现有包结构，优先局部修改，不引入无明确收益的抽象和依赖。
- 注释和维护文档使用简体中文，只说明最终设计意图和关键约束。
- 错误创建和包装优先使用 `core/errx`；日志和错误文本遵循现有英文风格。
- 修改 Proto 后执行 `make generate`，并提交对应的 `gen/` 和 `openapi/` 生成结果。

## 验证边界

- 修改后执行 `make lint`、`make build`、`git diff --check`，并检查最终工作区状态。
- 未经明确授权，不运行单元、集成、端到端或冒烟测试，不启动应用或 PostgreSQL。
- 编译和 Compose 配置解析不代表数据库迁移、鉴权、网络或文件持久化已经过运行时验证。
- 不执行部署、发布、上传、`git push`；只有用户明确要求时才创建本地提交。
