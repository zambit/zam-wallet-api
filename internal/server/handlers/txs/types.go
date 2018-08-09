package txs

import (
	"fmt"
	"github.com/ericlagergren/decimal"
)

// DecimalView represents decimal shown out of api
type DecimalView decimal.Big

// UnmarshalJSON implements json unmarshaler
func (view *DecimalView) UnmarshalJSON(data []byte) error {
	if data[0] == '"' && data[len(data)-1] == '"' {
		data = data[1 : len(data)-1]
	}
	return (*decimal.Big)(view).UnmarshalText(data)
}

// MarshalJSON implements json marshaller
func (view *DecimalView) MarshalJSON() ([]byte, error) {
	var res string
	d := (*decimal.Big)(view)
	if view == nil || d.Cmp(new(decimal.Big).SetFloat64(0)) == 0 {
		res = `"0.0"`
	} else {
		res = fmt.Sprintf(`"%f"`, decimal.WithContext(decimal.Context64).Set(d))
	}
	return []byte(res), nil
}
