package main

import (
	"fmt"
	"log"
	"os"

	"github.com/eiannone/keyboard"
	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

func keyboardActions() {

	// --production flag
	if commands.Production {
		return
	}

	hbConfig := getHyperBricksConfiguration()

	if hbConfig.Mode == "" {
		return
	}

	// test and open keyboard
	if err := keyboard.Open(); err != nil {
		logging.GetLogger().Warnf("No keyboard...")
		return
	}

	defer func() {
		if err := keyboard.Close(); err != nil {
			log.Fatalf("Failed to close keyboard: %v", err)
		}
	}()

	// Channel to signal when "r" is pressed
	rPressed := make(chan bool)
	// Channel to handle program termination (e.g., on ESC key)
	done := make(chan bool)

	// Goroutine to listen for key presses
	go func() {
		for {
			char, key, err := keyboard.GetKey()
			if err != nil {
				log.Printf("Error reading key: %v", err)
				done <- true
				return
			}

			// Check if "r" or "R" is pressed
			if char == 'r' || char == 'R' {
				rPressed <- true
			}

			// Optional: Exit on q - Q - ESC key and KeyCtrlC
			if char == 'q' || char == 'Q' || key == keyboard.KeyEsc || key == keyboard.KeyCtrlC {
				done <- true
				return
			}
		}
	}()

	// Main loop to handle events
	for {
		select {
		case <-rPressed:
			if hbConfig.Development.Watch {
				fmt.Println("Detected 'r' key press!")
				PreProcessAndPopulateHyperbricksConfigurations()
			}
			// Place your action here
		case <-done:
			fmt.Println("Exiting program.")
			cancel()
			os.Exit(1)
			return
		}
	}
}
