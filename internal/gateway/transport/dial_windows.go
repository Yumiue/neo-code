//go:build windows

package transport

import (
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

const defaultNamedPipeDialTimeout = 3 * time.Second

var dialPipeFn = winio.DialPipe

// dial 在 Windows 系统上通过 Named Pipe 连接网关。
func dial(address string) (net.Conn, error) {
	timeout := defaultNamedPipeDialTimeout
	return dialPipeFn(address, &timeout)
}
