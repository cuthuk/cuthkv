package main

import (
	"cuthkv/client"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)
	go client.InitClient()
	<-interrupt
	os.Exit(1)
}
