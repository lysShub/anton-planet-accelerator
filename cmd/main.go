package main

import (
	"fmt"
	"time"

	"github.com/lysShub/anton-planet-accelerator/event"
)

func main() {
	pl := event.NewProcessListen().Register("aces.exe")
	ul := event.NewUDPListener()

	for range time.Tick(time.Second) {
		es, err := pl.Walk()
		if err != nil {
			panic(err)
		}
		for _, e := range es {
			if e.Start() {
				ul.Register(e.Pid)
				fmt.Println(e.Pid)
			} else {
				ul.Delete(e.Pid)
			}
		}
		if ue, err := ul.Walk(); err != nil {
			panic(err)
		} else {
			for _, e := range ue {
				if e.Start() {
					fmt.Println(time.Now().Format(time.TimeOnly), "start", e.LocAddr.String(), e.Pid)
				} else {
					fmt.Println(time.Now().Format(time.TimeOnly), "close", e.LocAddr.String(), e.Pid)
				}
			}
		}
	}

}
