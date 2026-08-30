package examples

import (
	"errors"
	"fmt"
)

// monetaryAmount przechowuje kwotę w JEDNOSTKACH MINOROWYCH (groszach,
// centach) jako liczbę całkowitą - nigdy jako float64.
//
// Dlaczego: binarny float nie potrafi dokładnie reprezentować 0.01, 0.10 czy
// 0.20 (klasyczne 0.1 + 0.2 != 0.3). Przy sumowaniu księgi błędy się kumulują,
// a formatowanie %.2f je maskuje - saldo cicho odjeżdża od prawdy.
// Alternatywa dla poważniejszych zastosowań: pakiet shopspring/decimal.
type monetaryAmount struct {
	minorUnits int64 // np. 12345 == 123,45 PLN
	currency   string
}

// Konwencja Go: wartownicze błędy nazywamy Err..., żeby wywołujący mógł
// porównać je przez errors.Is. errors.New, bo nie ma tu żadnych weryfikatorów
// formatu (poprzednio było fmt.Errorf i literówka "currnency").
var ErrCurrencyMismatch = errors.New("currency mismatch")

// newMonetaryAmount przyjmuje jednostki główne i minorowe, np. (123, 45, "PLN").
func newMonetaryAmount(units, minorUnits int64, currency string) *monetaryAmount {
	return &monetaryAmount{units*100 + minorUnits, currency}
}

// String sprawia, że typ sam wie, jak się wypisać - bez tego fmt.Println
// drukowałby surowe &{12345 PLN}.
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
	return apply(ma, amount, func(monetaryAmount, otherMonetaryAmount *monetaryAmount) {
		monetaryAmount.minorUnits += otherMonetaryAmount.minorUnits
	})
}

func (ma *monetaryAmount) subtract(amount *monetaryAmount) error {
	return apply(ma, amount, func(monetaryAmount, otherMonetaryAmount *monetaryAmount) {
		monetaryAmount.minorUnits -= otherMonetaryAmount.minorUnits
	})
}

func apply(monetaryAmount, otherMonetaryAmount *monetaryAmount, operator func(monetaryAmount, otherMonetaryAmount *monetaryAmount)) error {
	if monetaryAmount.currency != otherMonetaryAmount.currency {
		return ErrCurrencyMismatch
	}
	operator(monetaryAmount, otherMonetaryAmount)
	return nil
}

func MonetaryAmount() {
	amount := newMonetaryAmount(0, 10, "PLN")      // 0,10 PLN
	otherAmount := newMonetaryAmount(0, 20, "PLN") // 0,20 PLN

	// Na float64 ta suma dałaby 0.30000000000000004.
	if err := amount.add(otherAmount); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println(amount) // 0.30 PLN

	// Operacja na różnych walutach zwraca błąd, a nie panikuje.
	if err := amount.add(newMonetaryAmount(1, 0, "EUR")); errors.Is(err, ErrCurrencyMismatch) {
		fmt.Println("Nie można dodać EUR do PLN:", err)
	}
}
