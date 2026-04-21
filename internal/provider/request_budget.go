package provider

const (
	// MaxSessionAssetsTotalBytes 定义单次请求允许携带的 session_asset 原始总字节上限（20 MiB）。
	MaxSessionAssetsTotalBytes int64 = 20 * 1024 * 1024
)

// RequestAssetBudget 描述单次模型请求可携带的附件总预算限制。
type RequestAssetBudget struct {
	MaxSessionAssetsTotalBytes int64
}

// DefaultRequestAssetBudget 返回请求附件预算的默认值。
func DefaultRequestAssetBudget() RequestAssetBudget {
	return RequestAssetBudget{
		MaxSessionAssetsTotalBytes: MaxSessionAssetsTotalBytes,
	}
}

// NormalizeRequestAssetBudget 归一化请求附件预算，确保不越过硬上限且不低于单个附件上限。
func NormalizeRequestAssetBudget(budget RequestAssetBudget, maxSessionAssetBytes int64) RequestAssetBudget {
	normalized := budget
	if normalized.MaxSessionAssetsTotalBytes <= 0 {
		normalized.MaxSessionAssetsTotalBytes = MaxSessionAssetsTotalBytes
	}
	if normalized.MaxSessionAssetsTotalBytes > MaxSessionAssetsTotalBytes {
		normalized.MaxSessionAssetsTotalBytes = MaxSessionAssetsTotalBytes
	}
	if normalized.MaxSessionAssetsTotalBytes < maxSessionAssetBytes {
		normalized.MaxSessionAssetsTotalBytes = maxSessionAssetBytes
	}
	return normalized
}
