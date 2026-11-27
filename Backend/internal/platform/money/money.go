// Package money parses and renders exact integer micro-dollar amounts.
package money

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Micros is an exact amount in millionths of a dollar.
type Micros int64

// Money accepts a JSON number or decimal string and always emits a JSON number.
type Money Micros

// FromDecimalString parses JSON decimal syntax, including exact exponent notation.
// Both the written fractional part and the expanded value have at most six places.
func FromDecimalString(s string) (Micros, error) {
	if s == "" || strings.TrimSpace(s) != s || !json.Valid([]byte(s)) {
		return 0, fmt.Errorf("invalid decimal amount")
	}
	negative := strings.HasPrefix(s, "-")
	unsigned := strings.TrimPrefix(s, "-")
	exponent := 0
	if at := strings.IndexAny(unsigned, "eE"); at >= 0 {
		var err error
		exponent, err = strconv.Atoi(unsigned[at+1:])
		if err != nil || exponent < -1000 || exponent > 1000 {
			return 0, fmt.Errorf("amount exponent out of range")
		}
		unsigned = unsigned[:at]
	}
	whole, fraction, _ := strings.Cut(unsigned, ".")
	if whole == "" || len(fraction) > 6 {
		return 0, fmt.Errorf("amount must have at most 6 decimal places")
	}
	for _, c := range whole + fraction {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid decimal amount")
		}
	}
	scale := 6 - len(fraction) + exponent
	if scale < 0 {
		return 0, fmt.Errorf("amount must have at most 6 decimal places")
	}
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return 0, nil
	}
	if len(digits)+scale > 19 {
		return 0, fmt.Errorf("amount overflows int64 micros")
	}
	digits += strings.Repeat("0", scale)
	if negative {
		digits = "-" + digits
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount overflows int64 micros: %w", err)
	}
	return Micros(n), nil
}

// FromJSON rejects null, nonnumeric values, and trailing JSON values.
func FromJSON(data []byte) (Micros, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return 0, fmt.Errorf("decode money: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return 0, fmt.Errorf("money must be one JSON value")
	}
	switch v := value.(type) {
	case string:
		return FromDecimalString(v)
	case json.Number:
		return FromDecimalString(v.String())
	default:
		return 0, fmt.Errorf("money must be a JSON number or string")
	}
}

// String renders up to six fractional places without floating point or overflow.
func (m Micros) String() string {
	digits := strconv.FormatInt(int64(m), 10)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	if len(digits) <= 6 {
		digits = strings.Repeat("0", 7-len(digits)) + digits
	}
	split := len(digits) - 6
	fraction := strings.TrimRight(digits[split:], "0")
	if fraction == "" {
		return sign + digits[:split]
	}
	return sign + digits[:split] + "." + fraction
}

// ImpressionCost rounds a nonnegative CPM up to the next whole micro-dollar.
func (m Micros) ImpressionCost() (Micros, error) {
	if m < 0 {
		return 0, fmt.Errorf("CPM must be nonnegative")
	}
	cost := m / 1000
	if m%1000 != 0 {
		cost++
	}
	return cost, nil
}

// MarshalJSON emits an exact JSON number.
func (m Money) MarshalJSON() ([]byte, error) { return []byte(Micros(m).String()), nil }

// UnmarshalJSON replaces the receiver only after successful validation.
func (m *Money) UnmarshalJSON(data []byte) error {
	value, err := FromJSON(data)
	if err != nil {
		return err
	}
	*m = Money(value)
	return nil
}
