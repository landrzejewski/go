package examples

import (
	"errors"
	"fmt"
)

// monetaryAmount stores the value in MINOR UNITS (grosze, cents) as an integer -
// never as a float64.
//
// Why: a binary float cannot represent 0.01, 0.10 or 0.20 exactly (the classic
// 0.1 + 0.2 != 0.3). Summing a ledger accumulates those errors, and %.2f
// formatting hides them - the balance quietly drifts away from the truth.
// For anything more serious, use a decimal package such as shopspring/decimal.
type monetaryAmount struct {
	minorUnits int64 // np. 12345 == 123,45 PLN
	currency   string
}

// Go convention: sentinel errors are named Err... so callers can test them with
// errors.Is. errors.New here because there is nothing to interpolate (an earlier
// version used fmt.Errorf and carried the typo "currnency").
var ErrCurrencyMismatch = errors.New("currency mismatch")

// newMonetaryAmount takes major and minor units, e.g. (123, 45, "PLN") for 123.45.
//
// The minor part is always added in the direction of the major part's sign,
// so (-1, 50, "PLN") is -1.50 and not -0.50. Writing units*100+minorUnits
// would get every negative amount wrong.
func newMonetaryAmount(units, minorUnits int64, currency string) *monetaryAmount {
	total := units*100 + minorUnits
	if units < 0 {
		total = units*100 - minorUnits
	}
	return &monetaryAmount{total, currency}
}

// String lets the type print itself - without it fmt.Println would show the raw
// &{12345 PLN}.
func (ma *monetaryAmount) String() string {
	sign := ""
	value := ma.minorUnits
	if value < 0 {
		sign = "-"
		value = -value
	}
	return fmt.Sprintf("%s%d.%02d %s", sign, value/100, value%100, ma.currency)
}

func (ma *monetaryAmount) add(amount *monetaryAmount) error {
	if err := ma.checkCurrency(amount); err != nil {
		return err
	}
	ma.minorUnits += amount.minorUnits
	return nil
}

func (ma *monetaryAmount) subtract(amount *monetaryAmount) error {
	if err := ma.checkCurrency(amount); err != nil {
		return err
	}
	ma.minorUnits -= amount.minorUnits
	return nil
}

// checkCurrency is the one rule both operations share. An earlier version routed
// both through an apply(a, b, operator func(...)) helper, but the callback bought
// nothing over two three-line methods - and its parameters were named after the
// type, shadowing monetaryAmount inside the function.
func (ma *monetaryAmount) checkCurrency(other *monetaryAmount) error {
	if ma.currency != other.currency {
		return ErrCurrencyMismatch
	}
	return nil
}

func MonetaryAmount() {
	amount := newMonetaryAmount(0, 10, "PLN")      // 0,10 PLN
	otherAmount := newMonetaryAmount(0, 20, "PLN") // 0,20 PLN

	// On float64 this sum would come out as 0.30000000000000004.
	if err := amount.add(otherAmount); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(amount) // 0.30 PLN

	// Mixing currencies returns an error rather than panicking.
	if err := amount.add(newMonetaryAmount(1, 0, "EUR")); errors.Is(err, ErrCurrencyMismatch) {
		fmt.Println("cannot add EUR to PLN:", err)
	}
}
