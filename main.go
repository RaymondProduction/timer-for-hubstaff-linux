package main

import (
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    "github.com/getlantern/systray"
)

type HubstaffStatus struct {
    ActiveProject struct {
        TrackedToday string `json:"tracked_today"`
    } `json:"active_project"`
    Tracking bool `json:"tracking"`
}

var trackedTime time.Duration

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

    // Initial fetch of tracked time
    trackedTime = fetchInitialTime()

    // Run a goroutine to update the time every second
    go func() {
        ticker := time.NewTicker(1 * time.Second)
        for range ticker.C {
            trackedTime += time.Second
            systray.SetTitle(fmt.Sprintf("Tracked: %s", formatDuration(trackedTime)))
        }
    }()

    // Run a goroutine to sync time with Hubstaff CLI every minute
    go func() {
        for {
            time.Sleep(1 * time.Minute)
            trackedTime = fetchInitialTime()
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

// fetchInitialTime fetches the tracked time from Hubstaff CLI and converts it to a time.Duration
func fetchInitialTime() time.Duration {
    // Get the home directory
    homeDir, err := os.UserHomeDir()
    if err != nil {
        fmt.Println("Error fetching home directory:", err)
        return 0
    }

    // Create the command with the specified directory
    cmd := exec.Command("./HubstaffCLI.bin.x86_64", "status")
    cmd.Dir = filepath.Join(homeDir, "Hubstaff")

    output, err := cmd.Output()
    if err != nil {
        fmt.Println("Error fetching tracked time:", err)
        return 0
    }

    var status HubstaffStatus
    err = json.Unmarshal(output, &status)
    if err != nil {
        fmt.Println("Error parsing tracked time:", err)
        return 0
    }

    fmt.Println("Synchronization by command ./HubstaffCLI.bin.x86_64. TrackedToday = ", status.ActiveProject.TrackedToday)

    duration, err := parseDuration(status.ActiveProject.TrackedToday)
    if err != nil {
        fmt.Println("Error parsing duration:", err)
        return 0
    }

    return duration
}

// parseDuration parses a duration string in the format "hh:mm:ss" to time.Duration
func parseDuration(s string) (time.Duration, error) {
    parts := strings.Split(s, ":")
    if len(parts) != 3 {
        return 0, fmt.Errorf("invalid duration format")
    }
    hours, err := strconv.Atoi(parts[0])
    if err != nil {
        return 0, err
    }
    minutes, err := strconv.Atoi(parts[1])
    if err != nil {
        return 0, err
    }
    seconds, err := strconv.Atoi(parts[2])
    if err != nil {
        return 0, err
    }
    return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second, nil
}

// formatDuration formats a time.Duration to a string in the format "hh:mm:ss"
func formatDuration(d time.Duration) string {
    hours := int(d.Hours())
    minutes := int(d.Minutes()) % 60
    seconds := int(d.Seconds()) % 60
    return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
