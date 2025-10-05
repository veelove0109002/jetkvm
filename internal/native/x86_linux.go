//go:build linux && amd64

package native

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

/*
x86 Linux 后端：使用 ffmpeg 捕获 V4L2 MJPG 并编码为 H.264，通过现有通道推送到 WebRTC。
环境变量：
- VIDEO_DEVICE: 采集设备路径，默认 /dev/video0
- VIDEO_FPS: 目标帧率，默认 30
- VIDEO_BITRATE: x264码率，如 4M/8M，默认 4M
*/

var (
	ffmpegCmd       *exec.Cmd
	ffmpegCancel    context.CancelFunc
	ffmpegStdout    ioReadCloser
	readerWG        sync.WaitGroup
	streamLock      sync.Mutex
	targetFPS       = 30
	targetBitrate   = "4M"
	videoDevice     = "/dev/video0"
	qualityFactor   = 1.0 // 0.5~2.0：线性映射到码率
	currentState    = VideoState{Ready: false}
)

// 硬件加速相关配置
var (
	hwAccelMode = "auto"                     // auto|vaapi|qsv|none
	vaapiDevice = "/dev/dri/renderD128"      // 可通过 VIDEO_VAAPI_DEVICE 覆盖
)

type ioReadCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}

func setUpNativeHandlers() {
	// x86 后端无需 cgo handler 绑定，直接使用 Go 通道
}

func uiInit(rotation uint16) {
	// x86 无屏控，no-op
}

func uiTick() {
	// x86 无屏控，no-op
}

func uiSetVar(name string, value string)               {}
func uiGetVar(name string) string                      { return "" }
func uiSwitchToScreen(screen string)                   {}
func uiGetCurrentScreen() string                       { return "" }
func uiObjAddState(objName string, state string) (bool, error) { return false, nil }
func uiObjClearState(objName string, state string) (bool, error) { return false, nil }
func uiObjAddFlag(objName string, flag string) (bool, error) { return false, nil }
func uiObjClearFlag(objName string, flag string) (bool, error) { return false, nil }
func uiObjHide(objName string) (bool, error)           { return false, nil }
func uiObjShow(objName string) (bool, error)           { return false, nil }
func uiObjSetOpacity(objName string, opacity int) (bool, error) { return false, nil }
func uiObjFadeIn(objName string, duration uint32) (bool, error) { return false, nil }
func uiObjFadeOut(objName string, duration uint32) (bool, error) { return false, nil }
func uiLabelSetText(objName string, text string) (bool, error)   { return false, nil }
func uiImgSetSrc(objName string, src string) (bool, error)       { return false, nil }
func uiDispSetRotation(rotation uint16) (bool, error)            { return false, nil }
func uiEventCodeToName(code int) string                          { return "" }
func uiGetLVGLVersion() string                                   { return "" }

func videoInit() error {
	streamLock.Lock()
	defer streamLock.Unlock()

	// 读取环境配置
	if dev := strings.TrimSpace(os.Getenv("VIDEO_DEVICE")); dev != "" {
		videoDevice = dev
	}
	if fpsStr := strings.TrimSpace(os.Getenv("VIDEO_FPS")); fpsStr != "" {
		if v, err := strconv.Atoi(fpsStr); err == nil && v > 0 && v <= 120 {
			targetFPS = v
		}
	}
	if br := strings.TrimSpace(os.Getenv("VIDEO_BITRATE")); br != "" {
		targetBitrate = br
	}
	// 硬件加速环境变量
	if accel := strings.TrimSpace(os.Getenv("VIDEO_HWACCEL")); accel != "" {
		// 允许 auto|vaapi|qsv|none
		accelLower := strings.ToLower(accel)
		if accelLower == "auto" || accelLower == "vaapi" || accelLower == "qsv" || accelLower == "none" {
			hwAccelMode = accelLower
		}
	}
	if dev := strings.TrimSpace(os.Getenv("VIDEO_VAAPI_DEVICE")); dev != "" {
		vaapiDevice = dev
	}

	// 简单检查视频采集设备存在
	if _, err := os.Stat(videoDevice); err != nil {
		return fmt.Errorf("video device not found: %s", videoDevice)
	}

	return nil
}

func videoShutdown() {
	// 与 videoStop 等价
	videoStop()
}

func buildFFmpegArgs() []string {
	// 根据硬件能力选择 VAAPI/QSV/CPU。输入为 V4L2 MJPEG，输出 H.264 Annex B bytestream 到 stdout。
	// 优先 VAAPI（多数核显可用），其次 QSV（Intel），否则回退 CPU。
	// 低延迟：关闭 B 帧，保持小缓冲。
	// 注意：QSV 需显式选择输入解码器 mjpeg_qsv，VAAPI 用 hwupload 将软件解码帧上传到 GPU。

	// 判定是否能用 VAAPI（auto 模式下优先）
	canVAAPI := false
	if hwAccelMode == "vaapi" || hwAccelMode == "auto" {
		if _, err := os.Stat(vaapiDevice); err == nil {
			canVAAPI = true
		}
	}

	// QSV 通常也依赖 /dev/dri/renderD128（Intel），这里用存在性作为粗略判定
	canQSV := false
	if hwAccelMode == "qsv" || (hwAccelMode == "auto" && !canVAAPI) {
		if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
			canQSV = true
		}
	}

	if canVAAPI && hwAccelMode != "none" {
		return []string{
			"-hide_banner",
			"-loglevel", "warning",
			"-fflags", "nobuffer",
			"-flags", "low_delay",
			// 输入侧硬件加速需放在 -i 之前
			"-hwaccel", "vaapi",
			"-vaapi_device", vaapiDevice,
			// 输入：V4L2 MJPEG
			"-f", "v4l2",
			"-input_format", "mjpeg",
			"-i", videoDevice,
			"-an",
			// VAAPI：将帧上传到 GPU，编码 h264_vaapi
			"-vf", "format=nv12,hwupload",
			"-c:v", "h264_vaapi",
			"-bf", "0",
			"-r", strconv.Itoa(targetFPS),
			"-b:v", targetBitrate,
			"-f", "h264",
			"pipe:1",
		}
	}

	if canQSV && hwAccelMode != "none" {
		return []string{
			"-hide_banner",
			"-loglevel", "warning",
			"-fflags", "nobuffer",
			"-flags", "low_delay",
			// 输入侧硬件加速需放在 -i 之前
			"-hwaccel", "qsv",
			// 输入：V4L2 MJPEG
			"-f", "v4l2",
			"-input_format", "mjpeg",
			"-i", videoDevice,
			"-an",
			// 输出编码 h264_qsv
			"-c:v", "h264_qsv",
			"-look_ahead", "0",
			"-bf", "0",
			"-r", strconv.Itoa(targetFPS),
			"-b:v", targetBitrate,
			"-f", "h264",
			"pipe:1",
		}
	}

	// 回退：CPU libx264（原始实现）
	return []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-f", "v4l2",
		"-input_format", "mjpeg",
		"-i", videoDevice,
		"-an",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-tune", "zerolatency",
		"-profile:v", "baseline",
		"-level", "3.1",
		"-pix_fmt", "yuv420p",
		"-r", strconv.Itoa(targetFPS),
		"-b:v", targetBitrate,
		"-f", "h264",
		"pipe:1",
	}
}

func videoStart() {
	streamLock.Lock()
	defer streamLock.Unlock()

	// 若已在运行，忽略
	if ffmpegCmd != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	ffmpegCancel = cancel
	args := buildFFmpegArgs()
	ffmpegCmd = exec.CommandContext(ctx, "ffmpeg", args...)

	stdout, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		logChan <- nativeLogMessage{Level: zerolog.ErrorLevel, Message: fmt.Sprintf("ffmpeg stdout pipe error: %v", err)}
		ffmpegCmd = nil
		ffmpegCancel = nil
		return
	}
	ffmpegStdout = stdout

	stderr, _ := ffmpegCmd.StderrPipe()
	if err := ffmpegCmd.Start(); err != nil {
		logChan <- nativeLogMessage{Level: zerolog.ErrorLevel, Message: fmt.Sprintf("ffmpeg start failed: %v", err)}
		ffmpegCmd = nil
		ffmpegCancel = nil
		return
	}

	// 读取 stderr（可选日志）
	readerWG.Add(1)
	go func() {
		defer readerWG.Done()
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			line := sc.Text()
			logChan <- nativeLogMessage{Level: zerolog.InfoLevel, Message: "[ffmpeg] " + line}
			// 可解析分辨率/状态，但为简化暂不处理
		}
	}()

	// 读取 stdout，按 AUD 切帧
	readerWG.Add(1)
	go func() {
		defer readerWG.Done()
		// 防御：避免读协程 panic 导致进程退出
		defer func() {
			if r := recover(); r != nil {
				logChan <- nativeLogMessage{Level: zerolog.WarnLevel, Message: fmt.Sprintf("video reader recovered: %v", r)}
			}
			if ffmpegStdout != nil {
				_ = ffmpegStdout.Close()
			}
		}()

		var buf bytes.Buffer
		startCode := []byte{0x00, 0x00, 0x00, 0x01}
		// 估算 duration
		frameDuration := time.Second / time.Duration(max(1, targetFPS))

		tmp := make([]byte, 64*1024)
		for {
			n, err := ffmpegStdout.Read(tmp)
			if n > 0 {
				buf.Write(tmp[:n])
				// 搜索 AUD 作为帧边界
				for {
					data := buf.Bytes()
					idx := indexAUD(data, startCode)
					if idx <= 0 {
						break
					}
					// 从起始到 AUD 前为一帧（跳过首个 AUD头）
					frame := data[:idx]
					if len(frame) > 0 {
						if videoFrameChan != nil {
							videoFrameChan <- append([]byte{}, frame...)
						}
					}
					// 丢弃已消费
					buf.Next(idx)
				}
				// 如果没有 AUD，则可按时间推送整体（退化处理）
				if buf.Len() > 256*1024 {
					if videoFrameChan != nil {
						videoFrameChan <- buf.Next(buf.Len())
					} else {
						_ = buf.Next(buf.Len())
					}
				}
				// 发送状态（低频）
				currentState.Ready = true
				currentState.FramePerSecond = float64(targetFPS)
				if videoStateChan != nil {
					videoStateChan <- currentState
				}
				time.Sleep(frameDuration)
			}
			if err != nil {
				break
			}
		}
	}()

	currentState.Ready = true
	currentState.Error = ""
	if videoStateChan != nil {
		videoStateChan <- currentState
	}
}

func indexAUD(b []byte, sc []byte) int {
	// 查找下一个 start code，检测其后首字节NAL type是否为 AUD(9)
	for i := 0; ; {
		j := bytes.Index(b[i:], sc)
		if j < 0 {
			return -1
		}
		pos := i + j
		// 读取 NAL header
		if pos+len(sc) < len(b) {
			h := b[pos+len(sc)]
			if h&0x1F == 9 {
				// 找到 AUD，返回其位置
				return pos
			}
		}
		i = pos + len(sc)
		if i >= len(b) {
			return -1
		}
	}
}

func videoStop() {
	streamLock.Lock()
	defer streamLock.Unlock()

	if ffmpegCancel != nil {
		ffmpegCancel()
	}
	if ffmpegCmd != nil {
		_ = ffmpegCmd.Process.Kill()
		_ = ffmpegCmd.Wait()
	}
	ffmpegCmd = nil
	ffmpegCancel = nil
	ffmpegStdout = nil

	currentState.Ready = false
	if videoStateChan != nil {
		videoStateChan <- currentState
	}
}

func videoLogStatus() string {
	return fmt.Sprintf("device=%s fps=%d bitrate=%s ready=%t", videoDevice, targetFPS, targetBitrate, currentState.Ready)
}

func videoGetStreamQualityFactor() (float64, error) {
	return qualityFactor, nil
}

func videoSetStreamQualityFactor(factor float64) error {
	// 简单线性映射：基础 4M，factor 0.5~2.0 -> 2M~8M
	if factor < 0.5 {
		factor = 0.5
	}
	if factor > 2.0 {
		factor = 2.0
	}
	qualityFactor = factor
	mbps := int(4 * factor)
	if mbps < 2 {
		mbps = 2
	}
	if mbps > 12 {
		mbps = 12
	}
	targetBitrate = fmt.Sprintf("%dM", mbps)

	// 若正在运行，重启管线以应用码率
	if ffmpegCmd != nil {
		videoStop()
		videoStart()
	}
	return nil
}

func videoGetEDID() (string, error) {
	// 采集卡场景通常不可设置 EDID
	return "", nil
}

func videoSetEDID(edid string) error {
	// 不支持，忽略
	return nil
}

func crash() {
	// 测试用，x86 无 cgo 崩溃通道
	panic("crash invoked")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}