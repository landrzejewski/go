package examples

import (
	"bufio"
	"fmt"
	"os"
)

const (
	empty    = "-"
	playerX  = "X"
	playerO  = "O"
	maxIndex = 2
)

var board = [][]string{
	{empty, empty, empty},
	{empty, empty, empty},
	{empty, empty, empty},
}

var currentPlayer = playerX
var movesCount = 0

func printBoard() {
	for _, row := range board {
		fmt.Println(row)
	}
}

func makeMove(row, col int) bool {
	if isFieldOutsideBoard(row, col) || isFieldTaken(row, col) {
		return false
	}
	board[row][col] = currentPlayer
	movesCount++
	return true
}

func changePlayer() {
	if currentPlayer == playerX {
		currentPlayer = playerO
	} else {
		currentPlayer = playerX
	}
}

// Nazwa musi zgadzać się ze zwracaną wartością: funkcja zwraca true, gdy pole
// jest POZA planszą. Wcześniej nazywała się isFieldOnBoard, co twierdziło
// dokładnie odwrotnie - kod działał tylko dlatego, że wywołanie kompensowało
// błąd, a pierwsza "naprawa" na !isFieldOnBoard dałaby index out of range.
func isFieldOutsideBoard(row, col int) bool {
	return row < 0 || row > maxIndex || col < 0 || col > maxIndex
}

func isFieldTaken(row, col int) bool {
	return board[row][col] != empty
}

// Zwracamy symbol, który FAKTYCZNIE utworzył zwycięską linię, a nie
// currentPlayer. Poprzednia wersja była poprawna tylko przy niewidocznym
// założeniu "checkWinner wołane dokładnie raz, zaraz po ruchu, przed
// changePlayer" - i psuła się przy każdym innym użyciu (np. sprawdzeniu
// wczytanej planszy).
func checkWinner() string {
	for i := 0; i <= maxIndex; i++ {
		if board[i][0] != empty && board[i][0] == board[i][1] && board[i][1] == board[i][2] {
			return board[i][0]
		}
		if board[0][i] != empty && board[0][i] == board[1][i] && board[1][i] == board[2][i] {
			return board[0][i]
		}
	}

	if board[0][0] != empty && board[0][0] == board[1][1] && board[1][1] == board[2][2] {
		return board[0][0]
	}
	if board[0][2] != empty && board[0][2] == board[1][1] && board[1][1] == board[2][0] {
		return board[0][2]
	}

	return empty
}

func isBoardFull() bool {
	return movesCount >= 9
}

func TicTacToe() {
	// Czytamy CAŁY wiersz i dopiero z niego parsujemy liczby. fmt.Scanln przy
	// błędzie parsowania nie konsumuje wadliwego tokenu, więc wpisanie "abc"
	// powodowało, że kolejne wywołanie potykało się o te same bajty i program
	// w nieskończoność wypisywał "Invalid move. Try again".
	input := bufio.NewScanner(os.Stdin)
	printBoard()
	for {
		fmt.Printf("Player %s, enter move (column, row): ", currentPlayer)
		if !input.Scan() {
			fmt.Println("\nKoniec wejścia, kończę grę.")
			return
		}

		var row, col int
		if _, err := fmt.Sscanf(input.Text(), "%d %d", &col, &row); err != nil || !makeMove(row, col) {
			fmt.Println("Invalid move. Try again")
			continue
		}

		printBoard()

		winner := checkWinner()

		if winner != empty {
			fmt.Printf("Player %s wins\n", winner)
			break
		}

		if isBoardFull() {
			fmt.Println("The game is a draw!")
			break
		}

		changePlayer()
	}
}
