package store

import (
	"errors"
	"log/slog"

	"gorm.io/gorm"
)

func (s *Store) UpsertUser(chatID int64, username, firstName *string) (uint, error) {
	now := nowISO()

	var u User
	err := s.db.Where("chat_id = ?", chatID).First(&u).Error
	if err == nil {
		if err := s.db.Model(&User{}).Where("id = ?", u.ID).Updates(map[string]any{
			"username":      username,
			"first_name":    firstName,
			"last_activity": now,
		}).Error; err != nil {
			return 0, err
		}
		return u.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	u = User{Username: username, FirstName: firstName, ChatID: chatID, FirstSeen: now, LastActivity: now}
	if err := s.db.Create(&u).Error; err != nil {
		return 0, err
	}
	return u.ID, nil
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
	var u User
	err := s.db.Select("newsletter").Where("chat_id = ?", chatID).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	newStatus := !u.Newsletter
	if err := s.db.Model(&User{}).Where("chat_id = ?", chatID).Update("newsletter", newStatus).Error; err != nil {
		return false, err
	}
	return newStatus, nil
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
