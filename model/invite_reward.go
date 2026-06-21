package model

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	InviteRewardStatusPending = "pending"
	InviteRewardStatusGranted = "granted" // legacy QuotaForInviter rows only — never written by new code
	InviteRewardStatusSettled = "settled" // new rebate rows that have been credited
	InviteRewardStatusRevoked = "revoked"

	InviteRewardTriggerTopUp            = "topup"
	InviteRewardTriggerSubscription     = "subscription"
	InviteRewardTriggerAdminManualTopUp = "admin_manual_topup"
	InviteRewardTriggerRedemption       = "redemption"
)

// InviteRewardRecord represents one rebate row.
//
// Schema notes:
//   - InviteeUserId is indexed but NOT unique: one invitee may have many rebate rows over time.
//   - IdempotencyKey is *string with a nullable unique index — NULL allows multiple rows,
//     a non-empty value is unique across the table and dedupes retries from the same trigger.
//   - RewardQuota is the rebate amount in quota units (already ratio-applied).
//   - SourceAmount is the base amount (quota units) the ratio was applied to, for audit.
//   - UnlockAt is the unix-second the row becomes settle-eligible; 0 on legacy pending rows.
type InviteRewardRecord struct {
	Id                   int     `json:"id"`
	InviteeUserId        int     `json:"invitee_user_id" gorm:"index"`
	InviterUserId        int     `json:"inviter_user_id" gorm:"index"`
	RewardQuota          int     `json:"reward_quota" gorm:"default:0"`
	SourceAmount         int     `json:"source_amount" gorm:"default:0"`
	RewardRatio          float64 `json:"reward_ratio" gorm:"default:0"`
	Status               string  `json:"status" gorm:"type:varchar(32);index"`
	TriggerType          string  `json:"trigger_type" gorm:"type:varchar(32);default:''"`
	TriggerTradeNo       string  `json:"trigger_trade_no" gorm:"type:varchar(255);default:''"`
	TriggerPaymentMethod string  `json:"trigger_payment_method" gorm:"type:varchar(50);default:''"`
	IdempotencyKey       *string `json:"idempotency_key" gorm:"type:varchar(191);uniqueIndex"`
	PaidAt               int64   `json:"paid_at" gorm:"bigint;default:0"`
	UnlockAt             int64   `json:"unlock_at" gorm:"bigint;default:0;index"`
	SettledAt            int64   `json:"settled_at" gorm:"bigint;default:0"`
	CreatedAt            int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt            int64   `json:"updated_at" gorm:"bigint"`
}

func (r *InviteRewardRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *InviteRewardRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

// IncreaseInviterCount bumps aff_count by 1 on signup with an inviter.
// Called from the user-creation paths instead of waiting until first paid event.
func IncreaseInviterCount(inviterUserId int) error {
	if inviterUserId <= 0 {
		return nil
	}
	return DB.Model(&User{}).Where("id = ?", inviterUserId).
		Update("aff_count", gorm.Expr("aff_count + ?", 1)).Error
}

// IssueInviteRebate issues a rebate row for one paid event.
//
// Behavior:
//   - No-op when invitee has no inviter, ratio<=0, or sourceAmount<=0.
//   - Computes rebateQuota = floor(sourceAmount * ratio). No-op when result <= 0.
//   - Idempotent via IdempotencyKey: triggerType + ":" + sha1(tradeNo). Empty tradeNo is rejected.
//   - When settleDays<=0 the rebate settles instantly (in the same tx) and credits the inviter now.
//   - When settleDays>0 the row is left pending until the settle ticker flips it.
//
// The caller MUST pass a tx that already covers the rest of the paid-event write (inviter balance,
// trade record, etc.). This avoids partial credit on crash and keeps refund-window logic atomic.
func IssueInviteRebate(tx *gorm.DB, inviteeUserId int, sourceAmount int, triggerType string, tradeNo string, paymentMethod string) error {
	if tx == nil {
		return errors.New("IssueInviteRebate: tx must not be nil")
	}
	if inviteeUserId <= 0 || sourceAmount <= 0 {
		return nil
	}
	ratio := common.InviteRewardRatio
	if ratio <= 0 {
		return nil
	}

	triggerType = strings.TrimSpace(triggerType)
	if triggerType == "" {
		triggerType = InviteRewardTriggerTopUp
	}
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		common.SysError(fmt.Sprintf("IssueInviteRebate: empty tradeNo (invitee=%d trigger=%s); skipped", inviteeUserId, triggerType))
		return nil
	}

	var invitee User
	if err := tx.Select("id", "inviter_id").
		Where("id = ?", inviteeUserId).First(&invitee).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if invitee.InviterId <= 0 {
		return nil
	}

	rebateQuota := int(math.Floor(float64(sourceAmount) * ratio))
	if rebateQuota <= 0 {
		return nil
	}

	now := common.GetTimestamp()
	settleDays := common.InviteRewardSettleDays
	if settleDays < 0 {
		settleDays = 0
	}
	unlockAt := now + int64(settleDays)*86400
	instantSettle := settleDays == 0

	key := buildInviteRebateIdempotencyKey(triggerType, tradeNo)
	status := InviteRewardStatusPending
	settledAt := int64(0)
	if instantSettle {
		status = InviteRewardStatusSettled
		settledAt = now
	}

	record := &InviteRewardRecord{
		InviteeUserId:        inviteeUserId,
		InviterUserId:        invitee.InviterId,
		RewardQuota:          rebateQuota,
		SourceAmount:         sourceAmount,
		RewardRatio:          ratio,
		Status:               status,
		TriggerType:          triggerType,
		TriggerTradeNo:       tradeNo,
		TriggerPaymentMethod: paymentMethod,
		IdempotencyKey:       &key,
		PaidAt:               now,
		UnlockAt:             unlockAt,
		SettledAt:            settledAt,
	}

	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(record)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// duplicate trigger — already handled, no further work.
		return nil
	}

	if !instantSettle {
		return nil
	}

	if err := tx.Model(&User{}).Where("id = ?", record.InviterUserId).Updates(map[string]interface{}{
		"aff_quota":   gorm.Expr("aff_quota + ?", record.RewardQuota),
		"aff_history": gorm.Expr("aff_history + ?", record.RewardQuota),
	}).Error; err != nil {
		return err
	}

	RecordLog(record.InviterUserId, LogTypeSystem,
		fmt.Sprintf("邀请用户充值返利 %s（订单号 %s）", logger.LogQuota(record.RewardQuota), tradeNo))
	return nil
}

// RevokePendingRebatesForInvitee revokes all pending rebates tied to the invitee.
// Settled rows are never touched. Called by admin "override balance to 0" (treated as refund).
//
// Locking: caller MUST pass a tx; we SELECT ... FOR UPDATE to serialize against the settle ticker.
func RevokePendingRebatesForInvitee(tx *gorm.DB, inviteeUserId int) (int, error) {
	if tx == nil {
		return 0, errors.New("RevokePendingRebatesForInvitee: tx must not be nil")
	}
	if inviteeUserId <= 0 {
		return 0, nil
	}

	var pending []InviteRewardRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("invitee_user_id = ? AND status = ? AND unlock_at > 0", inviteeUserId, InviteRewardStatusPending).
		Find(&pending).Error; err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil
	}

	now := common.GetTimestamp()
	revoked := 0
	for _, r := range pending {
		res := tx.Model(&InviteRewardRecord{}).
			Where("id = ? AND status = ? AND unlock_at > 0", r.Id, InviteRewardStatusPending).
			Updates(map[string]interface{}{
				"status":     InviteRewardStatusRevoked,
				"settled_at": now,
			})
		if res.Error != nil {
			return revoked, res.Error
		}
		if res.RowsAffected == 1 {
			revoked++
			RecordLog(r.InviterUserId, LogTypeSystem,
				fmt.Sprintf("被邀请用户已退款，撤销冷冻期内返利 %s", logger.LogQuota(r.RewardQuota)))
		}
	}
	return revoked, nil
}

// SettlePendingInviteRebates is called by the master-node ticker.
// It atomically flips one row at a time from pending->granted and credits the inviter
// in the SAME tx, so a crash never desyncs the ledger.
func SettlePendingInviteRebates(now int64) (int, error) {
	const batchSize = 100
	settled := 0
	for {
		var rows []InviteRewardRecord
		if err := DB.Where("status = ? AND unlock_at > 0 AND unlock_at <= ?",
			InviteRewardStatusPending, now).
			Order("id asc").Limit(batchSize).Find(&rows).Error; err != nil {
			return settled, err
		}
		if len(rows) == 0 {
			return settled, nil
		}
		for _, row := range rows {
			ok, err := settleOneInviteRebate(row.Id)
			if err != nil {
				return settled, err
			}
			if ok {
				settled++
			}
		}
		if len(rows) < batchSize {
			return settled, nil
		}
	}
}

// settleOneInviteRebate claims one pending row by id and credits the inviter atomically.
// Returns (true,nil) when this call did the work; (false,nil) when the row was already taken
// by a concurrent settler or revoke.
func settleOneInviteRebate(id int) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	var inviterId, rewardQuota int
	var tradeNo string
	settled := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var record InviteRewardRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, id).Error; err != nil {
			return err
		}
		if record.Status != InviteRewardStatusPending {
			return nil
		}
		now := common.GetTimestamp()
		if record.InviterUserId <= 0 || record.RewardQuota <= 0 {
			return tx.Model(&InviteRewardRecord{}).
				Where("id = ? AND status = ?", record.Id, InviteRewardStatusPending).
				Updates(map[string]interface{}{
					"status":     InviteRewardStatusRevoked,
					"settled_at": now,
				}).Error
		}
		if record.UnlockAt <= 0 || record.UnlockAt > now {
			return nil
		}
		res := tx.Model(&InviteRewardRecord{}).
			Where("id = ? AND status = ? AND unlock_at > 0 AND unlock_at <= ?", record.Id, InviteRewardStatusPending, now).
			Updates(map[string]interface{}{
				"status":     InviteRewardStatusSettled,
				"settled_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&User{}).Where("id = ?", record.InviterUserId).Updates(map[string]interface{}{
			"aff_quota":   gorm.Expr("aff_quota + ?", record.RewardQuota),
			"aff_history": gorm.Expr("aff_history + ?", record.RewardQuota),
		}).Error; err != nil {
			return err
		}
		inviterId = record.InviterUserId
		rewardQuota = record.RewardQuota
		tradeNo = record.TriggerTradeNo
		settled = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if settled {
		RecordLog(inviterId, LogTypeSystem,
			fmt.Sprintf("邀请返利冷冻期结束，已入账 %s（订单号 %s）", logger.LogQuota(rewardQuota), tradeNo))
	}
	return settled, nil
}

// GetInviteRewardSummaryByInviteeIds returns settled/pending rebate totals per invitee.
// Used by the admin "invited users" page to replace the legacy single-row LEFT JOIN.
type InviteRewardAggregate struct {
	InviteeUserId    int
	SettledQuota     int
	PendingQuota     int
	PendingCount     int
	EarliestUnlockAt int64
}

func GetInviteRewardSummaryByInviteeIds(inviteeIds []int) (map[int]InviteRewardAggregate, error) {
	out := make(map[int]InviteRewardAggregate, len(inviteeIds))
	if len(inviteeIds) == 0 {
		return out, nil
	}
	type row struct {
		InviteeUserId    int
		SettledQuota     int
		PendingQuota     int
		PendingCount     int
		EarliestUnlockAt int64
	}
	var rows []row
	if err := DB.Model(&InviteRewardRecord{}).
		Select(
			"invitee_user_id, "+
				"COALESCE(SUM(CASE WHEN status = ? OR status = ? THEN reward_quota ELSE 0 END), 0) AS settled_quota, "+
				"COALESCE(SUM(CASE WHEN status = ? AND unlock_at > 0 THEN reward_quota ELSE 0 END), 0) AS pending_quota, "+
				"COALESCE(SUM(CASE WHEN status = ? AND unlock_at > 0 THEN 1 ELSE 0 END), 0) AS pending_count, "+
				"COALESCE(MIN(CASE WHEN status = ? AND unlock_at > 0 THEN unlock_at ELSE NULL END), 0) AS earliest_unlock_at",
			InviteRewardStatusGranted, InviteRewardStatusSettled,
			InviteRewardStatusPending, InviteRewardStatusPending, InviteRewardStatusPending,
		).
		Where("invitee_user_id IN ?", inviteeIds).
		Group("invitee_user_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.InviteeUserId] = InviteRewardAggregate{
			InviteeUserId:    r.InviteeUserId,
			SettledQuota:     r.SettledQuota,
			PendingQuota:     r.PendingQuota,
			PendingCount:     r.PendingCount,
			EarliestUnlockAt: r.EarliestUnlockAt,
		}
	}
	return out, nil
}

func GetTotalPendingQuotaByInviterId(inviterId int) (int, error) {
	if inviterId <= 0 {
		return 0, nil
	}
	var total struct{ PendingQuota int }
	err := DB.Model(&InviteRewardRecord{}).
		Select("COALESCE(SUM(CASE WHEN status = ? AND unlock_at > 0 THEN reward_quota ELSE 0 END), 0) AS pending_quota",
			InviteRewardStatusPending).
		Where("inviter_user_id = ?", inviterId).
		Scan(&total).Error
	return total.PendingQuota, err
}

func buildInviteRebateIdempotencyKey(triggerType string, tradeNo string) string {
	raw := triggerType + ":" + tradeNo
	if len(raw) <= 191 {
		return raw
	}
	return triggerType + ":" + common.Sha1([]byte(tradeNo))
}
