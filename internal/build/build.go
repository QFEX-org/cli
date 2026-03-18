package build

import (
	"fmt"
	"runtime"
)

// Version is set at build time via ldflags: -X github.com/qfex/cli/internal/build.Version=v1.2.3
var Version = "dev"

// UserAgent returns the User-Agent string for HTTP and WebSocket connections.
func UserAgent() string {
	return fmt.Sprintf("qfex-cli/%s (%s/%s)", Version, runtime.GOOS, runtime.GOARCH)
}
