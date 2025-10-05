//go:build nonbd

package kvm

import (
	"fmt"
	"net"
	"os"

	"github.com/rs/zerolog"
)

// Stub constants to satisfy references in nonbd builds
const nbdSocketPath = "/var/run/nbd.socket"
const nbdDevicePath = "/dev/nbd0"

// Stub NBDDevice to satisfy usage in usb_mass_storage.go when NBD is disabled
type NBDDevice struct {
	listener   net.Listener
	serverConn net.Conn
	clientConn net.Conn
	dev        *os.File

	l *zerolog.Logger
}

func NewNBDDevice() *NBDDevice {
	return &NBDDevice{}
}

// Start returns an error to indicate NBD is disabled in this build
func (d *NBDDevice) Start() error {
	return fmt.Errorf("NBD is disabled in amd64 build (nonbd tag)")
}

func (d *NBDDevice) Close() {
	// no-op
}