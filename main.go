package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Starting Quicks on :6380")

	if err := ListenAndServe(":6380"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
