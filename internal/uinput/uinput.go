package uinput

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/jetkvm/kvm/internal/usbgadget"
	"github.com/rs/zerolog"
)

type UInputBackend struct {
	fd                    *os.File
	log                   *zerolog.Logger
	onKeyboardStateChange *func(state usbgadget.KeyboardState)
	onKeysDownChange      *func(state usbgadget.KeysDownState)
	onKeepAliveReset      *func()

	keyboardStateLock sync.Mutex
	keyboardState     byte
	keysDownState     usbgadget.KeysDownState

	lastUserInput time.Time
}

var defaultLogger = zerolog.New(os.Stdout).With().Str("subsystem", "uinput").Logger()

// evdev/uinput 常量
const (
	UI_DEV_CREATE = 0x5501
	UI_DEV_DESTROY= 0x5502
	UI_SET_EVBIT  = 0x40045564
	UI_SET_KEYBIT = 0x40045565

	EV_SYN = 0x00
	EV_KEY = 0x01

	SYN_REPORT = 0
)

type input_event struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

// NewUInputBackend 创建并注册一个虚拟键盘设备
func NewUInputBackend(logger *zerolog.Logger) (*UInputBackend, error) {
	if logger == nil {
		l := defaultLogger
		logger = &l
	}
	u := &UInputBackend{
		log:            logger,
		keyboardState:  0,
		keysDownState:  usbgadget.KeysDownState{Modifier: 0, Keys: []byte{0,0,0,0,0,0}},
		lastUserInput:  time.Now(),
	}

	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open /dev/uinput failed: %w. Ensure 'modprobe uinput' and permissions", err)
	}
	u.fd = f

	// 使能 EV_KEY
	if err := u.ioctl(UI_SET_EVBIT, EV_KEY); err != nil {
		return nil, fmt.Errorf("ioctl UI_SET_EVBIT EV_KEY failed: %w", err)
	}
	// 注册常用按键和修饰键
	for _, code := range hidToLinux {
		_ = u.ioctl(UI_SET_KEYBIT, uint64(code))
	}
	for _, code := range hidModifierToLinux {
		_ = u.ioctl(UI_SET_KEYBIT, uint64(code))
	}

	// 创建设备（最简，不设置名称/厂商）
	if err := u.ioctl(UI_DEV_CREATE, 0); err != nil {
		return nil, fmt.Errorf("ioctl UI_DEV_CREATE failed: %w", err)
	}

	return u, nil
}

func (u *UInputBackend) Close() error {
	if u.fd != nil {
		_ = u.ioctl(UI_DEV_DESTROY, 0)
		_ = u.fd.Close()
		u.fd = nil
	}
	return nil
}

func (u *UInputBackend) ioctl(request uintptr, arg uint64) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, u.fd.Fd(), request, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

func (u *UInputBackend) writeEvent(typ, code uint16, val int32) error {
	ev := input_event{
		Time:  syscall.Timeval{Sec: 0, Usec: 0},
		Type:  typ,
		Code:  code,
		Value: val,
	}
	return binary.Write(u.fd, binary.LittleEndian, &ev)
}

func (u *UInputBackend) sync() {
	_ = u.writeEvent(EV_SYN, SYN_REPORT, 0)
}

func (u *UInputBackend) SetOnKeyboardStateChange(f func(state usbgadget.KeyboardState)) {
	u.onKeyboardStateChange = &f
}

func (u *UInputBackend) SetOnKeysDownChange(f func(state usbgadget.KeysDownState)) {
	u.onKeysDownChange = &f
}

func (u *UInputBackend) SetOnKeepAliveReset(f func()) {
	u.onKeepAliveReset = &f
}

func (u *UInputBackend) OpenKeyboardHidFile() error {
	// uinput 无需打开 hid 文件，no-op
	return nil
}

func (u *UInputBackend) GetUsbState() string {
	// 在 uinput 模式下，不是 USB Gadget。返回 native
	return "native"
}

func (u *UInputBackend) GetKeyboardState() usbgadget.KeyboardState {
	u.keyboardStateLock.Lock()
	defer u.keyboardStateLock.Unlock()
	// LED 状态无法从 uinput 查询，返回当前记录值（默认为 0）
	return getKeyboardState(u.keyboardState)
}

// 复用 usbgadget 的 getKeyboardState 逻辑
func getKeyboardState(b byte) usbgadget.KeyboardState {
	return usbgadget.KeyboardState{
		NumLock:    b&usbgadget.KeyboardLedMaskNumLock != 0,
		CapsLock:   b&usbgadget.KeyboardLedMaskCapsLock != 0,
		ScrollLock: b&usbgadget.KeyboardLedMaskScrollLock != 0,
		Compose:    b&usbgadget.KeyboardLedMaskCompose != 0,
		Kana:       b&usbgadget.KeyboardLedMaskKana != 0,
		Shift:      b&usbgadget.KeyboardLedMaskShift != 0,
	}
}

func (u *UInputBackend) GetKeysDownState() usbgadget.KeysDownState {
	u.keyboardStateLock.Lock()
	defer u.keyboardStateLock.Unlock()
	return u.keysDownState
}

func (u *UInputBackend) updateKeysDown(modifier byte, keys []byte) usbgadget.KeysDownState {
	// 复制并规范长度
	k := make([]byte, 6)
	copy(k, keys)
	state := usbgadget.KeysDownState{Modifier: modifier, Keys: k}
	u.keyboardStateLock.Lock()
	u.keysDownState = state
	u.keyboardStateLock.Unlock()
	if u.onKeysDownChange != nil {
		(*u.onKeysDownChange)(state)
	}
	return state
}

// 公开方法以满足接口
func (u *UInputBackend) UpdateKeysDown(modifier byte, keys []byte) usbgadget.KeysDownState {
	return u.updateKeysDown(modifier, keys)
}


func (u *UInputBackend) resetUserInputTime() {
	u.lastUserInput = time.Now()
}

func (u *UInputBackend) GetLastUserInputTime() time.Time {
	return u.lastUserInput
}

// KeyboardReport：按 HID 语义注入键盘状态（modifier+keys）为一帧
func (u *UInputBackend) KeyboardReport(modifier byte, keys []byte) error {
	// 先处理修饰键（每次帧重置：简单做开关键事件）
	for hid, code := range hidModifierToLinux {
		pressed := (modifier & hidMaskFor(hid)) != 0
		if pressed {
			_ = u.writeEvent(EV_KEY, uint16(code), 1)
		} else {
			_ = u.writeEvent(EV_KEY, uint16(code), 0)
		}
	}

	// 清空普通键后重放当前 keys 阵列（简化处理）
	// 实际更佳做法是比较上一帧状态，这里为最小实现。
	for _, code := range hidToLinux {
		_ = u.writeEvent(EV_KEY, uint16(code), 0)
	}
	for _, hid := range keys {
		if code, ok := hidToLinux[hid]; ok && hid != 0 {
			_ = u.writeEvent(EV_KEY, uint16(code), 1)
		}
	}
	u.sync()

	u.updateKeysDown(modifier, keys)
	u.resetUserInputTime()
	return nil
}

func hidMaskFor(hid byte) byte {
	switch hid {
	case 0xE0: return 0x01
	case 0xE1: return 0x02
	case 0xE2: return 0x04
	case 0xE3: return 0x08
	case 0xE4: return 0x10
	case 0xE5: return 0x20
	case 0xE6: return 0x40
	case 0xE7: return 0x80
	default: return 0
	}
}

// KeypressReport：按单键 press/release 注入（更贴近 usbgadget 的行为）
func (u *UInputBackend) KeypressReport(key byte, press bool) error {
	// uinput 模式下不使用自动释放，DelayAutoReleaseWithDuration 为 no-op
	// 修饰键
	if code, ok := hidModifierToLinux[key]; ok {
		if press {
			_ = u.writeEvent(EV_KEY, uint16(code), 1)
		} else {
			_ = u.writeEvent(EV_KEY, uint16(code), 0)
		}
		u.sync()
		// 更新 modifier 位
		mod := u.keysDownState.Modifier
		mask := hidMaskFor(key)
		if press {
			mod |= mask
		} else {
			mod &^= mask
		}
		// 普通键不变
		u.updateKeysDown(mod, u.keysDownState.Keys)
		u.resetUserInputTime()
		return nil
	}

	// 普通键
	code, ok := hidToLinux[key]
	if !ok {
		// 未映射的键忽略但仍更新状态
		return nil
	}
	val := int32(0)
	if press {
		val = 1
	}
	_ = u.writeEvent(EV_KEY, uint16(code), val)
	u.sync()

	// 更新 keysDown（简单加入/移除）
	ks := append([]byte(nil), u.keysDownState.Keys...)
	if press {
		placed := false
		for i := range ks {
			if ks[i] == 0 {
				ks[i] = key
				placed = true
				break
			}
		}
		if !placed {
			// 溢出忽略
		}
	} else {
		for i := range ks {
			if ks[i] == key {
				ks[i] = 0
				break
			}
		}
	}
	u.updateKeysDown(u.keysDownState.Modifier, ks)
	u.resetUserInputTime()
	return nil
}

func (u *UInputBackend) DelayAutoReleaseWithDuration(resetDuration time.Duration) {
	// no-op in uinput
}

func (u *UInputBackend) GetPath(subpath string) (string, error) { return "", nil }

// 鼠标在 uinput 下暂不实现，保留空实现以兼容编译与调用
func (u *UInputBackend) AbsMouseReport(x int, y int, buttons uint8) error { return nil }
func (u *UInputBackend) RelMouseReport(dx int8, dy int8, buttons uint8) error { return nil }
func (u *UInputBackend) AbsMouseWheelReport(wheelY int8) error { return nil }

 // gadget 相关操作在 uinput 下无意义，均返回 no-op
func (u *UInputBackend) IsUDCBound() (bool, error) { return false, nil }
func (u *UInputBackend) BindUDC() error { return nil }
func (u *UInputBackend) UnbindUDC() error { return nil }
func (u *UInputBackend) SetGadgetConfig(cfg *usbgadget.Config) {}
func (u *UInputBackend) OverrideGadgetConfig(manufacturer, product, serial string) (error, bool) { return nil, false }
func (u *UInputBackend) UpdateGadgetConfig() error { return nil }
func (u *UInputBackend) SetGadgetDevices(dev *usbgadget.Devices) {}