package p

import "fmt"

func F0New() error {
	return nil
}

func F1() error {
	return fmt.Errorf("error")
}
