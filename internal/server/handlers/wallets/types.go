package wallets

import (
	"fmt"
	"github.com/ericlagergren/decimal"
)

// BalanceView represents balance shown out of api
type BalanceView decimal.Big

// MarshalJSON implements json marshaller
func (balance *BalanceView) MarshalJSON() ([]byte, error) {
	var res string
	d := (*decimal.Big)(balance)
	if balance == nil || d.Cmp(new(decimal.Big).SetFloat64(0)) == 0 {
		res = `"0.0"`
	} else {
		res = fmt.Sprintf(`"%f"`, decimal.WithContext(decimal.Context64).Set(d))
	}
	return []byte(res), nil
}
