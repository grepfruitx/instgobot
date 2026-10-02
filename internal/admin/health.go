package admin

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func diskFree() (string, bool) {
	var stat unix.Statfs_t
	if err := unix.Statfs("/", &stat); err != nil {
		return "", false
	}
	freeGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
	totalGB := float64(stat.Blocks*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%.1fGB свободно из %.1fGB", freeGB, totalGB), true
}

func memoryRSS() (string, bool) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return "", false
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return "", false
		}
		return fmt.Sprintf("%.1fMB", float64(kb)/1024), true
	}
	return "", false
}

func (h *Handler) handleHealth(ctx context.Context, chatID int64) {
	var sb strings.Builder
	sb.WriteString("Здоровье бота:\n\n")

	if h.uc.Connected() {
		sb.WriteString("Userbot: подключён\n")
	} else {
		sb.WriteString("Userbot: не подключён (сторисы/приватные посты не работают)\n")
	}

	if disk, ok := diskFree(); ok {
		fmt.Fprintf(&sb, "Диск: %s\n", disk)
	} else {
		sb.WriteString("Диск: не удалось определить\n")
	}

	if mem, ok := memoryRSS(); ok {
		fmt.Fprintf(&sb, "Память процесса: %s\n", mem)
	} else {
		sb.WriteString("Память процесса: не удалось определить\n")
	}

	disabled, err := h.st.GetDisabledPlatforms()
	if err != nil {
		fmt.Fprintf(&sb, "Платформы: ошибка получения статуса (%v)\n", err)
	} else if len(disabled) == 0 {
		sb.WriteString("Платформы: все включены\n")
	} else {
		fmt.Fprintf(&sb, "Платформы отключены: %s\n", strings.Join(disabled, ", "))
	}

	h.send(ctx, chatID, sb.String())
}
