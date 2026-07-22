// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperexpr

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// MaxDecimalScale is the deterministic precision bound for expression
// numbers. Operations that cannot produce an exact result within this bound
// fail instead of rounding.
const MaxDecimalScale uint8 = 9

var allowedUnits = map[string]bool{"pt": true, "mm": true, "cm": true, "in": true, "px": true, "pc": true, "em": true, "rem": true, "vh": true, "vw": true, "%": true, "fr": true}

func validUnit(unit string) bool { return allowedUnits[unit] }

func absoluteUnitFactor(unit string) (numerator, denominator int64, ok bool) {
	switch unit {
	case "in":
		return 1, 1, true
	case "cm":
		return 50, 127, true
	case "mm":
		return 5, 127, true
	case "pt":
		return 1, 72, true
	case "pc":
		return 1, 6, true
	case "px":
		return 1, 96, true
	default:
		return 0, 0, false
	}
}

func unitsCompatible(left, right string) bool {
	if left == right {
		return true
	}
	_, _, leftAbsolute := absoluteUnitFactor(left)
	_, _, rightAbsolute := absoluteUnitFactor(right)
	return leftAbsolute && rightAbsolute
}

// ParseNumber converts a canonical base-10 number to its exact bounded value.
func ParseNumber(raw string) (Value, error) {
	negative := strings.HasPrefix(raw, "-")
	digits := strings.TrimPrefix(raw, "-")
	parts := strings.Split(digits, ".")
	if raw == "" || strings.HasPrefix(raw, "+") || len(parts) > 2 || parts[0] == "" || len(parts[0]) > 1 && parts[0][0] == '0' || negative && digits == "0" {
		return Value{}, fmt.Errorf("%w: number must use canonical base-10 notation", ErrInvalid)
	}
	scale := 0
	if len(parts) == 2 {
		if parts[1] == "" || strings.HasSuffix(parts[1], "0") || len(parts[1]) > int(MaxDecimalScale) {
			return Value{}, fmt.Errorf("%w: decimal exceeds canonical precision", ErrInvalid)
		}
		scale = len(parts[1])
		digits = parts[0] + parts[1]
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return Value{}, fmt.Errorf("%w: number must use canonical base-10 notation", ErrInvalid)
		}
	}
	if negative {
		digits = "-" + digits
	}
	coefficient, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return Value{}, fmt.Errorf("%w: number is outside the bounded int64 decimal range", ErrInvalid)
	}
	return Value{Kind: Integer, Integer: coefficient, Scale: uint8(scale)}, nil
}

// FormatNumber returns the canonical exact spelling of a number or unit value.
func FormatNumber(value Value) string {
	digits := strconv.FormatInt(value.Integer, 10)
	negative := strings.HasPrefix(digits, "-")
	if negative {
		digits = strings.TrimPrefix(digits, "-")
	}
	if value.Scale > 0 {
		for len(digits) <= int(value.Scale) {
			digits = "0" + digits
		}
		cut := len(digits) - int(value.Scale)
		digits = digits[:cut] + "." + digits[cut:]
	}
	if negative {
		digits = "-" + digits
	}
	return digits
}

func normalizeNumber(value Value) Value {
	for value.Scale > 0 && value.Integer%10 == 0 {
		value.Integer /= 10
		value.Scale--
	}
	return value
}

func numberBig(value Value, scale uint8) *big.Int {
	result := big.NewInt(value.Integer)
	if scale > value.Scale {
		result.Mul(result, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale-value.Scale)), nil))
	}
	return result
}

func checkedBig(kind Kind, coefficient *big.Int, scale uint8, unit string) (Value, error) {
	value := normalizeNumber(Value{Kind: kind, Integer: coefficient.Int64(), Scale: scale, Unit: unit})
	if !coefficient.IsInt64() || value.Scale > MaxDecimalScale {
		return Value{}, fmt.Errorf("%w: decimal overflow", ErrType)
	}
	return value, nil
}

func exactQuotient(kind Kind, numerator, denominator *big.Int, unit string) (Value, error) {
	scaled := new(big.Int).Mul(numerator, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(MaxDecimalScale)), nil))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(scaled, denominator, remainder)
	if remainder.Sign() != 0 {
		return Value{}, fmt.Errorf("%w: result exceeds exact decimal precision", ErrType)
	}
	return checkedBig(kind, quotient, MaxDecimalScale, unit)
}

func convertUnit(value Value, target string) (Value, error) {
	if value.Kind != Unit || !unitsCompatible(value.Unit, target) {
		return Value{}, fmt.Errorf("%w: incompatible units %q and %q", ErrType, value.Unit, target)
	}
	if value.Unit == target {
		return value, nil
	}
	fromNumerator, fromDenominator, _ := absoluteUnitFactor(value.Unit)
	toNumerator, toDenominator, _ := absoluteUnitFactor(target)
	numerator := new(big.Int).Mul(big.NewInt(value.Integer), big.NewInt(fromNumerator*toDenominator))
	denominator := new(big.Int).Mul(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(value.Scale)), nil), big.NewInt(fromDenominator*toNumerator))
	return exactQuotient(Unit, numerator, denominator, target)
}

func compareNumber(left, right Value) int {
	scale := left.Scale
	if right.Scale > scale {
		scale = right.Scale
	}
	return numberBig(left, scale).Cmp(numberBig(right, scale))
}

func negateNumber(value Value) (Value, error) {
	if value.Kind != Integer && value.Kind != Unit || value.Integer == -1<<63 {
		return Value{}, fmt.Errorf("%w: numeric negation overflow", ErrType)
	}
	value.Integer = -value.Integer
	return value, nil
}

func numericArithmetic(op Op, left, right Value) (Value, error) {
	numeric := func(v Value) bool { return v.Kind == Integer || v.Kind == Unit }
	if !numeric(left) || !numeric(right) {
		return Value{}, fmt.Errorf("%w: arithmetic requires numbers or units", ErrType)
	}
	kind, unit := Integer, ""
	switch op {
	case OpAddInteger, OpSubInteger:
		if left.Kind != right.Kind || left.Kind == Unit && !unitsCompatible(left.Unit, right.Unit) {
			return Value{}, fmt.Errorf("%w: addition and subtraction require matching units", ErrType)
		}
		kind, unit = left.Kind, left.Unit
		if kind == Unit && right.Unit != unit {
			var err error
			right, err = convertUnit(right, unit)
			if err != nil {
				return Value{}, err
			}
		}
		scale := left.Scale
		if right.Scale > scale {
			scale = right.Scale
		}
		coefficient := numberBig(left, scale)
		if op == OpAddInteger {
			coefficient.Add(coefficient, numberBig(right, scale))
		} else {
			coefficient.Sub(coefficient, numberBig(right, scale))
		}
		return checkedBig(kind, coefficient, scale, unit)
	case OpMultiplyInteger:
		if left.Kind == Unit && right.Kind == Unit {
			return Value{}, fmt.Errorf("%w: multiplying two units is unsupported", ErrType)
		}
		if left.Kind == Unit {
			kind, unit = Unit, left.Unit
		} else if right.Kind == Unit {
			kind, unit = Unit, right.Unit
		}
		coefficient := new(big.Int).Mul(big.NewInt(left.Integer), big.NewInt(right.Integer))
		return checkedBig(kind, coefficient, left.Scale+right.Scale, unit)
	case OpDivideInteger:
		if right.Integer == 0 {
			return Value{}, fmt.Errorf("%w: division by zero", ErrType)
		}
		if right.Kind == Unit || left.Kind == Unit && right.Kind != Integer {
			return Value{}, fmt.Errorf("%w: division supports number/number or unit/number", ErrType)
		}
		kind, unit = left.Kind, left.Unit
		numerator := new(big.Int).Mul(big.NewInt(left.Integer), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(right.Scale)), nil))
		denominator := new(big.Int).Mul(big.NewInt(right.Integer), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(left.Scale)), nil))
		return exactQuotient(kind, numerator, denominator, unit)
	default:
		return Value{}, ErrInvalid
	}
}
