package main

import (
	"os"
	"os/signal"
)

func getSignalChan() <-chan os.Signal {
	x := make(chan os.Signal, 10)
	signal.Notify(x, os.Interrupt, os.Kill)
	return x
}
