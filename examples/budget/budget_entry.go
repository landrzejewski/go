package budget

import (
	"fmt"
	"time"
)

type BudgetEntry struct {
	// Kwota w GROSZACH (jednostkach minorowych) jako liczba całkowita.
	// float64 nie reprezentuje dokładnie 0.01/0.10/0.20, więc sumowanie księgi
	// kumulowałoby błąd, a formatowanie %.2f by go ukrywało.
	AmountMinor   int64         `json:"amountMinor"`
	OperationType OperationType `json:"operationType"`
	Timestamp     time.Time     `json:"timestamp"`
	Description   string        `json:"description"`
}

// Typ ZDEFINIOWANY, nie alias. Poprzednie `type OperationType = int` czyniło
// OperationType i int tym samym typem: dowolny int przechodził jako rodzaj
// operacji i nie dało się dodać metody String().
type OperationType int

const (
	Deposit OperationType = iota
	Withdraw
)

var operationName = map[OperationType]string{
	Deposit:  "Deposit",
	Withdraw: "Withdraw",
}

func (o OperationType) String() string {
	if name, exists := operationName[o]; exists {
		return name
	}
	return fmt.Sprintf("OperationType(%d)", int(o))
}

// FormatMinor formatuje grosze jako kwotę z dwoma miejscami po przecinku.
func FormatMinor(minor int64) string {
	sign := ""
	if minor < 0 {
		sign = "-"
		minor = -minor
	}
	return fmt.Sprintf("%s%d.%02d", sign, minor/100, minor%100)
}

func (e *BudgetEntry) Print() {
	date := e.Timestamp.Format(time.DateOnly)
	fmt.Printf("%v %10v %-12v %-20v\n", date, FormatMinor(e.AmountMinor), e.OperationType, e.Description)
}

func NewBudgetEntry(amountMinor int64, operationType OperationType, description string) *BudgetEntry {
	return &BudgetEntry{amountMinor, operationType, time.Now(), description}
}
