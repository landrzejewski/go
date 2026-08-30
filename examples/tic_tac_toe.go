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

// The name has to match the return value: this reports true when the field is
// OUTSIDE the board. It used to be called isFieldOnBoard, which claimed exactly
// the opposite - the code only worked because the call site compensated for the
// mistake, and the obvious "fix" to !isFieldOnBoard would have given an index
// out of range.
func isFieldOutsideBoard(row, col int) bool {
	return row < 0 || row > maxIndex || col < 0 || col > maxIndex
}

func isFieldTaken(row, col int) bool {
	return board[row][col] != empty
}

// Return the symbol that ACTUALLY formed the winning line, not currentPlayer.
// The previous version was correct only under the invisible assumption
// "checkWinner is called exactly once, right after a move, before changePlayer"
// - and broke under any other use (for instance checking a board loaded from
// somewhere else).
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
	// Read the WHOLE line and only then parse numbers out of it. fmt.Scanln
	// on a parse error does not consume the offending token, so typing "abc" made
	// the next call trip over the same bytes and the program printed
	// "Invalid move. Try again" forever.
	input := bufio.NewScanner(os.Stdin)
	printBoard()
	for {
		fmt.Printf("Player %s, enter move (column, row): ", currentPlayer)
		if !input.Scan() {
			fmt.Println("\nEnd of input, quitting.")
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
