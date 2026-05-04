package gateway

import (
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// knownAPIPrefixes 定义属于 Gateway API 的路径前缀，静态文件中间件不会拦截这些路径。
var knownAPIPrefixes = map[string]bool{
	"/healthz":      true,
	"/version":      true,
	"/rpc":          true,
	"/ws":           true,
	"/sse":          true,
	"/metrics":      true,
	"/metrics.json": true,
}

// WithStaticFileHandler 返回一个 http.Handler，将 API 请求转发给 apiHandler，
// 其余请求从 staticDir 提供静态文件。对于 SPA 路由，不存在的路径会回退到 index.html。
func WithStaticFileHandler(apiHandler http.Handler, staticDir string, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		cleanPath := path.Clean("/" + request.URL.Path)

		// API 路径直接转发
		if isAPIPath(cleanPath) {
			apiHandler.ServeHTTP(writer, request)
			return
		}

		// 根路径 → index.html
		relPath := strings.TrimPrefix(cleanPath, "/")
		if relPath == "" {
			relPath = "index.html"
		}

		// 检查文件是否存在
		fullPath := filepath.Join(staticDir, filepath.FromSlash(relPath))
		info, err := os.Stat(fullPath)
		if err == nil && !info.IsDir() {
			setCacheHeaders(writer, relPath)
			http.ServeFile(writer, request, fullPath)
			return
		}

		// SPA fallback：文件不存在时返回 index.html
		indexPath := filepath.Join(staticDir, "index.html")
		if _, statErr := os.Stat(indexPath); statErr == nil {
			writer.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(writer, request, indexPath)
			return
		}

		if logger != nil {
			logger.Printf("static files: index.html not found in %s", staticDir)
		}
		http.NotFound(writer, request)
	})
}

// isAPIPath 判断请求路径是否属于 Gateway API。
func isAPIPath(cleanPath string) bool {
	if knownAPIPrefixes[cleanPath] {
		return true
	}
	for prefix := range knownAPIPrefixes {
		if strings.HasPrefix(cleanPath, prefix+"/") {
			return true
		}
	}
	return false
}

// setCacheHeaders 根据文件名设置缓存策略。
// Vite hashed assets（如 assets/index-BzA30N4.js）使用 immutable 缓存，
// 其他文件使用 no-cache。
func setCacheHeaders(writer http.ResponseWriter, relPath string) {
	base := path.Base(relPath)
	if strings.Contains(base, "-") && !strings.HasSuffix(base, ".html") {
		writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		writer.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	}
}
