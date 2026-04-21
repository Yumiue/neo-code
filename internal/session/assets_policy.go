package session

const (
	// MaxSessionAssetBytes 定义 session_asset 在读写链路中的统一大小上限（20 MiB）。
	MaxSessionAssetBytes int64 = 20 * 1024 * 1024
)

// AssetPolicy 描述 session_asset 在单文件维度的存储与读写策略。
type AssetPolicy struct {
	MaxSessionAssetBytes int64
}

// DefaultAssetPolicy 返回 session_asset 策略的默认值。
func DefaultAssetPolicy() AssetPolicy {
	return AssetPolicy{
		MaxSessionAssetBytes: MaxSessionAssetBytes,
	}
}

// NormalizeAssetPolicy 归一化 session_asset 策略并施加硬上限兜底。
func NormalizeAssetPolicy(policy AssetPolicy) AssetPolicy {
	normalized := policy
	if normalized.MaxSessionAssetBytes <= 0 {
		normalized.MaxSessionAssetBytes = MaxSessionAssetBytes
	}
	if normalized.MaxSessionAssetBytes > MaxSessionAssetBytes {
		normalized.MaxSessionAssetBytes = MaxSessionAssetBytes
	}
	return normalized
}
