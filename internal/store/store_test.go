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
	if stats.Downloads24h != 1 {
		t.Fatalf("expected 1 download in the last 24h, got %+v", stats)
	}
	if stats.Errors24h != 0 {
		t.Fatalf("expected 0 errors in the last 24h, got %+v", stats)
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

func TestClearCacheMatchesRawAndNormalized(t *testing.T) {
	s := newTestStore(t)

	if err := s.SetCachedFileID("https://example.com/p", "video", 0, "file1"); err != nil {
		t.Fatalf("SetCachedFileID: %v", err)
	}
	if err := s.SetCachedFileID("https://youtube.com/watch?v=abc", "yt_v_720", 0, "file2"); err != nil {
		t.Fatalf("SetCachedFileID: %v", err)
	}

	rows, err := s.ClearCache("https://example.com/p/")
	if err != nil {
		t.Fatalf("ClearCache: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 row cleared via normalized match, got %d", rows)
	}

	rows, err = s.ClearCache("https://youtube.com/watch?v=abc")
	if err != nil {
		t.Fatalf("ClearCache: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 row cleared via raw match, got %d", rows)
	}
}

func TestBanUnbanUser(t *testing.T) {
	s := newTestStore(t)

	banned, err := s.IsBanned(800)
	if err != nil || banned {
		t.Fatalf("expected not banned by default, got %v err=%v", banned, err)
	}

	if err := s.BanUser(800); err != nil {
		t.Fatalf("BanUser: %v", err)
	}
	banned, err = s.IsBanned(800)
	if err != nil || !banned {
		t.Fatalf("expected banned, got %v err=%v", banned, err)
	}

	users, err := s.GetBannedUsers()
	if err != nil || len(users) != 1 || users[0].ChatID != 800 {
		t.Fatalf("unexpected banned users list: %+v err=%v", users, err)
	}

	if err := s.UnbanUser(800); err != nil {
		t.Fatalf("UnbanUser: %v", err)
	}
	banned, err = s.IsBanned(800)
	if err != nil || banned {
		t.Fatalf("expected unbanned, got %v err=%v", banned, err)
	}
}

func TestRateLimitHits(t *testing.T) {
	s := newTestStore(t)

	if err := s.RecordRateLimitHit("general"); err != nil {
		t.Fatalf("RecordRateLimitHit: %v", err)
	}
	if err := s.RecordRateLimitHit("general"); err != nil {
		t.Fatalf("RecordRateLimitHit: %v", err)
	}
	if err := s.RecordRateLimitHit("youtube"); err != nil {
		t.Fatalf("RecordRateLimitHit: %v", err)
	}

	hits, err := s.GetRateLimitHits()
	if err != nil {
		t.Fatalf("GetRateLimitHits: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 kinds, got %d: %+v", len(hits), hits)
	}
	if hits[0].Kind != "general" || hits[0].Count != 2 {
		t.Fatalf("unexpected general hits: %+v", hits[0])
	}
	if hits[1].Kind != "youtube" || hits[1].Count != 1 {
		t.Fatalf("unexpected youtube hits: %+v", hits[1])
	}
}

func TestGetStatsErrors24h(t *testing.T) {
	s := newTestStore(t)

	if err := s.RecordError(900, "youtube download", "boom", nil, nil, nil); err != nil {
		t.Fatalf("RecordError: %v", err)
	}

	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.Errors24h != 1 {
		t.Fatalf("expected 1 error in the last 24h, got %+v", stats)
	}
}

func TestPlatformWaitlist(t *testing.T) {
	s := newTestStore(t)

	if err := s.JoinWaitlist(1000, "tiktok"); err != nil {
		t.Fatalf("JoinWaitlist: %v", err)
	}
	if err := s.JoinWaitlist(1001, "tiktok"); err != nil {
		t.Fatalf("JoinWaitlist: %v", err)
	}
	if err := s.JoinWaitlist(1000, "tiktok"); err != nil {
		t.Fatalf("JoinWaitlist (duplicate join): %v", err)
	}
	if err := s.JoinWaitlist(2000, "instagram"); err != nil {
		t.Fatalf("JoinWaitlist: %v", err)
	}

	waiting, err := s.GetWaitlist("tiktok")
	if err != nil {
		t.Fatalf("GetWaitlist: %v", err)
	}
	if len(waiting) != 2 {
		t.Fatalf("expected 2 waiting on tiktok (duplicate join should not double-add), got %v", waiting)
	}

	if err := s.ClearWaitlist("tiktok"); err != nil {
		t.Fatalf("ClearWaitlist: %v", err)
	}
	waiting, err = s.GetWaitlist("tiktok")
	if err != nil {
		t.Fatalf("GetWaitlist: %v", err)
	}
	if len(waiting) != 0 {
		t.Fatalf("expected empty tiktok waitlist after clear, got %v", waiting)
	}

	stillWaiting, err := s.GetWaitlist("instagram")
	if err != nil {
		t.Fatalf("GetWaitlist: %v", err)
	}
	if len(stillWaiting) != 1 || stillWaiting[0] != 2000 {
		t.Fatalf("expected instagram waitlist untouched by tiktok clear, got %v", stillWaiting)
	}
}

func TestPostCacheManifest(t *testing.T) {
	s := newTestStore(t)

	if _, ok, err := s.GetPostCache("https://example.com/p/abc"); err != nil || ok {
		t.Fatalf("expected a miss on an unknown post, got ok=%v err=%v", ok, err)
	}

	if err := s.SetPostCache("https://example.com/p/abc", 2, 1, "hello"); err != nil {
		t.Fatalf("SetPostCache: %v", err)
	}

	pc, ok, err := s.GetPostCache("https://example.com/p/abc")
	if err != nil || !ok {
		t.Fatalf("expected a hit, got ok=%v err=%v", ok, err)
	}
	if pc.PhotoCount != 2 || pc.VideoCount != 1 || pc.PostText != "hello" || !pc.HasMedia() {
		t.Fatalf("unexpected manifest: %+v", pc)
	}

	// re-caching the same post must overwrite, not duplicate or fail
	if err := s.SetPostCache("https://example.com/p/abc", 0, 0, "only text"); err != nil {
		t.Fatalf("SetPostCache (upsert): %v", err)
	}
	pc, _, _ = s.GetPostCache("https://example.com/p/abc")
	if pc.PhotoCount != 0 || pc.VideoCount != 0 || pc.PostText != "only text" || pc.HasMedia() {
		t.Fatalf("expected the manifest to be overwritten, got %+v", pc)
	}
}

func TestClearCacheAlsoDropsManifest(t *testing.T) {
	s := newTestStore(t)

	if err := s.SetCachedFileID("https://www.instagram.com/p/ABC", "photo", 0, "file1"); err != nil {
		t.Fatalf("SetCachedFileID: %v", err)
	}
	if err := s.SetPostCache("https://www.instagram.com/p/ABC", 1, 0, ""); err != nil {
		t.Fatalf("SetPostCache: %v", err)
	}

	// admin pastes the link with the query string still attached
	rows, err := s.ClearCache("https://www.instagram.com/p/ABC/?img_index=2")
	if err != nil {
		t.Fatalf("ClearCache: %v", err)
	}
	if rows != 2 {
		t.Fatalf("expected both the file_id and the manifest cleared, got %d rows", rows)
	}
	if _, ok, _ := s.GetPostCache("https://www.instagram.com/p/ABC"); ok {
		t.Fatal("manifest survived ClearCache")
	}
}
