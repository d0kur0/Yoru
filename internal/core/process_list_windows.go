package core

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"unsafe"
)

func ListProcesses(ctx context.Context) ([]ProcessInfo, error) {
	snapshot, e := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if e != nil {
		return nil, e
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	items := []ProcessInfo{}
	for e = windows.Process32First(snapshot, &entry); e == nil; e = windows.Process32Next(snapshot, &entry) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if entry.ProcessID == 0 {
			continue
		}
		item := ProcessInfo{PID: int(entry.ProcessID), Name: windows.UTF16ToString(entry.ExeFile[:])}
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
		if err == nil {
			buf := make([]uint16, 32768)
			n := uint32(len(buf))
			if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) == nil {
				item.Path = windows.UTF16ToString(buf[:n])
			}
			windows.CloseHandle(h)
		}
		items = append(items, item)
	}
	if !errors.Is(e, windows.ERROR_NO_MORE_FILES) {
		return nil, e
	}
	return sortProcesses(items), nil
}
