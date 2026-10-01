//go:build windows

package main

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Find the actual League install directory, which may be on any drive.
// QueryFullProcessImageName reads only the executable path, never command-line
// arguments or the ephemeral lockfile password.
func runningLeagueClientPaths() ([]string, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, err
	}
	var paths []string
	for {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "LeagueClient.exe") {
			process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
			if err == nil {
				buffer := make([]uint16, windows.MAX_PATH*4)
				size := uint32(len(buffer))
				if err := windows.QueryFullProcessImageName(process, 0, &buffer[0], &size); err == nil {
					paths = append(paths, windows.UTF16ToString(buffer[:size]))
				}
				windows.CloseHandle(process)
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			break
		}
	}
	return paths, nil
}
