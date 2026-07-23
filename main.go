package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yongwei9527-art/s-ui-go/app"
	"github.com/yongwei9527-art/s-ui-go/cmd"
)

func runApp() {
	app := app.NewApp()

	err := app.Init()
	if err != nil {
		log.Fatal(err)
	}

	err = app.Start()
	if err != nil {
		log.Fatal(err)
	}

	sigCh := make(chan os.Signal, 1)
	// Trap shutdown signals
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM)
	for {
		sig := <-sigCh

		switch sig {
		case syscall.SIGHUP:
			if err := app.RestartApp(); err != nil {
				log.Fatal(err)
			}
		default:
			app.Stop()
			return
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		runApp()
		return
	} else {
		if err := cmd.ParseCmd(); err != nil {
			log.Print(err)
			os.Exit(1)
		}
	}
}
