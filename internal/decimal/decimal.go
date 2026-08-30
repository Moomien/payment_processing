package decimal

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

const MaxAbsExponent = 1000

var zeroInt = big.NewInt(0)
var tenInt = big.NewInt(10)

type Decimal struct {
	value *big.Int
	exp   int32
}

func NewFromString(value string) (Decimal, error) {
	originalInput := value
	var intString string
	var exp int64

	// Check if number is using scientific notation and find dots
	eIndex := -1
	pIndex := -1
	for i, r := range value {
		if r == 'E' || r == 'e' {
			if eIndex > -1 {
				return Decimal{}, fmt.Errorf("can't convert %s to decimal: multiple 'E' characters found", value)
			}
			eIndex = i
			continue
		}

		if r == '.' {
			if pIndex > -1 {
				return Decimal{}, fmt.Errorf("can't convert %s to decimal: too many .s", value)
			}
			pIndex = i
		}
	}

	if eIndex != -1 {
		expInt, err := strconv.ParseInt(value[eIndex+1:], 10, 32)
		if err != nil {
			if e, ok := err.(*strconv.NumError); ok && e.Err == strconv.ErrRange {
				return Decimal{}, fmt.Errorf("can't convert %s to decimal: fractional part too long", value)
			}
			return Decimal{}, fmt.Errorf("can't convert %s to decimal: exponent is not numeric", value)
		}
		value = value[:eIndex]
		exp = expInt
	}

	if pIndex == -1 {
		// There is no decimal point, we can just parse the original string as
		// an int
		intString = value
	} else {
		if pIndex+1 < len(value) {
			intString = value[:pIndex] + value[pIndex+1:]
		} else {
			intString = value[:pIndex]
		}
		expInt := -len(value[pIndex+1:])
		exp += int64(expInt)
	}

	var dValue *big.Int
	// strconv.ParseInt is faster than new(big.Int).SetString so this is just a shortcut for strings we know won't overflow
	if len(intString) <= 18 {
		parsed64, err := strconv.ParseInt(intString, 10, 64)
		if err != nil {
			return Decimal{}, fmt.Errorf("can't convert %s to decimal", value)
		}
		dValue = big.NewInt(parsed64)
	} else {
		dValue = new(big.Int)
		_, ok := dValue.SetString(intString, 10)
		if !ok {
			return Decimal{}, fmt.Errorf("can't convert %s to decimal", value)
		}
	}

	if exp < math.MinInt32 || exp > math.MaxInt32 {
		// NOTE(vadim): I doubt a string could realistically be this long
		return Decimal{}, fmt.Errorf("can't convert %s to decimal: fractional part too long", originalInput)
	}
	if exp < -MaxAbsExponent || exp > MaxAbsExponent {
		return Decimal{}, fmt.Errorf("can't convert %s to decimal: exponent exceeds %d", originalInput, MaxAbsExponent)
	}

	return Decimal{
		value: dValue,
		exp:   int32(exp),
	}, nil
}

// FitsNumeric reports whether d can be stored exactly in NUMERIC(precision, scale).
// It avoids rendering the decimal, so exponent notation cannot trigger a large allocation.
func (d Decimal) FitsNumeric(precision, scale int32) bool {
	if precision <= 0 || scale < 0 || scale > precision {
		return false
	}
	if d.Sign() == 0 {
		return true
	}

	digits := new(big.Int).Abs(d.getValue()).String()
	exp := d.exp
	for len(digits) > 1 && exp < 0 && digits[len(digits)-1] == '0' {
		digits = digits[:len(digits)-1]
		exp++
	}

	fractionalDigits := int32(0)
	if exp < 0 {
		fractionalDigits = -exp
	}
	integerDigits := int32(len(digits)) + exp
	if integerDigits < 0 {
		integerDigits = 0
	}

	return fractionalDigits <= scale && integerDigits <= precision-scale
}

func (d Decimal) String() string {
	return d.string(true, false)
}

// Add returns d + d2.
func (d Decimal) Add(d2 Decimal) Decimal {
	rd, rd2 := RescalePair(d, d2)

	d3Value := new(big.Int).Add(rd.getValue(), rd2.getValue())
	return Decimal{
		value: d3Value,
		exp:   rd.exp,
	}
}

// Sub returns d - d2.
func (d Decimal) Sub(d2 Decimal) Decimal {
	rd, rd2 := RescalePair(d, d2)

	d3Value := new(big.Int).Sub(rd.getValue(), rd2.getValue())
	return Decimal{
		value: d3Value,
		exp:   rd.exp,
	}
}

// RescalePair rescales two decimals to common exponential value (minimal exp of both decimals)
func RescalePair(d1 Decimal, d2 Decimal) (Decimal, Decimal) {
	if d1.exp < d2.exp {
		return d1, d2.rescale(d1.exp)
	} else if d1.exp > d2.exp {
		return d1.rescale(d2.exp), d2
	}

	return d1, d2
}

// Compare compares the numbers represented by d and d2 and returns:
//
//	-1 if d <  d2
//	 0 if d == d2
//	+1 if d >  d2
func (d Decimal) Compare(d2 Decimal) int {
	return d.Cmp(d2)
}

func (d Decimal) Cmp(d2 Decimal) int {
	if d.exp == d2.exp {
		return d.getValue().Cmp(d2.getValue())
	}

	rd, rd2 := RescalePair(d, d2)

	return rd.getValue().Cmp(rd2.getValue())
}

func (d Decimal) ScientificNotationString() string {
	exp := int(d.exp)
	intStr := new(big.Int).Abs(d.getValue()).String()
	if intStr == "0" {
		return intStr
	}
	first := intStr[0]
	var remaining string
	if len(intStr) > 1 {
		remaining = "." + intStr[1:]
		exp = exp + len(intStr) - 1
	}
	number := string(first) + remaining + "E" + strconv.Itoa(exp)
	if d.value.Sign() < 0 {
		return "-" + number
	}
	return number
}

func (d Decimal) string(trimTrailingZeros, useScientificNotation bool) string {
	if d.exp == 0 {
		return d.rescale(0).getValue().String()
	}
	if d.exp >= 0 {
		if useScientificNotation {
			return d.ScientificNotationString()
		} else {
			return d.rescale(0).value.String()
		}
	}

	abs := new(big.Int).Abs(d.getValue())
	str := abs.String()

	var intPart, fractionalPart string

	// NOTE(vadim): this cast to int will cause bugs if d.exp == INT_MIN
	// and you are on a 32-bit machine. Won't fix this super-edge case.
	dExpInt := int(d.exp)
	if len(str) > -dExpInt {
		intPart = str[:len(str)+dExpInt]
		fractionalPart = str[len(str)+dExpInt:]
	} else {
		intPart = "0"

		num0s := -dExpInt - len(str)
		fractionalPart = strings.Repeat("0", num0s) + str
	}

	if trimTrailingZeros {
		i := len(fractionalPart) - 1
		for ; i >= 0; i-- {
			if fractionalPart[i] != '0' {
				break
			}
		}
		fractionalPart = fractionalPart[:i+1]
	}

	number := intPart
	if len(fractionalPart) > 0 {
		number += "." + fractionalPart
	}

	if d.getValue().Sign() < 0 {
		return "-" + number
	}

	return number
}

func (d Decimal) getValue() *big.Int {
	if d.value == nil {
		return zeroInt
	}
	return d.value
}

func (d Decimal) rescale(exp int32) Decimal {
	if d.exp == exp {
		return Decimal{
			new(big.Int).Set(d.getValue()),
			d.exp,
		}
	}

	// NOTE(vadim): must convert exps to float64 before - to prevent overflow
	diff := math.Abs(float64(exp) - float64(d.exp))
	value := new(big.Int).Set(d.getValue())

	expScale := new(big.Int).Exp(tenInt, big.NewInt(int64(diff)), nil)
	if exp > d.exp {
		value = value.Quo(value, expScale)
	} else if exp < d.exp {
		value = value.Mul(value, expScale)
	}

	return Decimal{
		value: value,
		exp:   exp,
	}
}

// Sign returns:
//
//	-1 if d <  0
//	 0 if d == 0
//	+1 if d >  0
func (d Decimal) Sign() int {
	return d.getValue().Sign()
}

// IsPositive return
//
//	true if d > 0
//	false if d == 0
//	false if d < 0
func (d Decimal) IsPositive() bool {
	return d.Sign() == 1
}

func Zero() Decimal {
	return Decimal{
		value: big.NewInt(0),
		exp:   0,
	}
}

// MarshalJSON implements json.Marshaler
func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON implements json.Unmarshaler
func (d *Decimal) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		// If it's not a string, try to unmarshal as number and convert to string
		var n float64
		if err2 := json.Unmarshal(data, &n); err2 == nil {
			s = strconv.FormatFloat(n, 'f', -1, 64)
		} else {
			return err
		}
	}

	var err error
	*d, err = NewFromString(s)
	return err
}
