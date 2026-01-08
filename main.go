package main

import (
	"flag"
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"gritt/ride"
)

func main() {
	addr := flag.String("addr", "localhost:4502", "Dyalog RIDE address")
	flag.Parse()

	fmt.Printf("Connecting to %s...\n", *addr)
	client, err := ride.Connect(*addr)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	p := tea.NewProgram(NewModel(client), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
