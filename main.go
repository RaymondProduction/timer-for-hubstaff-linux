package main

import (
    "fmt"
    "time"
    "os"

    "github.com/getlantern/systray"
)

func main() {
    systray.Run(onReady, onExit)
}

func onReady() {
    systray.SetIcon(getIcon("redIcon.png"))
    systray.SetTitle("Tray Clock")
    systray.SetTooltip("Tray Clock with Messages")

    // Create a menu item to display a message
    mMessage := systray.AddMenuItem("Show Message", "Show a text message")
    mQuit := systray.AddMenuItem("Quit", "Quit the whole app")

    // Run a goroutine to update the time
    go func() {
        for {
            now := time.Now().Format("15:04:05")
            systray.SetTitle(fmt.Sprintf("Time: %s", now))
            time.Sleep(1 * time.Second)
        }
    }()

    // Handle menu events
    go func() {
        for {
            select {
            case <-mMessage.ClickedCh:
                systray.SetTitle("New Message!")
                fmt.Println("This is your message!")
            case <-mQuit.ClickedCh:
                systray.Quit()
                fmt.Println("Quiting...")
                return
            }
        }
    }()
}

func onExit() {
    // Cleaning up resources before exiting
}


// getIcon reads an icon file from the given path.
func getIcon(filePath string) []byte {
	icon, err := os.ReadFile(filePath)
	if err != nil {
		//log.Fatalf("Error during downloading icon: %v", err)
		fmt.Printf("Error during downloading icon: %v\n", err)
	}
	return icon
}
