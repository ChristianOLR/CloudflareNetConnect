//go:build !windows

package service

// EnsureAdmin en plataformas no-Windows es un no-op.
func EnsureAdmin() bool {
	return false
}
