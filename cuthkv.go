package main

import (
	"cuthkv/cache"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)
	go cache.InitCache()
	<-interrupt
	os.Exit(1)
}
