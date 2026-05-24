package operation_setting

import (
	"errors"

	"github.com/QuantumNous/new-api/setting/config"
)

// DefaultUserGroup 是系统内置的默认用户分组，享受渠道原始最低充值额。
const DefaultUserGroup = "default"

type PaymentSetting struct {
	AmountOptions  []int           `json:"amount_options"`
	AmountDiscount map[int]float64 `json:"amount_discount"` // 充值金额对应的折扣，例如 100 元 0.9 表示 100 元充值享受 9 折优惠
	// GroupMinTopUp 为各用户分组配置的最低充值额度（同各渠道原始单位）。
	// 仅 default 分组可缺省（沿用渠道原始最低充值）；其余分组必须显式配置，否则禁止充值。
	GroupMinTopUp map[string]int `json:"group_min_topup"`
}

// 默认配置
var paymentSetting = PaymentSetting{
	AmountOptions:  []int{10, 20, 50, 100, 200, 500},
	AmountDiscount: map[int]float64{},
	GroupMinTopUp:  map[string]int{},
}

// ErrGroupTopUpNotConfigured 表示当前用户分组未配置最低充值额度，禁止充值。
var ErrGroupTopUpNotConfigured = errors.New("当前分组未配置最低充值金额，请联系管理员")

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("payment_setting", &paymentSetting)
}

func GetPaymentSetting() *PaymentSetting {
	return &paymentSetting
}

// ResolveMinTopUpForGroup 计算指定分组在给定渠道最低额度（原始单位）下的有效最低充值额。
//
// 规则：
//   - default 分组：直接返回渠道原始 channelMin。
//   - 非 default 分组：必须在 GroupMinTopUp 中显式配置，否则返回
//     ErrGroupTopUpNotConfigured；配置存在时返回 max(channelMin, groupMin)。
//
// GroupMinTopUp 由管理员显式维护，是充值策略的唯一权威来源——无需与
// UserUsableGroups / GroupGroupRatio 等其它分组注册表交叉校验，避免对管理员
// 指派的特殊分组（不在自助可选列表中）产生误拒。
func ResolveMinTopUpForGroup(group string, channelMin int) (int, error) {
	if group == DefaultUserGroup {
		return channelMin, nil
	}
	groupMin, ok := paymentSetting.GroupMinTopUp[group]
	if !ok {
		return 0, ErrGroupTopUpNotConfigured
	}
	if groupMin > channelMin {
		return groupMin, nil
	}
	return channelMin, nil
}
