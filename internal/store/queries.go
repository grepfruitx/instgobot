package store

import (
	"errors"
	"log/slog"
	"strings"

	"gorm.io/gorm"
)

func (s *Store) UpsertUser(chatID int64, username, firstName *string) (uint, error) {
	now := nowISO()

	var id uint
	err := s.db.Raw(`
		INSERT INTO users (username, first_name, chat_id, first_seen, last_activity)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (chat_id) DO UPDATE SET
			username = excluded.username,
			first_name = excluded.first_name,
			last_activity = excluded.last_activity
		RETURNING id
	`, username, firstName, chatID, now, now).Scan(&id).Error
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) RecordDownload(chatID int64, url, platform, mediaType string, success bool, username, firstName *string) error {
	userID, err := s.UpsertUser(chatID, username, firstName)
	if err != nil {
		return err
	}

	d := Download{UserID: userID, URL: url, Platform: platform, MediaType: mediaType, Success: success, Timestamp: nowISO()}
	if err := s.db.Create(&d).Error; err != nil {
		return err
	}

	if success {
		return s.db.Model(&User{}).Where("id = ?", userID).UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error
	}
	return nil
}

func (s *Store) RecordDownloadLogged(chatID int64, url, platform, mediaType string, success bool, username, firstName *string) {
	if err := s.RecordDownload(chatID, url, platform, mediaType, success, username, firstName); err != nil {
		slog.Error("record download failed", "error", err, "chat_id", chatID)
	}
}

func (s *Store) RecordError(chatID int64, errorContext, errorMessage string, originalMessage, username, firstName *string) error {
	userID, err := s.UpsertUser(chatID, username, firstName)
	if err != nil {
		return err
	}

	e := ErrorLog{UserID: userID, ErrorContext: errorContext, ErrorMessage: errorMessage, OriginalMessage: originalMessage, Timestamp: nowISO()}
	if err := s.db.Create(&e).Error; err != nil {
		return err
	}

	return s.db.Model(&User{}).Where("id = ?", userID).UpdateColumn("error_count", gorm.Expr("error_count + 1")).Error
}

func (s *Store) GetUsers(limit int) ([]User, error) {
	var users []User
	err := s.db.Order("download_count desc, last_activity desc").Limit(limit).Find(&users).Error
	return users, err
}

type Stats struct {
	TotalUsers     int64
	TotalDownloads int64
	TotalErrors    int64
	ActiveUsers24h int64
}

func (s *Store) GetStats() (Stats, error) {
	var stats Stats
	if err := s.db.Model(&User{}).Count(&stats.TotalUsers).Error; err != nil {
		return stats, err
	}
	if err := s.db.Model(&Download{}).Where("success = ?", true).Count(&stats.TotalDownloads).Error; err != nil {
		return stats, err
	}
	if err := s.db.Model(&ErrorLog{}).Count(&stats.TotalErrors).Error; err != nil {
		return stats, err
	}
	err := s.db.Raw(`SELECT COUNT(*) FROM users WHERE datetime(last_activity) > datetime('now', '-24 hours')`).Scan(&stats.ActiveUsers24h).Error
	return stats, err
}

func (s *Store) GetTopUsers(limit int) ([]User, error) {
	var users []User
	err := s.db.Where("download_count > 0").Order("download_count desc").Limit(limit).Find(&users).Error
	return users, err
}

type RecentError struct {
	ID              uint
	UserID          int64 `gorm:"column:user_id"`
	ErrorContext    string
	ErrorMessage    string
	OriginalMessage *string
	Timestamp       string
	Username        *string
	FirstName       *string
	ChatID          int64
}

func (s *Store) GetRecentErrors(limit int) ([]RecentError, error) {
	var results []RecentError
	err := s.db.Raw(`
		SELECT e.*, u.username, u.first_name, u.chat_id
		FROM errors e
		JOIN users u ON e.user_id = u.id
		ORDER BY e.timestamp DESC
		LIMIT ?
	`, limit).Scan(&results).Error
	return results, err
}

type PlatformStat struct {
	Platform            string
	TotalRequests       int64
	SuccessfulDownloads int64
	SuccessRate         float64
}

func (s *Store) GetPlatformStats() ([]PlatformStat, error) {
	var results []PlatformStat
	err := s.db.Raw(`
		SELECT
			platform,
			COUNT(*) as total_requests,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as successful_downloads,
			ROUND((SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) * 100.0 / COUNT(*)), 2) as success_rate
		FROM downloads
		GROUP BY platform
		ORDER BY total_requests DESC
	`).Scan(&results).Error
	return results, err
}

type ErrorCluster struct {
	ErrorMessage string
	Count        int64
}

func (s *Store) GetTopErrorMessages(limit int) ([]ErrorCluster, error) {
	var results []ErrorCluster
	err := s.db.Raw(`
		SELECT error_message, COUNT(*) as count
		FROM errors
		GROUP BY error_message
		ORDER BY count DESC
		LIMIT ?
	`, limit).Scan(&results).Error
	return results, err
}

func (s *Store) RecordCacheEvent(platform string, hit bool) error {
	if hit {
		return s.db.Exec(`
			INSERT INTO cache_stats (platform, hits, misses) VALUES (?, 1, 0)
			ON CONFLICT (platform) DO UPDATE SET hits = hits + 1
		`, platform).Error
	}
	return s.db.Exec(`
		INSERT INTO cache_stats (platform, hits, misses) VALUES (?, 0, 1)
		ON CONFLICT (platform) DO UPDATE SET misses = misses + 1
	`, platform).Error
}

func (s *Store) GetCacheStats() ([]CacheStats, error) {
	var results []CacheStats
	err := s.db.Order("platform").Find(&results).Error
	return results, err
}

type RetentionStats struct {
	NewUsersToday    int64
	NewUsersThisWeek int64
	InactiveOver7d   int64
	InactiveOver30d  int64
}

func (s *Store) GetRetentionStats() (RetentionStats, error) {
	var r RetentionStats
	if err := s.db.Raw(`SELECT COUNT(*) FROM users WHERE datetime(first_seen) > datetime('now', '-1 day')`).Scan(&r.NewUsersToday).Error; err != nil {
		return r, err
	}
	if err := s.db.Raw(`SELECT COUNT(*) FROM users WHERE datetime(first_seen) > datetime('now', '-7 days')`).Scan(&r.NewUsersThisWeek).Error; err != nil {
		return r, err
	}
	if err := s.db.Raw(`SELECT COUNT(*) FROM users WHERE datetime(last_activity) <= datetime('now', '-7 days')`).Scan(&r.InactiveOver7d).Error; err != nil {
		return r, err
	}
	if err := s.db.Raw(`SELECT COUNT(*) FROM users WHERE datetime(last_activity) <= datetime('now', '-30 days')`).Scan(&r.InactiveOver30d).Error; err != nil {
		return r, err
	}
	return r, nil
}

type HourActivity struct {
	Hour  int
	Count int64
}

func (s *Store) GetActivityByHour() ([]HourActivity, error) {
	var results []HourActivity
	err := s.db.Raw(`
		SELECT CAST(strftime('%H', datetime(timestamp, '+3 hours')) AS INTEGER) as hour, COUNT(*) as count
		FROM downloads
		GROUP BY hour
		ORDER BY hour
	`).Scan(&results).Error
	return results, err
}

type WeekdayActivity struct {
	Weekday int // 0=Sunday..6=Saturday, Moscow-local via the same +3h shift as GetActivityByHour
	Count   int64
}

func (s *Store) GetActivityByWeekday() ([]WeekdayActivity, error) {
	var results []WeekdayActivity
	err := s.db.Raw(`
		SELECT CAST(strftime('%w', datetime(timestamp, '+3 hours')) AS INTEGER) as weekday, COUNT(*) as count
		FROM downloads
		GROUP BY weekday
		ORDER BY weekday
	`).Scan(&results).Error
	return results, err
}

type NewsletterUser struct {
	ChatID    int64 `gorm:"column:chat_id"`
	Username  *string
	FirstName *string `gorm:"column:first_name"`
}

func (s *Store) GetAllUsers() ([]NewsletterUser, error) {
	var results []NewsletterUser
	err := s.db.Model(&User{}).
		Select("chat_id", "username", "first_name").
		Where("newsletter = ?", true).
		Order("last_activity desc").
		Find(&results).Error
	return results, err
}

func (s *Store) ToggleNewsletterSubscription(chatID int64) (bool, error) {
	var newStatus bool
	err := s.db.Raw(`
		UPDATE users SET newsletter = NOT newsletter WHERE chat_id = ?
		RETURNING newsletter
	`, chatID).Scan(&newStatus).Error
	return newStatus, err
}

func (s *Store) GetNewsletterStatus(chatID int64) (bool, error) {
	var u User
	err := s.db.Select("newsletter").Where("chat_id = ?", chatID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return u.Newsletter, nil
}

type NewsletterStats struct {
	Total        int64
	Subscribed   int64
	Unsubscribed int64
}

func (s *Store) GetNewsletterStats() (NewsletterStats, error) {
	var stats NewsletterStats
	err := s.db.Raw(`
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN newsletter = 1 THEN 1 ELSE 0 END) as subscribed,
			SUM(CASE WHEN newsletter = 0 THEN 1 ELSE 0 END) as unsubscribed
		FROM users
	`).Scan(&stats).Error
	return stats, err
}

func (s *Store) SetPlatformDisabled(platform string, disabled bool) error {
	return s.db.Exec(`
		INSERT INTO platform_status (platform, disabled) VALUES (?, ?)
		ON CONFLICT (platform) DO UPDATE SET disabled = excluded.disabled
	`, platform, disabled).Error
}

func (s *Store) IsPlatformDisabled(platform string) (bool, error) {
	var ps PlatformStatus
	err := s.db.Where("platform = ?", platform).First(&ps).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return ps.Disabled, nil
}

func (s *Store) GetDisabledPlatforms() ([]string, error) {
	var platforms []string
	err := s.db.Model(&PlatformStatus{}).Where("disabled = ?", true).Pluck("platform", &platforms).Error
	return platforms, err
}

func (s *Store) GetCachedFileID(postURL, mediaType string, index int) (string, bool, error) {
	var mc MediaCache
	err := s.db.Where("post_url = ? AND media_type = ? AND media_index = ?", postURL, mediaType, index).First(&mc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return mc.FileID, true, nil
}

func (s *Store) SetCachedFileID(postURL, mediaType string, index int, fileID string) error {
	return s.db.Exec(`
		INSERT OR REPLACE INTO media_cache (post_url, media_type, media_index, file_id) VALUES (?, ?, ?, ?)
	`, postURL, mediaType, index, fileID).Error
}

func (s *Store) ClearCache(postURL string) (int64, error) {
	normalized := strings.TrimSuffix(strings.Split(postURL, "?")[0], "/")
	res := s.db.Exec(`DELETE FROM media_cache WHERE post_url = ? OR post_url = ?`, postURL, normalized)
	return res.RowsAffected, res.Error
}

func (s *Store) BanUser(chatID int64) error {
	return s.db.Exec(`
		INSERT INTO banned_users (chat_id, banned_at) VALUES (?, ?)
		ON CONFLICT (chat_id) DO NOTHING
	`, chatID, nowISO()).Error
}

func (s *Store) UnbanUser(chatID int64) error {
	return s.db.Exec(`DELETE FROM banned_users WHERE chat_id = ?`, chatID).Error
}

func (s *Store) IsBanned(chatID int64) (bool, error) {
	var count int64
	err := s.db.Model(&BannedUser{}).Where("chat_id = ?", chatID).Count(&count).Error
	return count > 0, err
}

func (s *Store) GetBannedUsers() ([]BannedUser, error) {
	var users []BannedUser
	err := s.db.Order("banned_at desc").Find(&users).Error
	return users, err
}

func (s *Store) RecordRateLimitHit(kind string) error {
	return s.db.Exec(`
		INSERT INTO rate_limit_hits (kind, count) VALUES (?, 1)
		ON CONFLICT (kind) DO UPDATE SET count = count + 1
	`, kind).Error
}

func (s *Store) GetRateLimitHits() ([]RateLimitHit, error) {
	var results []RateLimitHit
	err := s.db.Order("kind").Find(&results).Error
	return results, err
}
