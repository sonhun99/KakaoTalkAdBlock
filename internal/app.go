package internal

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"kakaotalkadblock/internal/win"
	"kakaotalkadblock/internal/win/winapi"

	"golang.org/x/sys/windows"
)

const sleepTime = 100 * time.Millisecond
const executable = "kakaotalk.exe"

var mutex = &sync.Mutex{}
var handles = make([]windows.HWND, 0)

func uint8ToStr(arr []uint8) string {
	n := bytes.Index(arr, []uint8{0})

	return string(arr[:n])
}

func watch() {
	var (
		pe32      winapi.ProcessEntry32
		szExeFile string
	)
	pe32.DwSize = uint32(unsafe.Sizeof(pe32))
	lastFoundAt := time.Now().Unix() - 2
	var snapshot windows.HWND
	var enumWindow = syscall.NewCallback(func(handle windows.HWND, processId uintptr) uintptr {
		winapi.GetWindowThreadProcessId(handle, &pe32.Th32ProcessID)
		if processId == uintptr(pe32.Th32ProcessID) {
			lastFoundAt = time.Now().Unix()
			handles = append(handles, handle)
		}
		return 1
	})

	for {
		mutex.Lock()
		handles = handles[:0]
		if lastFoundAt < time.Now().Unix()-1 {
			snapshot = winapi.CreateToolhelp32Snapshot(winapi.Th32csSnapprocess, 0)
			lastFoundAt = time.Now().Unix()
		}

		if winapi.Process32First(uintptr(snapshot), &pe32) {
			for {
				szExeFile = uint8ToStr(pe32.SzExeFile[:])

				if strings.ToLower(szExeFile) == executable {
					winapi.EnumWindows(enumWindow, uintptr(pe32.Th32ProcessID))
					//break
				}

				if !winapi.Process32Next(uintptr(snapshot), &pe32) {
					break
				}
			}
		}
		mutex.Unlock()
		time.Sleep(sleepTime)
	}
}

func removeAd() {
	childHandles := make([]windows.HWND, 0)

	var enumWindow = syscall.NewCallback(func(handle windows.HWND, _ uintptr) uintptr {
		childHandles = append(childHandles, handle)
		return 1
	})

	loopCount := 0
	for loopCount < 100 {
		mutex.Lock()
		for _, wnd := range handles {
			if wnd == 0 {
				continue
			}
			childHandles = childHandles[:0]
			var handle windows.HWND
			winapi.EnumChildWindows(wnd, enumWindow, uintptr(unsafe.Pointer(&handle)))

			rect := new(winapi.Rect)
			winapi.GetWindowRect(wnd, rect)
			var mainWindowParentHandle windows.HWND
			var candidates [][]windows.HWND
			for _, childHandle := range childHandles {
				className := winapi.GetClassName(childHandle)
				windowText := winapi.GetWindowText(childHandle)
				parentHandle := winapi.GetParent(childHandle)
				if className == "EVA_ChildWindow" {
					if windowText == "" {
						candidates = append(candidates, []windows.HWND{childHandle, parentHandle})
					} else if strings.HasPrefix(windowText, "OnlineMainView") {
						mainWindowParentHandle = parentHandle
					}
				}
				HideMainWindowAd(className, childHandle)
				HideMainViewAdArea(windowText, rect, childHandle)
				HideLockScreenAdArea(windowText, rect, childHandle)
				HidePopupAd(className, childHandle)
			}
			if mainWindowParentHandle != 0 && len(candidates) > 0 {
				for _, candidate := range candidates {
					if candidate[1] == mainWindowParentHandle {
						winapi.ShowWindow(candidate[0], 0)
						winapi.MoveWindow(candidate[0], 0, 0, 0, 0, true)
						break
					}
				}
			}
		}
		mutex.Unlock()
		time.Sleep(sleepTime)
		loopCount++
	}
	os.Exit(0)
}

func Run() {
	var quit = make(chan struct{})
	trayIcon := win.NewTrayIcon(&quit)
	trayIcon.Show()
	defer trayIcon.Hide()
	go watch()
	go removeAd()

	select {
	case <-quit:
		return
	}
}
