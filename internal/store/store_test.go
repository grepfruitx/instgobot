package store

import (
	"path/filepath"
	"sync"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func strp(s string) *string { return &s }

func TestUpsertUserInsertsThenUpdates(t *testing.T) {
	s := newTestStore(t)

	id1, err := s.UpsertUser(100, strp("alice"), strp("Alice"))
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	id2, err := s.UpsertUser(100, strp("alice2"), strp("Alice2"))
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("expected same user id, got %d and %d", id1, id2)
	}

	users, err := s.GetUsers(10)
	if err != nil {
		t.Fatalf("GetUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if *users[0].Username != "alice2" {
		t.Fatalf("expected username alice2, got %s", *users[0].Username)
	}
}

func TestRecordDownloadIncrementsCount(t *testing.T) {
	s := newTestStore(t)

	if err := s.RecordDownload(200, "https://example.com/x", "instagram", "video", true, nil, nil); err != nil {
		t.Fatalf("RecordDownload: %v", err)
	}
	if err := s.RecordDownload(200, "https://example.com/y", "instagram", "video", false, nil, nil); err != nil {
		t.Fatalf("RecordDownload: %v", err)
	}

	users, err := s.GetTopUsers(10)
	if err != nil {
		t.Fatalf("GetTopUsers: %v", err)
	}
	if len(users) != 1 || users[0].DownloadCount != 1 {
		t.Fatalf("expected 1 user with download_count=1, got %+v", users)
	}

	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalDownloads != 1 || stats.TotalUsers != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	platforms, err := s.GetPlatformStats()
	if err != nil {
		t.Fatalf("GetPlatformStats: %v", err)
	}
	if len(platforms) != 1 || platforms[0].TotalRequests != 2 || platforms[0].SuccessfulDownloads != 1 {
		t.Fatalf("unexpected platform stats: %+v", platforms)
	}
}

func TestNewsletterToggle(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.UpsertUser(300, nil, nil); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	status, err := s.GetNewsletterStatus(300)
	if err != nil || !status {
		t.Fatalf("expected default subscribed, got %v err=%v", status, err)
	}

	newStatus, err := s.ToggleNewsletterSubscription(300)
	if err != nil {
		t.Fatalf("ToggleNewsletterSubscription: %v", err)
	}
	if newStatus {
		t.Fatalf("expected unsubscribed after toggle, got subscribed")
	}

	ok, err := s.ToggleNewsletterSubscription(999)
	if err != nil {
		t.Fatalf("ToggleNewsletterSubscription: %v", err)
	}
	if ok {
		t.Fatalf("expected false for unknown chat id")
	}
}

func TestPlatformDisabled(t *testing.T) {
	s := newTestStore(t)

	disabled, err := s.IsPlatformDisabled("tiktok")
	if err != nil || disabled {
		t.Fatalf("expected not disabled by default, got %v err=%v", disabled, err)
	}

	if err := s.SetPlatformDisabled("tiktok", true); err != nil {
		t.Fatalf("SetPlatformDisabled: %v", err)
	}
	disabled, err = s.IsPlatformDisabled("tiktok")
	if err != nil || !disabled {
		t.Fatalf("expected disabled, got %v err=%v", disabled, err)
	}

	platforms, err := s.GetDisabledPlatforms()
	if err != nil || len(platforms) != 1 || platforms[0] != "tiktok" {
		t.Fatalf("unexpected disabled platforms: %v err=%v", platforms, err)
	}
}

func TestMediaCache(t *testing.T) {
	s := newTestStore(t)

	_, ok, err := s.GetCachedFileID("https://example.com/p", "video", 0)
	if err != nil || ok {
		t.Fatalf("expected cache miss, got ok=%v err=%v", ok, err)
	}

	if err := s.SetCachedFileID("https://example.com/p", "video", 0, "file123"); err != nil {
		t.Fatalf("SetCachedFileID: %v", err)
	}

	fileID, ok, err := s.GetCachedFileID("https://example.com/p", "video", 0)
	if err != nil || !ok || fileID != "file123" {
		t.Fatalf("expected cache hit file123, got %q ok=%v err=%v", fileID, ok, err)
	}
}

func TestConcurrentWritesDoNotFailWithBusy(t *testing.T) {
	s := newTestStore(t)

	const n = 100
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			chatID := int64(1000 + i%10) // 10 distinct users, 10 writes each — real contention
			err := s.RecordDownload(chatID, "https://example.com/x", "instagram", "video", true, nil, nil)
			errCh <- err
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent RecordDownload failed: %v", err)
		}
	}

	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalDownloads != n {
		t.Fatalf("expected %d downloads recorded, got %d", n, stats.TotalDownloads)
	}
}

func TestConcurrentTogglesAreAtomic(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.UpsertUser(400, nil, nil); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	const n = 100 // even, so a lost update would flip the final state
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.ToggleNewsletterSubscription(400)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent ToggleNewsletterSubscription failed: %v", err)
		}
	}

	status, err := s.GetNewsletterStatus(400)
	if err != nil {
		t.Fatalf("GetNewsletterStatus: %v", err)
	}
	if !status {
		t.Fatalf("expected even number of concurrent toggles to return to default (subscribed), got %v", status)
	}
}

func TestGetTopErrorMessages(t *testing.T) {
	s := newTestStore(t)

	if err := s.RecordError(500, "youtube download", "boom", nil, nil, nil); err != nil {
		t.Fatalf("RecordError: %v", err)
	}
	if err := s.RecordError(500, "youtube download", "boom", nil, nil, nil); err != nil {
		t.Fatalf("RecordError: %v", err)
	}
	if err := s.RecordError(500, "youtube download", "other error", nil, nil, nil); err != nil {
		t.Fatalf("RecordError: %v", err)
	}

	clusters, err := s.GetTopErrorMessages(10)
	if err != nil {
		t.Fatalf("GetTopErrorMessages: %v", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("expected 2 distinct error messages, got %d: %+v", len(clusters), clusters)
	}
	if clusters[0].ErrorMessage != "boom" || clusters[0].Count != 2 {
		t.Fatalf("expected top cluster to be 'boom' x2, got %+v", clusters[0])
	}
}

func TestCacheStats(t *testing.T) {
	s := newTestStore(t)

	if err := s.RecordCacheEvent("youtube", true); err != nil {
		t.Fatalf("RecordCacheEvent: %v", err)
	}
	if err := s.RecordCacheEvent("youtube", true); err != nil {
		t.Fatalf("RecordCacheEvent: %v", err)
	}
	if err := s.RecordCacheEvent("youtube", false); err != nil {
		t.Fatalf("RecordCacheEvent: %v", err)
	}
	if err := s.RecordCacheEvent("instagram", false); err != nil {
		t.Fatalf("RecordCacheEvent: %v", err)
	}

	stats, err := s.GetCacheStats()
	if err != nil {
		t.Fatalf("GetCacheStats: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 platforms, got %d: %+v", len(stats), stats)
	}
	if stats[0].Platform != "instagram" || stats[0].Hits != 0 || stats[0].Misses != 1 {
		t.Fatalf("unexpected instagram stats: %+v", stats[0])
	}
	if stats[1].Platform != "youtube" || stats[1].Hits != 2 || stats[1].Misses != 1 {
		t.Fatalf("unexpected youtube stats: %+v", stats[1])
	}
}

func TestGetRetentionStats(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.UpsertUser(600, nil, nil); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	r, err := s.GetRetentionStats()
	if err != nil {
		t.Fatalf("GetRetentionStats: %v", err)
	}
	if r.NewUsersToday != 1 || r.NewUsersThisWeek != 1 {
		t.Fatalf("expected freshly created user to count as new today/this week, got %+v", r)
	}
	if r.InactiveOver7d != 0 || r.InactiveOver30d != 0 {
		t.Fatalf("expected freshly active user to not count as inactive, got %+v", r)
	}
}

func TestGetActivityByHourAndWeekday(t *testing.T) {
	s := newTestStore(t)

	if err := s.RecordDownload(700, "https://example.com/x", "instagram", "video", true, nil, nil); err != nil {
		t.Fatalf("RecordDownload: %v", err)
	}

	hours, err := s.GetActivityByHour()
	if err != nil {
		t.Fatalf("GetActivityByHour: %v", err)
	}
	var hourTotal int64
	for _, h := range hours {
		hourTotal += h.Count
	}
	if hourTotal != 1 {
		t.Fatalf("expected 1 total download across hour buckets, got %d: %+v", hourTotal, hours)
	}

	weekdays, err := s.GetActivityByWeekday()
	if err != nil {
		t.Fatalf("GetActivityByWeekday: %v", err)
	}
	var weekdayTotal int64
	for _, w := range weekdays {
		weekdayTotal += w.Count
	}
	if weekdayTotal != 1 {
		t.Fatalf("expected 1 total download across weekday buckets, got %d: %+v", weekdayTotal, weekdays)
	}
}
