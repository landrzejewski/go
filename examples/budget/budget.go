package budget

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const budgetFile = "budget.json"

type Budget struct {
	Entries []BudgetEntry `json:"entries"`
}

func (b *Budget) Add(entry *BudgetEntry) {
	b.Entries = append(b.Entries, *entry)
}

func (b *Budget) Print() {
	// The balance is computed in integers - no error accumulation.
	var balanceMinor int64
	for _, entry := range b.Entries {
		if entry.OperationType == Deposit {
			balanceMinor += entry.AmountMinor
		} else {
			balanceMinor -= entry.AmountMinor
		}
		entry.Print()
	}
	fmt.Println("----------------------------------------------------")
	fmt.Printf("Balance: %v\n", FormatMinor(balanceMinor))
}

func (b *Budget) Save() error {
	bytes, err := json.MarshalIndent(b, "", "\t")
	if err != nil {
		return fmt.Errorf("encoding the budget: %w", err)
	}
	if err := os.WriteFile(budgetFile, bytes, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", budgetFile, err)
	}
	return nil
}

// Load is a constructor function, not a method: the previous
// `func (b *Budget) Load() *Budget` never used its receiver at all, which forced
// the odd `budget = budget.Load()` on the caller.
//
// A missing file is not an error - on the first run we start from an empty budget.
// Load used to panic, so the tool could not initialise itself at all. Errors are
// returned instead of panicking, which would also lose the original err.
func Load() (*Budget, error) {
	budget := &Budget{}
	bytes, err := os.ReadFile(budgetFile)
	if errors.Is(err, os.ErrNotExist) {
		return budget, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", budgetFile, err)
	}
	if err := json.Unmarshal(bytes, budget); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", budgetFile, err)
	}
	return budget, nil
}

// EntryFromArgs reads an entry from the command-line arguments: amount and
// description. It returns an error instead of silently doing nothing - a typo in
// the amount (or the wrong argument count) used to produce neither an entry nor
// any message.
//
// Reading os.Args inside a domain type is not ideal (it makes the method hard to
// test); a larger program would parse the arguments in main and pass them in.
func (b *Budget) EntryFromArgs() error {
	args := os.Args[1:]
	if len(args) != 2 {
		return fmt.Errorf("expected 2 arguments (amount description), got %d", len(args))
	}
	entry, err := parseArgs(args)
	if err != nil {
		return err
	}
	b.Add(entry)
	return nil
}

// parseArgs takes an amount formatted as "12.34" (or "-12.34" for an expense)
// and converts it to minor units without going through float64.
func parseArgs(args []string) (*BudgetEntry, error) {
	minor, err := parseAmountToMinor(args[0])
	if err != nil {
		return nil, err
	}

	operationType := Deposit
	if minor < 0 {
		operationType = Withdraw
		minor = -minor
	}

	return NewBudgetEntry(minor, operationType, args[1]), nil
}

func parseAmountToMinor(value string) (int64, error) {
	negative := strings.HasPrefix(value, "-")
	value = strings.TrimPrefix(strings.TrimPrefix(value, "-"), "+")

	units, cents, found := strings.Cut(value, ".")
	if !found {
		cents = "0"
	}
	// Pad to two digits: "5" -> "50" (i.e. 50 grosze); "123" is an error.
	switch len(cents) {
	case 1:
		cents += "0"
	case 2:
	default:
		return 0, fmt.Errorf("invalid amount %q: expected at most 2 digits after the dot", value)
	}

	unitsValue, err := strconv.ParseInt(units, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", value, err)
	}
	centsValue, err := strconv.ParseInt(cents, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", value, err)
	}

	minor := unitsValue*100 + centsValue
	if negative {
		minor = -minor
	}
	return minor, nil
}
