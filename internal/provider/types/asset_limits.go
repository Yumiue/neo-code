package types

const (
	// MaxSessionAssetBytes 定义 session_asset 在读写链路中的统一大小上限（20 MiB）。
	MaxSessionAssetBytes int64 = 20 * 1024 * 1024
	// MaxSessionAssetsTotalBytes 定义单次请求允许携带的 session_asset 原始总字节上限（20 MiB）。
	MaxSessionAssetsTotalBytes int64 = 20 * 1024 * 1024
)
