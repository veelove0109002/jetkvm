package uinput

// 该文件包含 HID Usage 到 Linux input KEY_* 的简易映射，覆盖常用按键。
// 注意：并非完整映射，未覆盖的按键将被忽略或仅维持 KeysDownState。
// 可按需扩展。

// Linux input key codes（摘取常用）
const (
	KEY_RESERVED = 0
	KEY_ESC      = 1
	KEY_1        = 2
	KEY_2        = 3
	KEY_3        = 4
	KEY_4        = 5
	KEY_5        = 6
	KEY_6        = 7
	KEY_7        = 8
	KEY_8        = 9
	KEY_9        = 10
	KEY_0        = 11
	KEY_MINUS    = 12
	KEY_EQUAL    = 13
	KEY_BACKSPACE= 14
	KEY_TAB      = 15
	KEY_Q        = 16
	KEY_W        = 17
	KEY_E        = 18
	KEY_R        = 19
	KEY_T        = 20
	KEY_Y        = 21
	KEY_U        = 22
	KEY_I        = 23
	KEY_O        = 24
	KEY_P        = 25
	KEY_LEFTBRACE= 26
	KEY_RIGHTBRACE=27
	KEY_ENTER    = 28
	KEY_LEFTCTRL = 29
	KEY_A        = 30
	KEY_S        = 31
	KEY_D        = 32
	KEY_F        = 33
	KEY_G        = 34
	KEY_H        = 35
	KEY_J        = 36
	KEY_K        = 37
	KEY_L        = 38
	KEY_SEMICOLON= 39
	KEY_APOSTROPHE=40
	KEY_GRAVE    = 41
	KEY_LEFTSHIFT= 42
	KEY_BACKSLASH= 43
	KEY_Z        = 44
	KEY_X        = 45
	KEY_C        = 46
	KEY_V        = 47
	KEY_B        = 48
	KEY_N        = 49
	KEY_M        = 50
	KEY_COMMA    = 51
	KEY_DOT      = 52
	KEY_SLASH    = 53
	KEY_RIGHTSHIFT=54
	KEY_KPASTERISK=55
	KEY_LEFTALT  = 56
	KEY_SPACE    = 57
	KEY_CAPSLOCK = 58
	KEY_F1       = 59
	KEY_F2       = 60
	KEY_F3       = 61
	KEY_F4       = 62
	KEY_F5       = 63
	KEY_F6       = 64
	KEY_F7       = 65
	KEY_F8       = 66
	KEY_F9       = 67
	KEY_F10      = 68
	KEY_NUMLOCK  = 69
	KEY_SCROLLLOCK=70
	KEY_LEFT     = 105
	KEY_RIGHT    = 106
	KEY_UP       = 103
	KEY_DOWN     = 108
	KEY_RIGHTCTRL= 97
	KEY_RIGHTALT = 100
	KEY_LEFTMETA = 125
	KEY_RIGHTMETA= 126
)

// HID Usage codes（键盘）：见 HID Usage Tables，取常用
// 4-29: A-Z, 30-39: 1-0
// 40 Enter, 41 Escape, 42 Backspace, 43 Tab, 44 Space
// 54~57 左右箭头、上、下（注意 HID 与 Linux keycode差异，这里直接映射常用箭头）
var hidToLinux = map[byte]int{
	// A-Z
	4:  KEY_A, 5: KEY_B, 6: KEY_C, 7: KEY_D, 8: KEY_E, 9: KEY_F,
	10: KEY_G, 11: KEY_H, 12: KEY_I, 13: KEY_J, 14: KEY_K, 15: KEY_L,
	16: KEY_M, 17: KEY_N, 18: KEY_O, 19: KEY_P, 20: KEY_Q, 21: KEY_R,
	22: KEY_S, 23: KEY_T, 24: KEY_U, 25: KEY_V, 26: KEY_W, 27: KEY_X,
	28: KEY_Y, 29: KEY_Z,
	// 1-0
	30: KEY_1, 31: KEY_2, 32: KEY_3, 33: KEY_4, 34: KEY_5,
	35: KEY_6, 36: KEY_7, 37: KEY_8, 38: KEY_9, 39: KEY_0,
	// 控制键
	40: KEY_ENTER,
	41: KEY_ESC,
	42: KEY_BACKSPACE,
	43: KEY_TAB,
	44: KEY_SPACE,
	// 方向
	82: KEY_UP,
	81: KEY_DOWN,
	80: KEY_LEFT,
	79: KEY_RIGHT,
}

// 修饰键 HID → Linux keycode
var hidModifierToLinux = map[byte]int{
	0xE0: KEY_LEFTCTRL,
	0xE1: KEY_LEFTSHIFT,
	0xE2: KEY_LEFTALT,
	0xE3: KEY_LEFTMETA,
	0xE4: KEY_RIGHTCTRL,
	0xE5: KEY_RIGHTSHIFT,
	0xE6: KEY_RIGHTALT,
	0xE7: KEY_RIGHTMETA,
}