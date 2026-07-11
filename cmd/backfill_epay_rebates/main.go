package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var epayPaymentMethods = []string{"alipay", "wxpay", "qqpay", "tenpay"}

func main() {
	dryRun := flag.Bool("dry-run", false, "report rows that would be inserted without writing")
	limit := flag.Int("limit", 0, "process at most N missing rebate rows; 0 means no limit")
	flag.Parse()

	os.Setenv("NODE_TYPE", "slave")

	_ = godotenv.Load(".env")
	common.InitEnv()
	logger.SetupLogger()
	ratio_setting.InitRatioSettings()
	if err := model.InitDB(); err != nil {
		log.Fatalf("init db: %v", err)
	}
	model.InitOptionMap()

	if common.InviteRewardRatio <= 0 {
		log.Fatalf("InviteRewardRatio=%f, refuse to backfill; enable via admin UI first", common.InviteRewardRatio)
	}
	fmt.Printf("[cfg] InviteRewardRatio=%.4f InviteRewardSettleDays=%d dry_run=%t limit=%d\n",
		common.InviteRewardRatio, common.InviteRewardSettleDays, *dryRun, *limit)

	qFile, err := os.Create("quarantine.csv")
	if err != nil {
		log.Fatalf("create quarantine.csv: %v", err)
	}
	defer qFile.Close()
	qw := csv.NewWriter(qFile)
	defer qw.Flush()
	_ = qw.Write([]string{"trade_no", "user_id", "amount", "payment_method", "create_time", "complete_time", "reason"})

	var topUps []model.TopUp
	if err := model.DB.Where("status = ? AND payment_method IN ?", common.TopUpStatusSuccess, epayPaymentMethods).
		Order("id asc").Find(&topUps).Error; err != nil {
		log.Fatalf("load topups: %v", err)
	}
	fmt.Printf("[load] loaded %d successful epay topups\n", len(topUps))

	processed, inserted, quarantined, skippedExisting := 0, 0, 0, 0
	for _, topUp := range topUps {
		key := buildIdempotencyKey(model.InviteRewardTriggerTopUp, topUp.TradeNo)
		exists, err := rebateExists(model.DB, key)
		if err != nil {
			log.Fatalf("check rebate trade_no=%s: %v", topUp.TradeNo, err)
		}
		if exists {
			skippedExisting++
			continue
		}
		if *limit > 0 && processed >= *limit {
			break
		}
		processed++

		if topUp.CompleteTime == 0 && topUp.CreateTime == 0 {
			quarantined++
			writeQuarantine(qw, topUp, "complete_time and create_time both zero")
			continue
		}
		if topUp.Amount <= 0 {
			quarantined++
			writeQuarantine(qw, topUp, fmt.Sprintf("invalid amount=%d", topUp.Amount))
			continue
		}

		sourceQuota := int(decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
		if sourceQuota <= 0 {
			quarantined++
			writeQuarantine(qw, topUp, fmt.Sprintf("computed source_quota<=0 amount=%d", topUp.Amount))
			continue
		}

		if *dryRun {
			fmt.Printf("[dry-run] would insert trade_no=%s user_id=%d source=%d\n", topUp.TradeNo, topUp.UserId, sourceQuota)
			inserted++
		} else {
			didInsert, err := issueBackfillRebate(topUp, sourceQuota, key)
			if err != nil {
				log.Fatalf("issue rebate trade_no=%s: %v", topUp.TradeNo, err)
			}
			if didInsert {
				inserted++
			}
		}
		if processed%100 == 0 {
			fmt.Printf("[progress] processed_missing=%d inserted=%d quarantined=%d skipped_existing=%d\n",
				processed, inserted, quarantined, skippedExisting)
		}
	}

	fmt.Printf("[done] dry_run=%t total_epay_topups=%d skipped_existing=%d processed_missing=%d inserted=%d quarantined=%d\n",
		*dryRun, len(topUps), skippedExisting, processed, inserted, quarantined)
}

func issueBackfillRebate(topUp model.TopUp, sourceQuota int, key string) (bool, error) {
	inserted := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		exists, err := rebateExists(tx, key)
		if err != nil || exists {
			return err
		}
		if err := model.IssueInviteRebate(tx, topUp.UserId, sourceQuota, model.InviteRewardTriggerTopUp, topUp.TradeNo, topUp.PaymentMethod); err != nil {
			return err
		}
		inserted, err = rebateExists(tx, key)
		return err
	})
	return inserted, err
}

func rebateExists(tx *gorm.DB, key string) (bool, error) {
	var count int64
	if err := tx.Model(&model.InviteRewardRecord{}).Where("idempotency_key = ?", key).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func buildIdempotencyKey(triggerType, tradeNo string) string {
	raw := triggerType + ":" + tradeNo
	if len(raw) <= 191 {
		return raw
	}
	return triggerType + ":" + common.Sha1([]byte(tradeNo))
}

func writeQuarantine(w *csv.Writer, topUp model.TopUp, reason string) {
	_ = w.Write([]string{
		topUp.TradeNo,
		fmt.Sprintf("%d", topUp.UserId),
		fmt.Sprintf("%d", topUp.Amount),
		topUp.PaymentMethod,
		fmt.Sprintf("%d", topUp.CreateTime),
		fmt.Sprintf("%d", topUp.CompleteTime),
		reason,
	})
}
