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
	"log"
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
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

type HubstaffStatus struct {
	ActiveProject struct {
		TrackedToday string `json:"tracked_today"`
	} `json:"active_project"`
	Tracking bool `json:"tracking"`
}

var redIcon []byte

var testMode string

//var secondTicker *time.Ticker

var trackedTime time.Duration

var tracking bool

var win *gtk.Window

func main() {
	gtk.Init(nil)
	win = initGTKWindow()

	flag.StringVar(&testMode, "t", "", "Enable test mode with status JSON")
	flag.StringVar(&testMode, "test", "", "Enable test mode with status JSON")
	// Set the custom usage function
	flag.Usage = usage
	flag.Parse()

	go func() {
		systray.Run(onReady, onExit)
	}()

	gtk.Main()
}

// Custom usage function to provide detailed usage instructions and examples
func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintln(os.Stdout, []any{`
To use the test mode, run the application with the -t or --test flag followed by a JSON string representing the status.

Examples:

    Tracking active with 3 hours 50 minutes 18 seconds tracked today:

    ./main -t '{"active_project":{"id":3,"name":"Development","tracked_today":"3:50:18"},"tracking":true}'

    Tracking active with 5 hours 50 minutes 18 seconds tracked today:

    ./main --test '{"active_project":{"id":3,"name":"Development","tracked_today":"5:50:18"},"tracking":true}'

    Tracking inactive with 5 hours 50 minutes 18 seconds tracked today:

    ./main --test '{"active_project":{"id":3,"name":"Development","tracked_today":"5:50:18"},"tracking":false}'
`}...)
}

func initGTKWindow() *gtk.Window {

	// Create builder
	builder, err := gtk.BuilderNew()
	if err != nil {
		log.Fatal("Error bulder:", err)
	}

	// Lload the window from the Glade file into the builder
	err = builder.AddFromFile("resources/main.glade")
	if err != nil {
		log.Fatal("Error when loading glade file:", err)
	}

	// We get the object of the main window by ID
	obj, err := builder.GetObject("setting-window")
	if err != nil {
		log.Fatal("Error:", err)
	}

	win := obj.(*gtk.Window)

	// We get the object of the main window by ID
	objOpenFolder, err := builder.GetObject("open_folder")
	if err != nil {
		log.Fatal("Error:", err)
	}

	button := objOpenFolder.(*gtk.Button)

	objPath, err := builder.GetObject("path")
	if err != nil {
		log.Fatal("Error:", err)
	}

	entry := objPath.(*gtk.Entry)

	button.Connect("clicked", func() {
		dialog, err := gtk.FileChooserDialogNewWith2Buttons("Select folder", win, gtk.FILE_CHOOSER_ACTION_SELECT_FOLDER, "Cancel", gtk.RESPONSE_CANCEL, "Select", gtk.RESPONSE_ACCEPT)
		if err != nil {
			log.Fatal("Failed to create dialog box:", err)
		}
		defer dialog.Destroy()

		response := dialog.Run()
		if response == gtk.RESPONSE_ACCEPT {
			folderPath := dialog.GetFilename()
			log.Println("Selected folder:", folderPath)
			entry.SetText(folderPath)
		}
	})

	win.Connect("destroy", func() {
		fmt.Println("Destroy")
	})

	win.Connect("delete-event", func() bool {
		win.Hide()  // Hide the window.
		return true // Returning true prevents further propagation of the signal and stops the window from closing.
	})

	return win
}

func onReady() {
	redIcon = getIcon("redIcon.png")

	fmt.Println("Red icon size:", len(redIcon))

	systray.SetIcon(redIcon)
	systray.SetTitle("Tray Clock")
	systray.SetTooltip("Tray Clock with Messages")

	// Play sound on startup
	//go playSound("resources/start.mp3")

	// Create a menu item to display a message
	mSettings := systray.AddMenuItem("Settings", "Settings of Hubstaff Time Tracking Tray Application")
	mQuit := systray.AddMenuItem("Quit", "Quit of Hubstaff Time Tracking Tray Application")

	// Initial fetch of tracked time
	if testMode != "" {
		trackedTime, tracking = parseTestStatus(testMode)
		fmt.Println("Test mode. Fake time = ", formatDuration(trackedTime))

	} else {
		trackedTime, tracking = fetchInitialTime()
	}
	fmt.Println("First sync = ", formatDuration(trackedTime))
	fmt.Println("Tray to start")

	if tracking {
		updateIcon()
		startDisplay()
	}

	//syncAndUpdate()

	// Handle icon changes, display tracked time and menu events in the main goroutine
	for {
		select {
		case <-mSettings.ClickedCh:
			glib.IdleAdd(func() {
				win.ShowAll()
				win.Present() // Ensure the window is brought to the front
			})
			fmt.Println("Open settings window")
		case <-mQuit.ClickedCh:
			glib.IdleAdd(func() {
				win.Destroy()
				gtk.MainQuit()
			})
			systray.Quit()
			fmt.Println("Quitting...")
			return
		}
	}
}

func syncAndUpdate() {
	if testMode == "" {
		trackedTime, tracking = fetchInitialTime()
	}

	fmt.Println("Tracked time = ", formatDuration(trackedTime))
	if (int(trackedTime.Minutes())%60 == 0 || int(trackedTime.Minutes())%60 == 30) && int(trackedTime.Seconds())%60 == 0 {
		//	go playSound("resources/alarm-clock-elapsed.mp3")
	}
	updateIcon()
	// if tracking /* && secondTicker == nil*/ {
	// 	startDisplay()
	// } else if !tracking && secondTicker != nil {
	// 	stopSesondTickerForDisplay()
	// }
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
func startDisplay() {
	go func() {
		for {
			trackedTime += time.Second
			fmt.Println("Tracked: ", formatDuration(trackedTime))
			time.Sleep(500 * time.Millisecond)
			systray.SetTitle(fmt.Sprintf("Tracked: %s", formatDuration(trackedTime)))
			time.Sleep(500 * time.Millisecond)
			if int(trackedTime.Seconds())%60 == 0 {
				syncAndUpdate()
			}

			if int(trackedTime.Minutes())%60 == 0 && int(trackedTime.Seconds())%60 == 0 {
				//go playSound("resources/alarm-clock-elapsed.oga")
			}
		}
	}()

	// secondTicker = time.NewTicker(1 * time.Second)
	// fmt.Println("Starting second ticker for display")
	// go func() {
	// 	for range secondTicker.C {

	// 	}
	// }()
}

// stopTicker stops the ticker
func stopSesondTickerForDisplay() {
	// if secondTicker != nil {
	// 	secondTicker.Stop()
	// 	secondTicker = nil
	// }
	fmt.Println("Stopping second ticker for display")
	systray.SetIcon(redIcon)
}

// updateIcon updates the progress icon based on the tracked time
func updateIcon() {
	fmt.Println("Update icon")
	progress := float64(trackedTime) / float64(8*time.Hour) // 8 hours as 100%
	systray.SetIcon(createProgressIcon(progress))
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
