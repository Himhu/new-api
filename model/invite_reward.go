package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	InviteRewardStatusPending           = "pending"
	InviteRewardStatusGranted           = "granted"
	InviteRewardTriggerTopUp            = "topup"
	InviteRewardTriggerSubscription     = "subscription"
	InviteRewardTriggerAdminManualTopUp = "admin_manual_topup"
)

type InviteRewardRecord struct {
	Id                   int    `json:"id"`
	InviteeUserId        int    `json:"invitee_user_id" gorm:"uniqueIndex"`
	InviterUserId        int    `json:"inviter_user_id" gorm:"index"`
	RewardQuota          int    `json:"reward_quota" gorm:"default:0"`
	Status               string `json:"status" gorm:"type:varchar(32);index"`
	TriggerType          string `json:"trigger_type" gorm:"type:varchar(32);default:''"`
	TriggerTradeNo       string `json:"trigger_trade_no" gorm:"type:varchar(255);default:''"`
	TriggerPaymentMethod string `json:"trigger_payment_method" gorm:"type:varchar(50);default:''"`
	PaidAt               int64  `json:"paid_at" gorm:"bigint;default:0"`
	CreatedAt            int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt            int64  `json:"updated_at" gorm:"bigint"`
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

func ensurePendingInviteRewardRecord(inviteeUserId int, inviterUserId int) error {
	if inviteeUserId <= 0 || inviterUserId <= 0 || common.QuotaForInviter <= 0 {
		return nil
	}
	record := &InviteRewardRecord{
		InviteeUserId: inviteeUserId,
		InviterUserId: inviterUserId,
		RewardQuota:   common.QuotaForInviter,
		Status:        InviteRewardStatusPending,
	}
	return DB.Clauses(clause.OnConflict{DoNothing: true}).Create(record).Error
}

func rewardInviter(inviterId int, rewardQuota int) error {
	if inviterId <= 0 || rewardQuota <= 0 {
		return nil
	}
	return DB.Model(&User{}).Where("id = ?", inviterId).Updates(map[string]interface{}{
		"aff_count":   gorm.Expr("aff_count + ?", 1),
		"aff_quota":   gorm.Expr("aff_quota + ?", rewardQuota),
		"aff_history": gorm.Expr("aff_history + ?", rewardQuota),
	}).Error
}

func GrantInviterRewardOnFirstPaidEvent(inviteeUserId int, triggerType string, triggerTradeNo string, paymentMethod string) error {
	if inviteeUserId <= 0 || common.QuotaForInviter <= 0 {
		return nil
	}
	triggerType = strings.TrimSpace(triggerType)
	if triggerType == "" {
		triggerType = InviteRewardTriggerTopUp
	}
	paidAt := common.GetTimestamp()
	var inviterId int
	var rewardQuota int
	var granted bool

	err := DB.Transaction(func(tx *gorm.DB) error {
		var record InviteRewardRecord
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("invitee_user_id = ?", inviteeUserId).First(&record).Error; err != nil {
			return err
		}
		if record.Status == InviteRewardStatusGranted {
			return nil
		}
		if record.Status != InviteRewardStatusPending {
			return nil
		}
		if record.InviterUserId <= 0 || record.RewardQuota <= 0 {
			return nil
		}
		if err := tx.Model(&User{}).Where("id = ?", record.InviterUserId).Updates(map[string]interface{}{
			"aff_count":   gorm.Expr("aff_count + ?", 1),
			"aff_quota":   gorm.Expr("aff_quota + ?", record.RewardQuota),
			"aff_history": gorm.Expr("aff_history + ?", record.RewardQuota),
		}).Error; err != nil {
			return err
		}
		record.Status = InviteRewardStatusGranted
		record.TriggerType = triggerType
		record.TriggerTradeNo = triggerTradeNo
		record.TriggerPaymentMethod = paymentMethod
		record.PaidAt = paidAt
		if err := tx.Save(&record).Error; err != nil {
			return err
		}
		inviterId = record.InviterUserId
		rewardQuota = record.RewardQuota
		granted = true
		return nil
	})
	if err != nil {
		return err
	}
	if granted {
		RecordLog(inviterId, LogTypeSystem, fmt.Sprintf("邀请用户首充后赠送 %s", logger.LogQuota(rewardQuota)))
	}
	return nil
}
