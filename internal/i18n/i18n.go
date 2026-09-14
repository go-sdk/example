package i18n

import "embed"

// Files 包含应用业务错误的非英文翻译，英文默认文案由 Proto 枚举选项提供。
//
//go:embed locales/*.toml
var Files embed.FS
