package version

import (
	"regexp"
	"strings"
)

var semverPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

// Version 表示当前构建注入的版本号；默认值用于本地开发构建。
var Version = "dev"

// Current 返回归一化后的当前版本；空值会回退为 dev。
func Current() string {
	value := strings.TrimSpace(Version)
	if value == "" {
		return "dev"
	}
	return value
}

// IsSemverRelease 判断给定版本字符串是否为可比较的语义化版本。
func IsSemverRelease(value string) bool {
	return semverPattern.MatchString(strings.TrimSpace(value))
}
