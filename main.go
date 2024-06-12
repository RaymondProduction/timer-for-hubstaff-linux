package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fogleman/gg"
	"github.com/getlantern/systray"
)

type HubstaffStatus struct {
	ActiveProject struct {
		TrackedToday string `json:"tracked_today"`
	} `json:"active_project"`
	Tracking bool `json:"tracking"`
}

var trackedTime time.Duration
var tracking bool
var ticker *time.Ticker

var redIcon []byte

var iconChangeChan chan []byte

var testMode string

func main() {
	flag.StringVar(&testMode, "t", "", "Enable test mode with status JSON")
	flag.StringVar(&testMode, "test", "", "Enable test mode with status JSON")
	flag.Parse()
	iconChangeChan = make(chan []byte, 1)
	systray.Run(onReady, onExit)
}

func onReady() {
	redIcon = getIcon("redIcon.png")

	fmt.Println("Red icon size:", len(redIcon))

	systray.SetIcon(redIcon)
	systray.SetTitle("Tray Clock")
	systray.SetTooltip("Tray Clock with Messages")

	// Play sound on startup
	go playSound("resources/start.wav")

	// Create a menu item to display a message
	mMessage := systray.AddMenuItem("Show Message", "Show a text message")
	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")

	// Initial fetch of tracked time
	trackedTime, tracking = fetchInitialTime()
	if tracking {
		startTicker()
		updateIcon()
	}

	// Run a goroutine to sync time with Hubstaff CLI every minute
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			trackedTime, tracking = fetchInitialTime()
			updateIcon()
			if tracking && ticker == nil {
				startTicker()
			} else if !tracking && ticker != nil {
				stopTicker()
			}
		}
	}()

	// Handle icon changes and menu events in the main goroutine
	for {
		select {
		case icon := <-iconChangeChan:
			fmt.Println("Main loop: Changing icon")
			systray.SetIcon(icon)
		case <-mMessage.ClickedCh:
			systray.SetTitle("New Message!")
			fmt.Println("This is your message!")
		case <-mQuit.ClickedCh:
			systray.Quit()
			fmt.Println("Quitting...")
			return
		}
	}
}

func onExit() {
	// Cleaning up resources before exiting
	close(iconChangeChan)
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
func fetchInitialTime() (time.Duration, bool) {
	if testMode != "" {
		return parseTestStatus(testMode)
	}

	// Get the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error fetching home directory:", err)
		return 0, false
	}

	// Create the command with the specified directory
	cmd := exec.Command("./HubstaffCLI.bin.x86_64", "status")
	cmd.Dir = filepath.Join(homeDir, "Hubstaff")

	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error fetching tracked time:", err)
		return 0, false
	}

	var status HubstaffStatus
	err = json.Unmarshal(output, &status)
	if err != nil {
		fmt.Println("Error parsing tracked time:", err)
		return 0, false
	}

	fmt.Println("Synchronization by command ./HubstaffCLI.bin.x86_64. TrackedToday = ", status.ActiveProject.TrackedToday)

	duration, err := parseDuration(status.ActiveProject.TrackedToday)
	if err != nil {
		fmt.Println("Error parsing duration:", err)
		return 0, false
	}

	return duration, status.Tracking
}

// parseTestStatus parses the test status JSON string and returns the tracked time and tracking status
func parseTestStatus(statusJSON string) (time.Duration, bool) {
	var status HubstaffStatus
	err := json.Unmarshal([]byte(statusJSON), &status)
	if err != nil {
		fmt.Println("Error parsing test status JSON:", err)
		return 0, false
	}

	duration, err := parseDuration(status.ActiveProject.TrackedToday)
	if err != nil {
		fmt.Println("Error parsing duration:", err)
		return 0, false
	}

	return duration, status.Tracking
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

// startTicker starts the ticker for updating the time every second
func startTicker() {
	ticker = time.NewTicker(1 * time.Second)
	fmt.Println("Starting ticker")
	go func() {
		for range ticker.C {
			trackedTime += time.Second
			systray.SetTitle(fmt.Sprintf("Tracked: %s", formatDuration(trackedTime)))

			if int(trackedTime.Minutes())%60 == 0 && int(trackedTime.Seconds())%60 == 0 {
				go playSound("resources/alarm-clock-elapsed.oga")
			}
		}
	}()
}

// stopTicker stops the ticker
func stopTicker() {
	if ticker != nil {
		ticker.Stop()
		ticker = nil
	}
	fmt.Println("Stopping ticker")
	iconChangeChan <- redIcon
}

// updateIcon updates the progress icon based on the tracked time
func updateIcon() {
	progress := float64(trackedTime) / float64(8*time.Hour) // 8 hours as 100%
	iconChangeChan <- createProgressIcon(progress)
}

// createProgressIcon creates an icon with a progress circle
func createProgressIcon(progress float64) []byte {
	const size = 64
	const borderThickness = 4 // Thickness of the border
	const radiusOffset = 4    // Offset to reduce the radius
	dc := gg.NewContext(size, size)

	// Draw transparent background
	dc.SetColor(color.RGBA{0, 0, 0, 0}) // Transparent color
	dc.Clear()

	// Draw progress circle
	dc.SetColor(color.RGBA{0, 255, 0, 255}) // Green color
	startAngle := -gg.Radians(90)
	endAngle := startAngle + (2 * math.Pi * progress)
	radius := float64(size)/2 - radiusOffset
	dc.DrawArc(float64(size)/2, float64(size)/2, radius, startAngle, endAngle)
	dc.LineTo(float64(size)/2, float64(size)/2)
	dc.ClosePath()
	dc.Fill()

	// Draw outer border
	dc.SetLineWidth(borderThickness)
	dc.SetColor(color.RGBA{0, 255, 0, 255}) // Green color
	dc.DrawCircle(float64(size)/2, float64(size)/2, radius)
	dc.Stroke()

	// Save to buffer
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), dc.Image(), image.Point{0, 0}, draw.Src)

	buf := new(bytes.Buffer)
	png.Encode(buf, img)
	return buf.Bytes()
}

// playSound uses paplay to play a sound file via PulseAudio
func playSound(filePath string) {
	cmd := exec.Command("paplay", filePath)
	if err := cmd.Run(); err != nil {
		fmt.Println("Error playing sound:", err)
	}
}
