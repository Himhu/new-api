package model

// RequestStat — 日志预聚合表
type RequestStat struct {
	Id                int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	BucketStart       int64  `json:"bucket_start" gorm:"bigint;index:idx_rs_bucket_group_channel;index:idx_rs_bucket"`
	GroupName         string `json:"group_name" gorm:"type:varchar(64);index:idx_rs_bucket_group_channel"`
	ChannelId         int    `json:"channel_id" gorm:"index:idx_rs_bucket_group_channel"`
	ModelName         string `json:"model_name" gorm:"type:varchar(255)"`
	TotalRequests     int    `json:"total_requests" gorm:"default:0"`
	SuccessRequests   int    `json:"success_requests" gorm:"default:0"`
	ErrorRequests     int    `json:"error_requests" gorm:"default:0"`
	TotalCacheTokens  int64  `json:"total_cache_tokens" gorm:"bigint;default:0"`
	TotalPromptTokens int64  `json:"total_prompt_tokens" gorm:"bigint;default:0"`
	CacheDataPoints   int    `json:"cache_data_points" gorm:"default:0"`
	SumResponseTime   int64  `json:"sum_response_time" gorm:"bigint;default:0"`
}

// ChannelMonitoringStat — 渠道级快照
type ChannelMonitoringStat struct {
	Id               int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	GroupName        string  `json:"group_name" gorm:"type:varchar(64);uniqueIndex:idx_cms_group_channel"`
	ChannelId        int     `json:"channel_id" gorm:"uniqueIndex:idx_cms_group_channel"`
	ChannelName      string  `json:"channel_name" gorm:"-"`
	ChannelStatus    int     `json:"channel_status" gorm:"-"`
	AvailabilityRate float64 `json:"availability_rate" gorm:"type:decimal(8,4);default:-1"`
	CacheHitRate     float64 `json:"cache_hit_rate" gorm:"type:decimal(8,4);default:-1"`
	LastResponseTime int     `json:"last_response_time" gorm:"default:0"`
	LastFRT          int     `json:"last_frt" gorm:"default:0"`
	LastTestTime     int64   `json:"last_test_time" gorm:"bigint;default:0"`
	LastTestModel    string  `json:"last_test_model" gorm:"type:varchar(255);default:''"`
	IsOnline         bool    `json:"is_online" gorm:"default:false"`
	UpdatedAt        int64   `json:"updated_at" gorm:"bigint"`
}

// GroupMonitoringStat — 分组级快照
type GroupMonitoringStat struct {
	Id               int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	GroupName        string  `json:"group_name" gorm:"type:varchar(64);uniqueIndex:idx_gms_group"`
	AvailabilityRate float64 `json:"availability_rate" gorm:"type:decimal(8,4);default:-1"`
	CacheHitRate     float64 `json:"cache_hit_rate" gorm:"type:decimal(8,4);default:-1"`
	AvgResponseTime  int     `json:"avg_response_time" gorm:"default:0"`
	AvgFRT           int     `json:"avg_frt" gorm:"default:0"`
	OnlineChannels   int     `json:"online_channels" gorm:"default:0"`
	TotalChannels    int     `json:"total_channels" gorm:"default:0"`
	GroupRatio       float64 `json:"group_ratio" gorm:"type:decimal(10,4);default:1"`
	LastTestModel    string  `json:"last_test_model" gorm:"type:varchar(255);default:''"`
	UpdatedAt        int64   `json:"updated_at" gorm:"bigint"`
}

// MonitoringHistory — 时间线数据(供图表)
type MonitoringHistory struct {
	Id               int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	GroupName        string  `json:"group_name" gorm:"type:varchar(64);index:idx_mh_group_time"`
	AvailabilityRate float64 `json:"availability_rate" gorm:"type:decimal(8,4);default:-1"`
	CacheHitRate     float64 `json:"cache_hit_rate" gorm:"type:decimal(8,4);default:-1"`
	RecordedAt       int64   `json:"recorded_at" gorm:"bigint;index:idx_mh_group_time;index:idx_mh_time"`
}
