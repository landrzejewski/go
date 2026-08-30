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
	// Saldo liczone na liczbach całkowitych - żadnej kumulacji błędu.
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
		return fmt.Errorf("zapis budżetu: %w", err)
	}
	if err := os.WriteFile(budgetFile, bytes, 0644); err != nil {
		return fmt.Errorf("zapis %s: %w", budgetFile, err)
	}
	return nil
}

// Load to funkcja konstruująca, a nie metoda: poprzednia wersja
// `func (b *Budget) Load() *Budget` w ogóle nie używała swojego odbiornika,
// co wymuszało u wywołującego dziwne `budget = budget.Load()`.
//
// Brak pliku nie jest błędem - przy pierwszym uruchomieniu zaczynamy od pustego
// budżetu. Wcześniej Load panikował, więc narzędzie nie potrafiło się w ogóle
// zainicjować. Błędy zwracamy, zamiast panikować i gubić przy tym oryginalny err.
func Load() (*Budget, error) {
	budget := &Budget{}
	bytes, err := os.ReadFile(budgetFile)
	if errors.Is(err, os.ErrNotExist) {
		return budget, nil
	}
	if err != nil {
		return nil, fmt.Errorf("odczyt %s: %w", budgetFile, err)
	}
	if err := json.Unmarshal(bytes, budget); err != nil {
		return nil, fmt.Errorf("parsowanie %s: %w", budgetFile, err)
	}
	return budget, nil
}

// EntryFromArgs czyta wpis z argumentów wiersza poleceń: kwota i opis.
// Zwraca błąd zamiast po cichu nic nie robić - wcześniej literówka w kwocie
// (albo zła liczba argumentów) nie dawała ani wpisu, ani żadnego komunikatu.
func (b *Budget) EntryFromArgs() error {
	args := os.Args[1:]
	if len(args) != 2 {
		return fmt.Errorf("oczekiwano 2 argumentów (kwota opis), otrzymano %d", len(args))
	}
	entry, err := parseArgs(args)
	if err != nil {
		return err
	}
	b.Add(entry)
	return nil
}

// parseArgs przyjmuje kwotę w formacie "12.34" (albo "-12.34" dla wydatku)
// i zamienia ją na grosze bez pośrednictwa float64.
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
	// Dopełniamy/przycinamy do dwóch cyfr: "5" -> "50", "5" gr, "123" -> błąd.
	switch len(cents) {
	case 1:
		cents += "0"
	case 2:
	default:
		return 0, fmt.Errorf("niepoprawna kwota %q: oczekiwano co najwyżej 2 cyfr po kropce", value)
	}

	unitsValue, err := strconv.ParseInt(units, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("niepoprawna kwota %q: %w", value, err)
	}
	centsValue, err := strconv.ParseInt(cents, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("niepoprawna kwota %q: %w", value, err)
	}

	minor := unitsValue*100 + centsValue
	if negative {
		minor = -minor
	}
	return minor, nil
}
