package youtube

import (
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

const (
	maxConcurrentAdaptive = 2
	minFreeDiskBytes      = 1500 * 1024 * 1024
)

var (
	adaptiveMu     sync.Mutex
	activeAdaptive int

	activeDownloadsMu sync.Mutex
	activeDownloads   = map[int64]struct{}{}
)

func hasEnoughDiskSpace() bool {
	var stat unix.Statfs_t
	if err := unix.Statfs(os.TempDir(), &stat); err != nil {
		return true
	}
	return stat.Bavail*uint64(stat.Bsize) > minFreeDiskBytes
}

func acquireAdaptiveSlot() bool {
	adaptiveMu.Lock()
	defer adaptiveMu.Unlock()
	if activeAdaptive >= maxConcurrentAdaptive {
		return false
	}
	activeAdaptive++
	return true
}

func releaseAdaptiveSlot() {
	adaptiveMu.Lock()
	defer adaptiveMu.Unlock()
	activeAdaptive--
}

func markDownloadActive(userID int64) bool {
	activeDownloadsMu.Lock()
	defer activeDownloadsMu.Unlock()
	if _, ok := activeDownloads[userID]; ok {
		return false
	}
	activeDownloads[userID] = struct{}{}
	return true
}

func markDownloadDone(userID int64) {
	activeDownloadsMu.Lock()
	defer activeDownloadsMu.Unlock()
	delete(activeDownloads, userID)
}
