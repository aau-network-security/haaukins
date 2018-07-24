package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	ntp "github.com/aau-network-security/go-ntp"
)

func handleCancel(clean func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		clean()
		os.Exit(1)
	}()
}

func main() {
	handleCancel(func() {
		fmt.Println("cancelled")
	})

	exerciseFlag := flag.String("e", "", "exercises")
	configFile := flag.String("c", "exercises.yml", "config file")
	flag.Parse()

	conf := loadConfig(*configFile)

	wantedExerTags := strings.Split(*exerciseFlag, ",")
	var lab []*ntp.ExerciseConfig

	for _, t := range wantedExerTags {
		exec, ok := conf.TagExercise[t]
		if !ok {
			log.Fatalf("Unknown exercise by tag: %s", t)
		}

		lab = append(lab, exec)
	}

	fmt.Println(lab)

}
