# timer-for-hubstaff-linux


# Hubstaff Time Tracking Tray Application

This application visualizes time tracking in the system tray from Hubstaff using the CLI for Linux (./Hubstaff/HubstaffCLI.bin.x86_64 help). It has been tested on Ubuntu 22.04.


## Features

- Displays the tracked time in the system tray.
- Updates the time every second.
- Changes the tray icon based on the tracking status.
- Supports a test mode for simulating different statuses.


# Using Test Mode for the Application

The application supports a test mode that allows you to simulate the status returned by the `HubstaffCLI.bin.x86_64` command. This can be useful for testing the application without needing to run the actual command.

## How to Use Test Mode

To use the test mode, run the application with the `-t` or `--test` flag followed by a JSON string representing the status.

### Examples

1. **Tracking active with 3 hours 50 minutes 18 seconds tracked today**:
```sh
./main -t '{"active_project":{"id":3,"name":"Development","tracked_today":"3:50:18"},"tracking":true}'
```

2. **Tracking active with 5 hours 50 minutes 18 seconds tracked today**:
```sh
./main --test '{"active_project":{"id":3,"name":"Development","tracked_today":"5:50:18"},"tracking":true}'
```

3. **Tracking inactive with 5 hours 50 minutes 18 seconds tracked today**:
```sh
./main --test '{"active_project":{"id":3,"name":"Development","tracked_today":"5:50:18"},"tracking":false}'
```