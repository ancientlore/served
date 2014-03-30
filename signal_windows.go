package main

import (
	"os"
)

func getSignalChan() chan os.Signal {
	return make(chan os.Signal)
}
