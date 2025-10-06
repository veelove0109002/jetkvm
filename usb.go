package kvm

import (
	"os"
	"sync"
	"time"

	"github.com/jetkvm/kvm/internal/usbgadget"
	"github.com/jetkvm/kvm/internal/uinput"
)

type inputBackend interface {
	OpenKeyboardHidFile() error
	KeyboardReport(modifier byte, keys []byte) error
	KeypressReport(key byte, press bool) error
	AbsMouseReport(x int, y int, buttons uint8) error
	RelMouseReport(dx int8, dy int8, buttons uint8) error
	AbsMouseWheelReport(wheelY int8) error

	// keyboard state
	GetKeyboardState() usbgadget.KeyboardState
	GetKeysDownState() usbgadget.KeysDownState
	UpdateKeysDown(modifier byte, keys []byte) usbgadget.KeysDownState
	DelayAutoReleaseWithDuration(resetDuration time.Duration)

	// callbacks
	SetOnKeyboardStateChange(func(state usbgadget.KeyboardState))
	SetOnKeysDownChange(func(state usbgadget.KeysDownState))
	SetOnKeepAliveReset(func())

	// misc
	GetUsbState() string
	GetLastUserInputTime() time.Time

	// gadget-specific operations (no-op in uinput)
	IsUDCBound() (bool, error)
	BindUDC() error
	UnbindUDC() error
	GetPath(subpath string) (string, error)
	SetGadgetConfig(cfg *usbgadget.Config) error
	OverrideGadgetConfig(manufacturer, product, serial string) (error, bool)
	UpdateGadgetConfig() error
	SetGadgetDevices(dev *usbgadget.Devices) error
}

var gadget inputBackend

 // detectUsbDeviceMode returns true if UDC exists and is usable (rough check)
func detectUsbDeviceMode() bool {
	if _, err := os.Stat("/sys/class/udc"); err != nil {
		return false
	}
	// 粗略检测：目录存在即认为可能可用，细化可读具体 UDC 条目
	return true
}

// initUsbGadget initializes input backend: prefer USB gadget if device mode present; else fallback to uinput.
func initUsbGadget() {
	if detectUsbDeviceMode() {
		usbLogger.Info().Msg("USB Device Mode detected, initializing USB gadget backend")
		gadget = usbgadget.NewUsbGadget(
			"jetkvm",
			config.UsbDevices,
			config.UsbConfig,
			usbLogger,
		)
		if gadget == nil {
			usbLogger.Warn().Msg("USB gadget init failed, falling back to uinput backend")
		}
	}

	if gadget == nil {
		usbLogger.Info().Msg("Initializing uinput backend")
		u, err := uinput.NewUInputBackend(usbLogger)
		if err != nil {
			usbLogger.Error().Err(err).Msg("failed to init uinput backend")
		} else {
			gadget = u
		}
	}

	if gadget == nil {
		usbLogger.Error().Msg("no input backend available")
		return
	}

	go func() {
		for {
			checkUSBState()
			time.Sleep(500 * time.Millisecond)
		}
	}()

	gadget.SetOnKeyboardStateChange(func(state usbgadget.KeyboardState) {
		if currentSession != nil {
			currentSession.reportHidRPCKeyboardLedState(state)
		}
	})

	gadget.SetOnKeysDownChange(func(state usbgadget.KeysDownState) {
		if currentSession != nil {
			currentSession.enqueueKeysDownState(state)
		}
	})

	gadget.SetOnKeepAliveReset(func() {
		if currentSession != nil {
			currentSession.resetKeepAliveTime()
		}
	})

	// open the keyboard hid file to listen for keyboard events
	if err := gadget.OpenKeyboardHidFile(); err != nil {
		usbLogger.Warn().Err(err).Msg("keyboard hid file open skipped or failed (backend-specific)")
	}
}

func rpcKeyboardReport(modifier byte, keys []byte) error {
	return gadget.KeyboardReport(modifier, keys)
}

func rpcKeypressReport(key byte, press bool) error {
	return gadget.KeypressReport(key, press)
}

func rpcAbsMouseReport(x int, y int, buttons uint8) error {
	return gadget.AbsMouseReport(x, y, buttons)
}

func rpcRelMouseReport(dx int8, dy int8, buttons uint8) error {
	return gadget.RelMouseReport(dx, dy, buttons)
}

func rpcWheelReport(wheelY int8) error {
	return gadget.AbsMouseWheelReport(wheelY)
}

func rpcGetKeyboardLedState() (state usbgadget.KeyboardState) {
	return gadget.GetKeyboardState()
}

func rpcGetKeysDownState() (state usbgadget.KeysDownState) {
	return gadget.GetKeysDownState()
}

var (
	usbState     = "unknown"
	usbStateLock sync.Mutex
)

func rpcGetUSBState() (state string) {
	return gadget.GetUsbState()
}

func triggerUSBStateUpdate() {
	go func() {
		if currentSession == nil {
			usbLogger.Info().Msg("No active RPC session, skipping USB state update")
			return
		}
		writeJSONRPCEvent("usbState", usbState, currentSession)
	}()
}

func checkUSBState() {
	usbStateLock.Lock()
	defer usbStateLock.Unlock()

	newState := gadget.GetUsbState()

	usbLogger.Trace().Str("old", usbState).Str("new", newState).Msg("Checking USB state")

	if newState == usbState {
		return
	}

	usbState = newState
	usbLogger.Info().Str("from", usbState).Str("to", newState).Msg("USB state changed")

	requestDisplayUpdate(true, "usb_state_changed")
	triggerUSBStateUpdate()
}
