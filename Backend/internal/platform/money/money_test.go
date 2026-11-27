package money_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"pulsebid/internal/platform/money"
)

func TestDecimalRoundTrip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input  string
		want   money.Micros
		output string
	}{
		{"0", 0, "0"}, {"-0", 0, "0"}, {"1", 1000000, "1"}, {"1.51", 1510000, "1.51"},
		{"0.00151", 1510, "0.00151"}, {"0.000001", 1, "0.000001"}, {"-0.000001", -1, "-0.000001"},
		{"50.00", 50000000, "50"}, {"1.000000", 1000000, "1"}, {"-1.250000", -1250000, "-1.25"},
		{"1e-6", 1, "0.000001"}, {"1.25e2", 125000000, "125"}, {"1E+1", 10000000, "10"},
		{"0e20", 0, "0"}, {"9223372036854.775807", money.Micros(math.MaxInt64), "9223372036854.775807"},
		{"-9223372036854.775808", money.Micros(math.MinInt64), "-9223372036854.775808"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			actual, err := money.FromDecimalString(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.want, actual)
			require.Equal(t, tc.output, actual.String())
			for _, raw := range []string{tc.input, `"` + tc.input + `"`} {
				var decoded money.Money
				require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
				require.Equal(t, money.Money(tc.want), decoded)
				encoded, err := json.Marshal(decoded)
				require.NoError(t, err)
				require.Equal(t, tc.output, string(encoded))
			}
		})
	}
}

func TestInvalidDecimal(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"", " ", " 1", "1 ", "+1", "01", ".1", "1.", "--1", "1,000", "NaN", "Infinity", "1.0000000", "0.0000001", "1e-7", "1.123456e-1", "9223372036854.775808", "-9223372036854.775809", "9223372036855", "1e100", "1e1001", "1e-1001", "1e999999999999999999999", "true", "null", "[]", "{}", `"1"`, "1 2"} {
		t.Run(input, func(t *testing.T) { t.Parallel(); _, err := money.FromDecimalString(input); require.Error(t, err) })
	}
}

func TestInvalidJSONDoesNotChangeReceiver(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"", `null`, `true`, `false`, `[]`, `{}`, `"bad"`, `"1.1234567"`, `1 2`, `1 garbage`, `" 1"`} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			value := money.Money(123)
			require.Error(t, value.UnmarshalJSON([]byte(input)))
			require.Equal(t, money.Money(123), value)
		})
	}
}

func TestImpressionCost(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ cpm, cost money.Micros }{{0, 0}, {1, 1}, {999, 1}, {1000, 1}, {1001, 2}, {1010000, 1010}, {1510000, 1510}, {money.Micros(math.MaxInt64), 9223372036854776}} {
		cost, err := tc.cpm.ImpressionCost()
		require.NoError(t, err)
		require.Equal(t, tc.cost, cost)
	}
	_, err := money.Micros(-1).ImpressionCost()
	require.Error(t, err)
}

func FuzzJSONRoundTrip(f *testing.F) {
	for _, seed := range []string{`1.51`, `"0.000001"`, `-9223372036854.775808`, `1e-7`, `null`, `1 2`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		value, err := money.FromJSON([]byte(input))
		if err != nil {
			return
		}
		again, err := money.FromDecimalString(value.String())
		require.NoError(t, err)
		require.Equal(t, value, again)
	})
}
