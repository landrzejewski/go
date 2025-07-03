package examples

import "flag"

func Cat() {
	numberLines := flag.Bool("n", false, "Number lines")
	numberNonEmptyLines := flag.Bool("nb", false, "Number non empty lines")
	flag.Parse()
	paths := flag.Args()
}
