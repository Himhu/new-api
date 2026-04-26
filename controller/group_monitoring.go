package controller

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// GetAdminMonitoringGroups returns all monitoring group stats for admin
func GetAdminMonitoringGroups(c *gin.Context) {
	setting := operation_setting.GetGroupMonitoringSetting()
	monitoringGroups := setting.MonitoringGroups
	if len(monitoringGroups) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    []interface{}{},
		})
		return
	}

	stats, err := model.GetGroupMonitoringStatsByNames(monitoringGroups)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取监控数据失败: " + err.Error(),
		})
		return
	}

	// Order by GroupDisplayOrder if set
	orderedStats := orderGroupStats(stats, setting.GroupDisplayOrder)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    orderedStats,
	})
}

// GetAdminMonitoringGroupDetail returns detailed monitoring info for a single group (admin)
func GetAdminMonitoringGroupDetail(c *gin.Context) {
	groupName := c.Param("group")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "分组名称不能为空",
		})
		return
	}

	groupStat, err := model.GetGroupMonitoringStatByName(groupName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "分组不存在或无监控数据",
		})
		return
	}

	channelStats, err := model.GetChannelMonitoringStatsByGroup(groupName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取渠道监控数据失败: " + err.Error(),
		})
		return
	}

	// Filter out deleted channels: only keep stats for currently active channels
	activeChannels, err := model.GetAllChannelsByGroup(groupName)
	if err == nil {
		activeSet := make(map[int]bool, len(activeChannels))
		channelNameMap := make(map[int]string, len(activeChannels))
		channelStatusMap := make(map[int]int, len(activeChannels))
		for _, ch := range activeChannels {
			activeSet[ch.Id] = true
			channelNameMap[ch.Id] = ch.Name
			channelStatusMap[ch.Id] = ch.Status
		}

		filtered := make([]model.ChannelMonitoringStat, 0, len(channelStats))
		for i := range channelStats {
			stat := channelStats[i]
			if activeSet[stat.ChannelId] {
				stat.ChannelName = channelNameMap[stat.ChannelId]
				stat.ChannelStatus = channelStatusMap[stat.ChannelId]
				filtered = append(filtered, stat)
			}
		}
		channelStats = filtered
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "",
		"data":          groupStat,
		"channel_stats": channelStats,
	})
}

// GetAdminMonitoringGroupHistory returns history data for charts (admin)
func GetAdminMonitoringGroupHistory(c *gin.Context) {
	groupName := c.Param("group")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "分组名称不能为空",
		})
		return
	}

	setting := operation_setting.GetGroupMonitoringSetting()
	intervalMinutes := setting.AggregationIntervalMinutes
	if intervalMinutes < 1 {
		intervalMinutes = 5
	}
	intervalSeconds := int64(intervalMinutes * 60)
	now := time.Now().Unix()
	currentBucketStart := now - (now % intervalSeconds)
	lastCompletedBucketStart := currentBucketStart - intervalSeconds
	startTime := currentBucketStart - int64(setting.AvailabilityPeriodMinutes*60)
	endTime := lastCompletedBucketStart

	history, err := model.GetMonitoringHistory(groupName, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取历史数据失败: " + err.Error(),
		})
		return
	}
	if snapshot, err := model.GetCurrentMonitoringSnapshot(groupName); err == nil && snapshot != nil {
		snapshot.RecordedAt = currentBucketStart
		history = append(history, *snapshot)
		endTime = currentBucketStart
	}

	c.JSON(http.StatusOK, gin.H{
		"success":                      true,
		"message":                      "",
		"data":                         history,
		"period_minutes":               setting.AvailabilityPeriodMinutes,
		"aggregation_interval_minutes": intervalMinutes,
		"window_start":                 startTime,
		"window_end":                   endTime,
		"summary_semantics":            "rolling_window",
		"history_semantics":            "per_bucket_with_current_snapshot",
	})
}

// RefreshMonitoringData triggers an immediate re-aggregation (admin)
func RefreshMonitoringData(c *gin.Context) {
	ok := service.TriggerAggregationRefresh()
	if !ok {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"message": "聚合正在运行中，请稍后再试",
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "刷新已触发，数据将在几秒后更新",
	})
}

// DeleteMonitoringGroupRecords clears all monitoring data for a group (admin)
func DeleteMonitoringGroupRecords(c *gin.Context) {
	groupName := c.Param("group")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "分组名称不能为空",
		})
		return
	}

	totalDeleted, err := model.DeleteAllMonitoringDataForGroup(groupName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "清空记录失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "清空成功",
		"data": gin.H{
			"deleted_rows": totalDeleted,
		},
	})
}

// GetPublicMonitoringGroups returns monitoring data for public/user view
func GetPublicMonitoringGroups(c *gin.Context) {
	setting := operation_setting.GetGroupMonitoringSetting()

	monitoringGroups := setting.MonitoringGroups
	if len(monitoringGroups) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    []interface{}{},
		})
		return
	}

	stats, err := model.GetGroupMonitoringStatsForPublic(monitoringGroups)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取监控数据失败",
		})
		return
	}

	// Desensitize: only return group-level info
	desensitized := make([]gin.H, 0, len(stats))
	for _, s := range stats {
		desensitized = append(desensitized, desensitizeGroupStat(&s))
	}

	orderedData := orderDesensitizedStats(desensitized, setting.GroupDisplayOrder)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    orderedData,
	})
}

// GetPublicMonitoringGroupHistory returns history data for charts (public/user)
func GetPublicMonitoringGroupHistory(c *gin.Context) {
	setting := operation_setting.GetGroupMonitoringSetting()

	groupName := c.Param("group")
	if groupName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "分组名称不能为空",
		})
		return
	}

	// Verify group is in the monitored list
	monitoringGroups := setting.MonitoringGroups
	found := false
	for _, g := range monitoringGroups {
		if g == groupName {
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "该分组不在监控列表中",
		})
		return
	}

	intervalMinutes := setting.AggregationIntervalMinutes
	if intervalMinutes < 1 {
		intervalMinutes = 5
	}
	intervalSeconds := int64(intervalMinutes * 60)
	now := time.Now().Unix()
	currentBucketStart := now - (now % intervalSeconds)
	lastCompletedBucketStart := currentBucketStart - intervalSeconds
	startTime := currentBucketStart - int64(setting.AvailabilityPeriodMinutes*60)
	endTime := lastCompletedBucketStart

	history, err := model.GetMonitoringHistory(groupName, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取历史数据失败",
		})
		return
	}
	if snapshot, err := model.GetCurrentMonitoringSnapshot(groupName); err == nil && snapshot != nil {
		snapshot.RecordedAt = currentBucketStart
		history = append(history, *snapshot)
		endTime = currentBucketStart
	}

	c.JSON(http.StatusOK, gin.H{
		"success":                      true,
		"message":                      "",
		"data":                         history,
		"period_minutes":               setting.AvailabilityPeriodMinutes,
		"aggregation_interval_minutes": intervalMinutes,
		"window_start":                 startTime,
		"window_end":                   endTime,
		"summary_semantics":            "rolling_window",
		"history_semantics":            "per_bucket_with_current_snapshot",
	})
}

// desensitizeGroupStat removes sensitive info from group stat for public view
func desensitizeGroupStat(stat *model.GroupMonitoringStat) gin.H {
	return gin.H{
		"group_name":        stat.GroupName,
		"availability_rate": stat.AvailabilityRate,
		"cache_hit_rate":    stat.CacheHitRate,
		"avg_response_time": stat.AvgResponseTime,
		"avg_frt":           stat.AvgFRT,
		"is_online":         stat.OnlineChannels > 0,
		"group_ratio":       stat.GroupRatio,
		"last_test_model":   stat.LastTestModel,
		"updated_at":        stat.UpdatedAt,
	}
}

// orderGroupStats orders stats by GroupDisplayOrder
func orderGroupStats(stats []model.GroupMonitoringStat, order []string) []model.GroupMonitoringStat {
	if len(order) == 0 {
		return stats
	}

	orderMap := make(map[string]int)
	for i, name := range order {
		orderMap[name] = i
	}

	ordered := make([]model.GroupMonitoringStat, 0, len(stats))
	remaining := make([]model.GroupMonitoringStat, 0)

	// First add ordered ones
	statMap := make(map[string]model.GroupMonitoringStat)
	for _, s := range stats {
		statMap[s.GroupName] = s
	}

	for _, name := range order {
		if s, ok := statMap[name]; ok {
			ordered = append(ordered, s)
			delete(statMap, name)
		}
	}

	// Then add remaining
	for _, s := range statMap {
		remaining = append(remaining, s)
	}

	return append(ordered, remaining...)
}

// orderDesensitizedStats orders desensitized stats by GroupDisplayOrder
func orderDesensitizedStats(stats []gin.H, order []string) []gin.H {
	if len(order) == 0 {
		return stats
	}

	orderMap := make(map[string]int)
	for i, name := range order {
		orderMap[name] = i
	}

	statMap := make(map[string]gin.H)
	for _, s := range stats {
		if name, ok := s["group_name"].(string); ok {
			statMap[name] = s
		}
	}

	ordered := make([]gin.H, 0, len(stats))
	for _, name := range order {
		if s, ok := statMap[name]; ok {
			ordered = append(ordered, s)
			delete(statMap, name)
		}
	}

	for _, s := range statMap {
		ordered = append(ordered, s)
	}

	return ordered
}
