//go:build windows

package platform

import "syscall"

const (
	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)
	processPerMonitorDPIAware            = uintptr(2)
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	shcore = syscall.NewLazyDLL("shcore.dll")

	setProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	setProcessDpiAwareness        = shcore.NewProc("SetProcessDpiAwareness")
)

func EnableDPIAwareness() {
	// Windows can otherwise scale rendering and mouse coordinates differently
	// on high-DPI full-screen or maximized windows.
	if err := user32.Load(); err == nil {
		if err := setProcessDpiAwarenessContext.Find(); err == nil {
			ret, _, _ := setProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
			if ret != 0 {
				return
			}
		}
	}

	if err := shcore.Load(); err == nil {
		if err := setProcessDpiAwareness.Find(); err == nil {
			_, _, _ = setProcessDpiAwareness.Call(processPerMonitorDPIAware)
		}
	}
}
