package main

import (
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "time"

    "github.com/getlantern/systray"
)

type HubstaffStatus struct {
    ActiveProject struct {
        TrackedToday string `json:"tracked_today"`
    } `json:"active_project"`
    Tracking bool `json:"tracking"`
}

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

    // Run a goroutine to update the time from Hubstaff CLI
    go func() {
        for {
            trackedTime, err := getTrackedTime()
            if err != nil {
                systray.SetTitle("Error fetching time")
                fmt.Println("Error fetching tracked time:", err)
            } else {
                systray.SetTitle(fmt.Sprintf("Tracked: %s", trackedTime))
            }
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
        fmt.Printf("Error during downloading icon: %v\n", err)
    }
    return icon
}

// getTrackedTime fetches the tracked time from Hubstaff CLI
func getTrackedTime() (string, error) {
    // Get the home directory
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }

    // Create the command with the specified directory
    cmd := exec.Command("./HubstaffCLI.bin.x86_64", "status")
    cmd.Dir = filepath.Join(homeDir, "Hubstaff")

    output, err := cmd.Output()
    if err != nil {
        return "", err
    }

    var status HubstaffStatus
    err = json.Unmarshal(output, &status)
    if err != nil {
        return "", err
    }

    return status.ActiveProject.TrackedToday, nil
}
