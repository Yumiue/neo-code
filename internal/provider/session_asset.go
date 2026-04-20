package provider

import (
	"context"
	"fmt"
	"io"
	"strings"

	providertypes "neo-code/internal/provider/types"
)

// ReadSessionAssetImage 读取单个 session_asset 图片，并统一执行预算、大小与 MIME 校验。
func ReadSessionAssetImage(
	ctx context.Context,
	assetReader providertypes.SessionAssetReader,
	asset *providertypes.AssetRef,
	remainingBudget int64,
	assetLimits providertypes.SessionAssetLimits,
) (string, []byte, int64, error) {
	normalizedAssetLimits := providertypes.NormalizeSessionAssetLimits(assetLimits)
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", nil, 0, err
	}
	if assetReader == nil {
		return "", nil, 0, fmt.Errorf("session_asset reader is not configured")
	}
	if asset == nil || strings.TrimSpace(asset.ID) == "" {
		return "", nil, 0, fmt.Errorf("session_asset image missing asset id")
	}
	if remainingBudget <= 0 {
		return "", nil, 0, fmt.Errorf(
			"session_asset total exceeds %d bytes",
			normalizedAssetLimits.MaxSessionAssetsTotalBytes,
		)
	}

	reader, mimeType, err := assetReader.Open(ctx, asset.ID)
	if err != nil {
		return "", nil, 0, fmt.Errorf("open session_asset %q: %w", asset.ID, err)
	}
	defer func() { _ = reader.Close() }()

	readLimit := normalizedAssetLimits.MaxSessionAssetBytes
	if remainingBudget < readLimit {
		readLimit = remainingBudget
	}

	data, err := io.ReadAll(io.LimitReader(reader, readLimit+1))
	if err != nil {
		return "", nil, 0, fmt.Errorf("read session_asset %q: %w", asset.ID, err)
	}
	if err := ctx.Err(); err != nil {
		return "", nil, 0, err
	}
	if int64(len(data)) > readLimit {
		if readLimit < normalizedAssetLimits.MaxSessionAssetBytes {
			return "", nil, 0, fmt.Errorf(
				"session_asset total exceeds %d bytes",
				normalizedAssetLimits.MaxSessionAssetsTotalBytes,
			)
		}
		return "", nil, 0, fmt.Errorf("session_asset %q exceeds %d bytes", asset.ID, normalizedAssetLimits.MaxSessionAssetBytes)
	}
	if len(data) == 0 {
		return "", nil, 0, fmt.Errorf("session_asset %q is empty", asset.ID)
	}

	resolvedMime := strings.TrimSpace(mimeType)
	if resolvedMime == "" {
		resolvedMime = strings.TrimSpace(asset.MimeType)
	}
	normalizedMime := strings.ToLower(resolvedMime)
	if normalizedMime == "" {
		return "", nil, 0, fmt.Errorf("session_asset %q missing mime type", asset.ID)
	}
	if !strings.HasPrefix(normalizedMime, "image/") {
		return "", nil, 0, fmt.Errorf("session_asset %q has unsupported mime type %q", asset.ID, resolvedMime)
	}

	return normalizedMime, data, int64(len(data)), nil
}
