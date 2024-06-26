package main

import (
	"bytes"
	"encoding/json"
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

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/fogleman/gg"
	"github.com/getlantern/systray"
)

type HubstaffStatus struct {
	ActiveProject struct {
		TrackedToday string `json:"tracked_today"`
	} `json:"active_project"`
	Tracking bool `json:"tracking"`
}

var (
	timezone    string
	trackedTime time.Duration
	redIcon     []byte
)

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	timezone = "Local"

	redIcon = getIcon("redIcon.png")
	fmt.Println("Red icon size:", len(redIcon))
	systray.SetIcon(redIcon)

	// Play sound on startup
	//go playSound("resources/start.mp3")

	localTime := systray.AddMenuItem("Local time", "Local time")
	hcmcTime := systray.AddMenuItem("Ho Chi Minh time", "Asia/Ho_Chi_Minh")
	sydTime := systray.AddMenuItem("Sydney time", "Australia/Sydney")
	gdlTime := systray.AddMenuItem("Guadalajara time", "America/Mexico_City")
	sfTime := systray.AddMenuItem("San Fransisco time", "America/Los_Angeles")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quits this app")

	go func() {
		trackedTime, _ = fetchInitialTime()
		for {

			displayTime, second := getClockTime(timezone)

			fmt.Printf("Display time = %s, second =  %d\n", displayTime, second)
			if int(trackedTime.Seconds())%60 == 0 {
				if int(trackedTime.Minutes())%60 == 0 || int(trackedTime.Minutes())%60 == 30 {
					playSound("resources/alarm-clock-elapsed.oga")
				}
				trackedTime, _ = fetchInitialTime()
			} else {
				trackedTime += time.Second
			}
			systray.SetTitle(formatDuration(trackedTime))
			systray.SetTooltip(timezone + " timezone")
			systray.SetIcon(createProgressIcon(float64(second) / float64(60)))
			time.Sleep(1 * time.Second)
		}
	}()

	go func() {
		for {
			select {
			case <-localTime.ClickedCh:
				timezone = "Local"
			case <-hcmcTime.ClickedCh:
				timezone = "Asia/Ho_Chi_Minh"
			case <-sydTime.ClickedCh:
				timezone = "Australia/Sydney"
			case <-gdlTime.ClickedCh:
				timezone = "America/Mexico_City"
			case <-sfTime.ClickedCh:
				timezone = "America/Los_Angeles"
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	// Cleaning stuff here.
}

func getClockTime(tz string) (string, int) {
	t := time.Now()
	utc, _ := time.LoadLocation(tz)

	t2 := t.In(utc)

	return t2.Format("15:04:05"), t2.Second()
}

// getIcon reads an icon file from the given path.
func getIcon(filePath string) []byte {
	icon, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error during downloading icon: %v\n", err)
	}
	return icon
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

// fetchInitialTime fetches the tracked time from Hubstaff CLI and converts it to a time.Duration
func fetchInitialTime() (time.Duration, bool) {

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

// playSound plays the specified sound file
func playSound(filePath string) {
	// try use pulseaudio package
	cmd := exec.Command("paplay", filePath)
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error playing sound:", err)
		fmt.Println("Try to use alsa package")
		// use alsa package
		f, err := os.Open(filePath)
		if err != nil {
			fmt.Println("Error opening sound file:", err)
			return
		}
		defer f.Close()

		streamer, format, err := mp3.Decode(f)
		if err != nil {
			// Error decoding sound file: mp3: mp3: MPEG version 2.5 is not supported
			fmt.Println("Error decoding sound file:", err)
			return
		}
		defer streamer.Close()

		speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

		done := make(chan bool)
		speaker.Play(beep.Seq(streamer, beep.Callback(func() {
			done <- true
		})))

		<-done
	}
}
