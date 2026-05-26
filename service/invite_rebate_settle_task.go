package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const inviteRebateSettleTickInterval = 60 * time.Second

var (
	inviteRebateSettleOnce    sync.Once
	inviteRebateSettleRunning atomic.Bool
)

func StartInviteRebateSettleTask() {
	inviteRebateSettleOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("invite rebate settle task started: tick=%s", inviteRebateSettleTickInterval))
			ticker := time.NewTicker(inviteRebateSettleTickInterval)
			defer ticker.Stop()

			runInviteRebateSettleOnce()
			for range ticker.C {
				runInviteRebateSettleOnce()
			}
		})
	})
}

func runInviteRebateSettleOnce() {
	if !inviteRebateSettleRunning.CompareAndSwap(false, true) {
		return
	}
	defer inviteRebateSettleRunning.Store(false)

	now := common.GetTimestamp()
	settled, err := model.SettlePendingInviteRebates(now)
	if err != nil {
		common.SysError("invite rebate settle tick error: " + err.Error())
	}
	if settled > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf("invite rebate settle tick: settled %d rows", settled))
	}
}
