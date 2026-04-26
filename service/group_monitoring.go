package service

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

var (
	groupMonitoringOnce sync.Once
	aggregationRunning  atomic.Int32
)

// StartGroupMonitoringAggregation starts the background aggregation loop
func StartGroupMonitoringAggregation() {
	if !common.IsMasterNode {
		return
	}
	groupMonitoringOnce.Do(func() {
		// Wait for DB to be ready
		time.Sleep(10 * time.Second)

		// Run initial aggregation
		runAggregationCycleSafe()

		for {
			setting := operation_setting.GetGroupMonitoringSetting()
			intervalMinutes := setting.AggregationIntervalMinutes
			if intervalMinutes < 1 {
				intervalMinutes = 5
			}

			if len(setting.MonitoringGroups) == 0 {
				// No groups configured, sleep and retry
				time.Sleep(1 * time.Minute)
				continue
			}

			interval := time.Duration(intervalMinutes) * time.Minute
			now := time.Now()
			nextRun := now.Truncate(interval).Add(interval).Add(2 * time.Second)
			time.Sleep(time.Until(nextRun))
			runAggregationCycleSafe()
		}
	})
}

// TriggerAggregationRefresh triggers an immediate refresh (clears all monitoring data and re-aggregates)
func TriggerAggregationRefresh() bool {
	if !aggregationRunning.CompareAndSwap(0, 1) {
		return false // already running
	}
	go func() {
		defer aggregationRunning.Store(0)

		// Clear all monitoring data before re-aggregation
		model.DeleteAllMonitoringData()

		runAggregationCycle(true)
	}()
	return true
}

func runAggregationCycleSafe() {
	if !aggregationRunning.CompareAndSwap(0, 1) {
		return // already running
	}
	defer aggregationRunning.Store(0)
	runAggregationCycle(false)
}

func runAggregationCycle(fullRefresh bool) {
	setting := operation_setting.GetGroupMonitoringSetting()
	monitoringGroups := setting.MonitoringGroups
	if len(monitoringGroups) == 0 {
		return
	}

	now := time.Now().Unix()
	intervalMinutes := setting.AggregationIntervalMinutes
	if intervalMinutes < 1 {
		intervalMinutes = 5
	}
	intervalSeconds := int64(intervalMinutes * 60)
	currentBucketStart := now - (now % intervalSeconds)
	lastCompletedBucketStart := currentBucketStart - intervalSeconds
	if lastCompletedBucketStart < 0 {
		lastCompletedBucketStart = 0
	}

	// Step 1: Determine time windows
	availPeriod := int64(setting.AvailabilityPeriodMinutes * 60)
	if availPeriod < 300 {
		availPeriod = 3600 // default 60 min
	}
	cachePeriod := int64(setting.CacheHitPeriodMinutes * 60)
	if cachePeriod < 300 {
		cachePeriod = 3600
	}

	// Use the larger period for full-refresh log query window
	queryPeriod := availPeriod
	if cachePeriod > queryPeriod {
		queryPeriod = cachePeriod
	}

	availWindowStart := currentBucketStart - availPeriod
	cacheWindowStart := currentBucketStart - cachePeriod
	historyWindowStart := availWindowStart
	if cacheWindowStart < historyWindowStart {
		historyWindowStart = cacheWindowStart
	}

	firstMissingBucketByGroup := make(map[string]int64, len(monitoringGroups))
	earliestMissingBucket := lastCompletedBucketStart
	if fullRefresh {
		for _, groupName := range monitoringGroups {
			firstMissingBucketByGroup[groupName] = historyWindowStart
		}
	} else {
		latestHistoryTimes, err := model.GetLatestMonitoringHistoryTimes(monitoringGroups)
		if err != nil {
			common.SysError("group monitoring: failed to get latest monitoring history times: " + err.Error())
			latestHistoryTimes = map[string]int64{}
		}
		for _, groupName := range monitoringGroups {
			firstMissingBucket := historyWindowStart
			if latestBucketStart, ok := latestHistoryTimes[groupName]; ok {
				firstMissingBucket = latestBucketStart + intervalSeconds
				if firstMissingBucket < historyWindowStart {
					firstMissingBucket = historyWindowStart
				}
			}
			firstMissingBucketByGroup[groupName] = firstMissingBucket
			if firstMissingBucket < earliestMissingBucket {
				earliestMissingBucket = firstMissingBucket
			}
		}
	}

	// Only aggregate fully closed buckets to avoid partial-bucket drift.
	var startTime int64
	if fullRefresh {
		startTime = currentBucketStart - queryPeriod
	} else {
		startTime = lastCompletedBucketStart
		if earliestMissingBucket < startTime {
			startTime = earliestMissingBucket
		}
	}
	endTime := currentBucketStart

	// Step 2: Query LOG_DB and aggregate logs
	rows, err := model.AggregateLogsForMonitoring(
		startTime,
		endTime,
		intervalSeconds,
	)
	if err != nil {
		common.SysError("group monitoring: failed to aggregate logs: " + err.Error())
		return
	}

	// Step 3: Convert to RequestStat and upsert
	stats := make([]model.RequestStat, len(rows))
	for i, row := range rows {
		stats[i] = model.RequestStat{
			BucketStart:       row.BucketStart,
			GroupName:         row.GroupName,
			ChannelId:         row.ChannelId,
			ModelName:         row.ModelName,
			TotalRequests:     row.TotalRequests,
			SuccessRequests:   row.SuccessRequests,
			ErrorRequests:     row.ErrorRequests,
			TotalCacheTokens:  row.TotalCacheTokens,
			TotalPromptTokens: row.TotalPromptTokens,
			CacheDataPoints:   row.CacheDataPoints,
			SumResponseTime:   row.SumResponseTime,
		}
	}
	if err := model.UpsertRequestStats(stats); err != nil {
		common.SysError("group monitoring: failed to upsert request stats: " + err.Error())
		return
	}

	// Step 4a: Aggregate availability stats directly from LOG_DB so availability-only
	// keyword exclusions do not affect request_stats/cache metrics.
	availAggs, err := model.AggregateAvailabilityFromLogsByGroupChannel(
		availWindowStart,
		currentBucketStart,
		setting.AvailabilityExcludeModels,
		setting.AvailabilityExcludeKeywords,
	)
	if err != nil {
		common.SysError("group monitoring: failed to aggregate availability stats: " + err.Error())
		return
	}

	// Step 4b: Aggregate cache hit stats (with cache excludes and period)
	cacheAggs, err := model.AggregateCacheHitByGroupChannel(cacheWindowStart, setting.CacheHitExcludeModels)
	if err != nil {
		common.SysError("group monitoring: failed to aggregate cache hit stats: " + err.Error())
		return
	}

	// Build maps: group+channel -> stats
	type channelKey struct {
		GroupName string
		ChannelId int
	}
	availMap := make(map[channelKey]*model.AvailabilityAggRow)
	for i := range availAggs {
		key := channelKey{availAggs[i].GroupName, availAggs[i].ChannelId}
		availMap[key] = &availAggs[i]
	}
	cacheMap := make(map[channelKey]*model.CacheHitAggRow)
	for i := range cacheAggs {
		key := channelKey{cacheAggs[i].GroupName, cacheAggs[i].ChannelId}
		cacheMap[key] = &cacheAggs[i]
	}

	// Step 4c: Per-interval aggregation for history chart
	// fullRefresh=true: aggregate all buckets in the availability/cache windows for backfill
	// fullRefresh=false: aggregate all still-missing closed buckets
	type bucketGroupKey struct {
		BucketStart int64
		GroupName   string
	}

	bucketAvailMap := make(map[bucketGroupKey]*model.GroupAvailabilityBucketRow)
	bucketCacheMap := make(map[bucketGroupKey]*model.GroupCacheHitBucketRow)

	if fullRefresh {
		// Aggregate all buckets across the entire availability period
		bucketAvailAggs, err := model.AggregateAvailabilityFromLogsByGroupBucket(
			availWindowStart,
			currentBucketStart,
			intervalSeconds,
			setting.AvailabilityExcludeModels,
			setting.AvailabilityExcludeKeywords,
		)
		if err != nil {
			common.SysError("group monitoring: failed to aggregate bucket availability: " + err.Error())
		} else {
			for i := range bucketAvailAggs {
				key := bucketGroupKey{bucketAvailAggs[i].BucketStart, bucketAvailAggs[i].GroupName}
				bucketAvailMap[key] = &bucketAvailAggs[i]
			}
		}
		bucketCacheAggs, err := model.AggregateCacheHitByGroupBucket(cacheWindowStart, setting.CacheHitExcludeModels)
		if err != nil {
			common.SysError("group monitoring: failed to aggregate bucket cache hit: " + err.Error())
		} else {
			for i := range bucketCacheAggs {
				key := bucketGroupKey{bucketCacheAggs[i].BucketStart, bucketCacheAggs[i].GroupName}
				bucketCacheMap[key] = &bucketCacheAggs[i]
			}
		}
	} else {
		// Normal cycle: aggregate all still-missing closed buckets.
		intervalStart := earliestMissingBucket
		if intervalStart < historyWindowStart {
			intervalStart = historyWindowStart
		}
		intervalEnd := currentBucketStart
		if intervalStart <= lastCompletedBucketStart {
			bucketAvailAggs, err := model.AggregateAvailabilityFromLogsByGroupBucket(
				intervalStart,
				intervalEnd,
				intervalSeconds,
				setting.AvailabilityExcludeModels,
				setting.AvailabilityExcludeKeywords,
			)
			if err != nil {
				common.SysError("group monitoring: failed to aggregate interval availability buckets: " + err.Error())
			} else {
				for i := range bucketAvailAggs {
					key := bucketGroupKey{bucketAvailAggs[i].BucketStart, bucketAvailAggs[i].GroupName}
					bucketAvailMap[key] = &bucketAvailAggs[i]
				}
			}
			bucketCacheAggs, err := model.AggregateCacheHitByGroupBucket(intervalStart, setting.CacheHitExcludeModels)
			if err != nil {
				common.SysError("group monitoring: failed to aggregate interval cache hit buckets: " + err.Error())
			} else {
				for i := range bucketCacheAggs {
					if bucketCacheAggs[i].BucketStart < intervalStart || bucketCacheAggs[i].BucketStart > lastCompletedBucketStart {
						continue
					}
					key := bucketGroupKey{bucketCacheAggs[i].BucketStart, bucketCacheAggs[i].GroupName}
					bucketCacheMap[key] = &bucketCacheAggs[i]
				}
			}
		}
	}

	// Step 5: Get channel test info
	channelTestInfos, err := model.GetChannelTestInfoByGroups(monitoringGroups)
	if err != nil {
		common.SysError("group monitoring: failed to get channel test info: " + err.Error())
		return
	}

	// Build channel test info map
	// A channel's Group field may be "group1,group2,group3", so map each individual group to the info
	testInfoMap := make(map[channelKey]*model.ChannelTestInfo)
	for i := range channelTestInfos {
		info := &channelTestInfos[i]
		for _, g := range strings.Split(info.Group, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				testInfoMap[channelKey{g, info.ChannelId}] = info
			}
		}
	}

	// Step 6: Upsert channel_monitoring_stats and calculate group stats
	var historyRecords []model.MonitoringHistory

	// Build set for cache-tokens-separate groups (Claude-style: prompt_tokens excludes cache)
	cacheSeparateSet := make(map[string]bool)
	for _, g := range setting.CacheTokensSeparateGroups {
		cacheSeparateSet[g] = true
	}

	for _, groupName := range monitoringGroups {
		var groupTotalRequests int
		var groupSuccessRequests int
		var groupTotalCacheTokens int64
		var groupTotalPromptTokens int64
		var groupCacheDataPoints int
		var groupSumResponseTime int64
		var onlineChannels int
		var totalChannels int
		var lastTestModel string
		isCacheSeparate := cacheSeparateSet[groupName]

		// Get all enabled channels in this group
		channels, err := model.GetChannelsByGroup(groupName)
		if err != nil {
			common.SysError("group monitoring: failed to get channels for group " + groupName + ": " + err.Error())
			continue
		}
		totalChannels = len(channels)

		// Get latest FRT for each channel in this group
		channelIds := make([]int, 0, len(channels))
		for _, ch := range channels {
			channelIds = append(channelIds, ch.Id)
		}
		frtMap, frtErr := model.GetLatestFRTForChannels(availWindowStart, groupName, channelIds)
		if frtErr != nil {
			common.SysError("group monitoring: failed to get FRT for group " + groupName + ": " + frtErr.Error())
			frtMap = nil
		}

		// Determine online/offline based on latest log status (not channel test)
		// Only use AvailabilityExcludeModels — cache excludes are unrelated to online status
		onlineMap, onlineErr := model.GetLatestLogStatusForChannels(availWindowStart, groupName, channelIds, setting.AvailabilityExcludeModels, setting.AvailabilityExcludeKeywords)
		if onlineErr != nil {
			common.SysError("group monitoring: failed to get online status for group " + groupName + ": " + onlineErr.Error())
			onlineMap = nil
		}

		// Get latest model_name for channels without TestModel configured
		modelMap, modelErr := model.GetLatestModelForChannels(groupName, channelIds)
		if modelErr != nil {
			common.SysError("group monitoring: failed to get latest model for group " + groupName + ": " + modelErr.Error())
			modelMap = nil
		}

		for _, ch := range channels {
			key := channelKey{groupName, ch.Id}
			availAgg := availMap[key]
			cacheAgg := cacheMap[key]
			testInfo := testInfoMap[key]

			var availRate float64 = -1
			var cacheHitRate float64 = -1
			var responseTime int
			var testTime int64
			testModel := ""
			isOnline := false

			if availAgg != nil {
				totalReq := availAgg.TotalRequests
				if totalReq > 0 {
					availRate = float64(availAgg.SuccessRequests) / float64(totalReq) * 100
				}
				groupTotalRequests += availAgg.TotalRequests
				groupSuccessRequests += availAgg.SuccessRequests
				groupSumResponseTime += availAgg.SumResponseTime
			}

			if cacheAgg != nil {
				if cacheAgg.CacheDataPoints > 0 {
					var totalTokens int64
					if isCacheSeparate {
						// Claude-style: prompt_tokens does NOT include cache_tokens
						totalTokens = cacheAgg.TotalPromptTokens + cacheAgg.TotalCacheTokens
					} else {
						// OpenAI-style: prompt_tokens already includes cache_tokens
						totalTokens = cacheAgg.TotalPromptTokens
					}
					if totalTokens > 0 {
						cacheHitRate = float64(cacheAgg.TotalCacheTokens) / float64(totalTokens) * 100
						if cacheHitRate > 100 {
							cacheHitRate = 100
						}
					}
				}
				groupTotalCacheTokens += cacheAgg.TotalCacheTokens
				groupTotalPromptTokens += cacheAgg.TotalPromptTokens
				groupCacheDataPoints += cacheAgg.CacheDataPoints
			}

			if testInfo != nil {
				responseTime = testInfo.ResponseTime
				testTime = testInfo.TestTime
				testModel = testInfo.TestModel
			}

			// Fallback: if TestModel is not configured, use latest model from logs
			if testModel == "" && modelMap != nil {
				if m, ok := modelMap[ch.Id]; ok {
					testModel = m
				}
			}

			// Determine online status from latest log entry
			if onlineMap != nil {
				isOnline = onlineMap[ch.Id]
			}

			if isOnline {
				onlineChannels++
			}
			if testModel != "" {
				lastTestModel = testModel
			}

			// Upsert channel monitoring stat
			lastFRT := 0
			if frtMap != nil {
				if v, ok := frtMap[ch.Id]; ok {
					lastFRT = v
				}
			}
			stat := &model.ChannelMonitoringStat{
				GroupName:        groupName,
				ChannelId:        ch.Id,
				AvailabilityRate: availRate,
				CacheHitRate:     cacheHitRate,
				LastResponseTime: responseTime,
				LastFRT:          lastFRT,
				LastTestTime:     testTime,
				LastTestModel:    testModel,
				IsOnline:         isOnline,
				UpdatedAt:        now,
			}
			if err := model.UpsertChannelMonitoringStat(stat); err != nil {
				common.SysError("group monitoring: failed to upsert channel stat: " + err.Error())
			}
		}

		// Clean up orphan channel monitoring stats (deleted channels)
		if len(channels) > 0 {
			activeChannelIds := make([]int, 0, len(channels))
			for _, ch := range channels {
				activeChannelIds = append(activeChannelIds, ch.Id)
			}
			model.DeleteOrphanChannelMonitoringStats(groupName, activeChannelIds)
		} else {
			// No active channels — delete all channel stats for this group
			model.DeleteChannelMonitoringStatsByGroup(groupName)
		}

		// Calculate group-level stats
		var groupAvailRate float64 = -1
		var groupCacheHitRate float64 = -1
		var avgResponseTime int

		if groupTotalRequests > 0 {
			groupAvailRate = float64(groupSuccessRequests) / float64(groupTotalRequests) * 100
		}
		if groupCacheDataPoints > 0 {
			var totalTokens int64
			if isCacheSeparate {
				totalTokens = groupTotalPromptTokens + groupTotalCacheTokens
			} else {
				totalTokens = groupTotalPromptTokens
			}
			if totalTokens > 0 {
				groupCacheHitRate = float64(groupTotalCacheTokens) / float64(totalTokens) * 100
				if groupCacheHitRate > 100 {
					groupCacheHitRate = 100
				}
			}
		}
		if groupTotalRequests > 0 {
			avgResponseTime = int(groupSumResponseTime / int64(groupTotalRequests))
		}

		// Calculate group-level average FRT from channel FRTs
		var avgFRT int
		if frtMap != nil && len(frtMap) > 0 {
			var frtSum, frtCount int
			for _, frt := range frtMap {
				frtSum += frt
				frtCount++
			}
			if frtCount > 0 {
				avgFRT = frtSum / frtCount
			}
		}

		groupRatio := ratio_setting.GetGroupRatio(groupName)

		groupStat := &model.GroupMonitoringStat{
			GroupName:        groupName,
			AvailabilityRate: groupAvailRate,
			CacheHitRate:     groupCacheHitRate,
			AvgResponseTime:  avgResponseTime,
			AvgFRT:           avgFRT,
			OnlineChannels:   onlineChannels,
			TotalChannels:    totalChannels,
			GroupRatio:       groupRatio,
			LastTestModel:    lastTestModel,
			UpdatedAt:        now,
		}
		if err := model.UpsertGroupMonitoringStat(groupStat); err != nil {
			common.SysError("group monitoring: failed to upsert group stat: " + err.Error())
		}

		// Step 7: Insert monitoring history
		if fullRefresh {
			for bucketStart := historyWindowStart; bucketStart <= lastCompletedBucketStart; bucketStart += intervalSeconds {
				var bAvailRate float64 = -1
				var bCacheRate float64 = -1

				bKey := bucketGroupKey{bucketStart, groupName}
				if ba := bucketAvailMap[bKey]; ba != nil && ba.TotalRequests > 0 {
					bAvailRate = float64(ba.SuccessRequests) / float64(ba.TotalRequests) * 100
				}
				if bc := bucketCacheMap[bKey]; bc != nil && bc.CacheDataPoints > 0 {
					var totalTokens int64
					if isCacheSeparate {
						totalTokens = bc.TotalPromptTokens + bc.TotalCacheTokens
					} else {
						totalTokens = bc.TotalPromptTokens
					}
					if totalTokens > 0 {
						bCacheRate = float64(bc.TotalCacheTokens) / float64(totalTokens) * 100
						if bCacheRate > 100 {
							bCacheRate = 100
						}
					}
				}

				historyRecords = append(historyRecords, model.MonitoringHistory{
					GroupName:        groupName,
					AvailabilityRate: bAvailRate,
					CacheHitRate:     bCacheRate,
					RecordedAt:       bucketStart,
				})
			}
		} else {
			firstMissingBucket := firstMissingBucketByGroup[groupName]
			if firstMissingBucket > lastCompletedBucketStart {
				continue
			}
			for bucketStart := firstMissingBucket; bucketStart <= lastCompletedBucketStart; bucketStart += intervalSeconds {
				var bucketAvailRate float64 = -1
				var bucketCacheRate float64 = -1

				bKey := bucketGroupKey{bucketStart, groupName}
				if ba := bucketAvailMap[bKey]; ba != nil && ba.TotalRequests > 0 {
					bucketAvailRate = float64(ba.SuccessRequests) / float64(ba.TotalRequests) * 100
				}
				if bc := bucketCacheMap[bKey]; bc != nil && bc.CacheDataPoints > 0 {
					var totalTokens int64
					if isCacheSeparate {
						totalTokens = bc.TotalPromptTokens + bc.TotalCacheTokens
					} else {
						totalTokens = bc.TotalPromptTokens
					}
					if totalTokens > 0 {
						bucketCacheRate = float64(bc.TotalCacheTokens) / float64(totalTokens) * 100
						if bucketCacheRate > 100 {
							bucketCacheRate = 100
						}
					}
				}

				historyRecords = append(historyRecords, model.MonitoringHistory{
					GroupName:        groupName,
					AvailabilityRate: bucketAvailRate,
					CacheHitRate:     bucketCacheRate,
					RecordedAt:       bucketStart,
				})
			}
		}
	}

	if err := model.BatchInsertMonitoringHistory(historyRecords); err != nil {
		common.SysError("group monitoring: failed to insert history: " + err.Error())
	}

	// Step 8: Cleanup old data
	cleanupTime := now - 7*24*3600
	if _, err := model.CleanupOldRequestStats(cleanupTime); err != nil {
		common.SysError("group monitoring: failed to cleanup old request stats: " + err.Error())
	}
	historyCleanupTime := now - 30*24*3600
	if _, err := model.CleanupOldMonitoringHistory(historyCleanupTime); err != nil {
		common.SysError("group monitoring: failed to cleanup old monitoring history: " + err.Error())
	}

	// Cleanup stale groups
	model.CleanupStaleMonitoringStats(monitoringGroups)
}
