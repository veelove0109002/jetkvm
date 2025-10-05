//go:build amd64

package ptitle

// SetProcTitle is a no-op on amd64 to avoid gspt build constraints
func SetProcTitle(title string) {}