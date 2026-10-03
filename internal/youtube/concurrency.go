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

	activeJobs sync.WaitGroup
)

func WaitForActiveDownloads() {
	activeJobs.Wait()
}

// hasEnoughDiskSpace reserves room for a merge: yt-dlp keeps both per-format
// intermediates on disk while writing the merged output, so ~2x the size.
func hasEnoughDiskSpace(expectedSize int64) bool {
	var stat unix.Statfs_t
	if err := unix.Statfs(os.TempDir(), &stat); err != nil {
		return true
	}
	return stat.Bavail*uint64(stat.Bsize) > minFreeDiskBytes+2*uint64(max(expectedSize, 0))
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
