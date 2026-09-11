package decimal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFitsNumeric36Scale18(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		fits  bool
	}{
		{name: "integer boundary", value: "999999999999999999", fits: true},
		{name: "integer overflow", value: "1000000000000000000", fits: false},
		{name: "scale boundary", value: "0.123456789012345678", fits: true},
		{name: "scale overflow", value: "0.1234567890123456789", fits: false},
		{name: "trailing fractional zeros are exact", value: "1.2300000000000000000", fits: true},
		{name: "scientific notation", value: "1e2", fits: true},
		{name: "smallest scale", value: "1e-18", fits: true},
		{name: "scale exponent overflow", value: "1e-19", fits: false},
		{name: "zero", value: "0", fits: true},
		{name: "negative", value: "-1.25", fits: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := NewFromString(tt.value)
			require.NoError(t, err)
			assert.Equal(t, tt.fits, d.FitsNumeric(36, 18))
		})
	}
}

func TestNewFromStringRejectsExcessiveExponent(t *testing.T) {
	t.Parallel()

	_, err := NewFromString("1e1001")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exponent exceeds")

	_, err = NewFromString("1e-1001")
	require.Error(t, err)
}

func TestNewFromStringRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", ".", "1.2.3", "1e2e3", "not-a-number", strings.Repeat("9", 128) + "x"} {
		_, err := NewFromString(value)
		require.Error(t, err, value)
	}
}

func FuzzNewFromString(f *testing.F) {
	for _, seed := range []string{"0", "1.25", "-3", "1e18", "1e-18", "bad"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 1024 {
			t.Skip()
		}
		d, err := NewFromString(value)
		if err == nil {
			_ = d.FitsNumeric(36, 18)
		}
	})
}
