//go:build !amd64

package ptitle

import "github.com/erikdubbelboer/gspt"

// SetProcTitle sets the process title (non-amd64 builds use gspt)
func SetProcTitle(title string) {
	gspt.SetProcTitle(title)
}