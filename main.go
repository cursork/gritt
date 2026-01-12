package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/colorprofile"
	"gritt/ride"
)

func main() {
	addr := flag.String("addr", "localhost:4502", "Dyalog RIDE address")
	logFile := flag.String("log", "", "Log protocol messages to file")
	flag.Parse()

	// Detect terminal color capabilities
	colorProfile := colorprofile.Detect(os.Stdout, os.Environ())

	// Set up logging if requested
	var logWriter *os.File
	if *logFile != "" {
		var err error
		logWriter, err = os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		defer logWriter.Close()
		ride.Logger = logWriter // Protocol messages
		fmt.Printf("Logging to %s\n", *logFile)
	}

	fmt.Printf("Connecting to %s...\n", *addr)
	client, err := ride.Connect(*addr)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	p := tea.NewProgram(NewModel(client, logWriter, colorProfile), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
