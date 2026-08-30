package common

/*func Add(a, b int) int {
	return a + b
}*/

/*func addInt(a, b int) int {
	return a + b
}

func addFloat(a, b float64) float64 {
	return a + b
}*/

/*func Add[T any](a, b T) T {
	return a + b
}*/

/*func Add[T int | float64 | uint](a, b T) T {
	return a + b
}*/

// The ~ prefix means "any type whose UNDERLYING type is this". Without it the
// constraint accepts only int, float64 and uint exactly, and rejects defined types
// such as `type Celsius float64`.
type number interface {
	~int | ~float64 | ~uint
}

func Add[T number](a, b T) T {
	return a + b
}
