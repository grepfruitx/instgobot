package store

type User struct {
	ID            uint `gorm:"primaryKey;autoIncrement"`
	Username      *string
	FirstName     *string `gorm:"column:first_name"`
	ChatID        int64   `gorm:"column:chat_id;uniqueIndex;not null"`
	FirstSeen     string  `gorm:"column:first_seen;not null"`
	LastActivity  string  `gorm:"column:last_activity;not null"`
	DownloadCount int64   `gorm:"column:download_count;default:0"`
	ErrorCount    int64   `gorm:"column:error_count;default:0"`
	Newsletter    bool    `gorm:"column:newsletter;default:true"`
}

func (User) TableName() string { return "users" }

type Download struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	UserID    uint   `gorm:"column:user_id;not null;index:idx_downloads_user_id"`
	URL       string `gorm:"column:url;not null"`
	Platform  string `gorm:"column:platform;not null"`
	MediaType string `gorm:"column:media_type;not null"`
	Success   bool   `gorm:"column:success;not null"`
	Timestamp string `gorm:"column:timestamp;not null"`
}

func (Download) TableName() string { return "downloads" }

type ErrorLog struct {
	ID              uint    `gorm:"primaryKey;autoIncrement"`
	UserID          uint    `gorm:"column:user_id;not null;index:idx_errors_user_id"`
	ErrorContext    string  `gorm:"column:error_context;not null"`
	ErrorMessage    string  `gorm:"column:error_message;not null"`
	OriginalMessage *string `gorm:"column:original_message"`
	Timestamp       string  `gorm:"column:timestamp;not null"`
}

func (ErrorLog) TableName() string { return "errors" }

type PlatformStatus struct {
	Platform string `gorm:"column:platform;primaryKey"`
	Disabled bool   `gorm:"column:disabled;not null;default:false"`
}

func (PlatformStatus) TableName() string { return "platform_status" }

type MediaCache struct {
	PostURL    string `gorm:"column:post_url;primaryKey"`
	MediaType  string `gorm:"column:media_type;primaryKey"`
	MediaIndex int    `gorm:"column:media_index;primaryKey"`
	FileID     string `gorm:"column:file_id;not null"`
}

func (MediaCache) TableName() string { return "media_cache" }
