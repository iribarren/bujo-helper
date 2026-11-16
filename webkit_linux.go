//go:build linux

package main

import "os"

// WebKitGTK's DMA-BUF renderer blanks the webview after a resize (e.g.
// maximizing) on some Wayland setups. Disable it before the webview starts,
// unless the user has set the variable themselves.
func init() {
	if _, set := os.LookupEnv("WEBKIT_DISABLE_DMABUF_RENDERER"); !set {
		os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	}
}
