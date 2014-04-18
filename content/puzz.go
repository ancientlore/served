package main

import (
	"fmt"
	"strings"
)

const (
	K = "GOTHACK"
	C = "SBARATMLHGLLUGISQDNDSGJGZDLKBGTF"
	A1 = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	A2 = "GOTHACKBDEFIJLMNPQRSUVWXYZ"
)

func main() {
	for _, c := range C {
		fmt.Printf("%c", A1[strings.Index(A2, string(c))])
	}
	fmt.Println()
}

