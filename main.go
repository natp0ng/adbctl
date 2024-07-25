package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

type DeviceInfo struct {
	Property string
	Value    string
}

var isDebug bool

func init() {
	isDebug = os.Getenv("DEBUG") != ""
}

func debugPrint(format string, a ...interface{}) {
	if isDebug {
		fmt.Printf(format, a...)
	}
}

func runAdbCommand(deviceID, command string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "adb", "-s", deviceID, "shell", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		debugPrint("Error executing command '%s': %v\n", command, err)
		return "n/a"
	}
	return strings.TrimSpace(string(output))
}

func getConnectedDevices() []string {
	cmd := exec.Command("adb", "devices")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error running adb devices:", err)
		os.Exit(1)
	}

	lines := strings.Split(string(output), "\n")
	var devices []string
	for _, line := range lines[1:] { // Skip the first line (header)
		if strings.TrimSpace(line) != "" && !strings.HasSuffix(line, "offline") {
			devices = append(devices, strings.Fields(line)[0])
		}
	}
	return devices
}

func selectDevice(devices []string) string {
	if len(devices) == 0 {
		fmt.Println("No devices connected.")
		fmt.Println("Please connect a device using 'adb connect <ip:port>' or ensure USB debugging is enabled.")
		fmt.Println("After connecting, run this tool again.")
		os.Exit(1)
	}
	if len(devices) == 1 {
		return devices[0]
	}

	fmt.Println("Multiple devices found. Please select a device:")
	for i, device := range devices {
		fmt.Printf("%d. %s\n", i+1, device)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter the number of the device you want to use: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		index := 0
		_, err := fmt.Sscanf(input, "%d", &index)
		if err == nil && index > 0 && index <= len(devices) {
			return devices[index-1]
		}
		fmt.Println("Invalid selection. Please try again.")
	}
}

func checkDeviceConnectivity(deviceID string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "adb", "-s", deviceID, "shell", "echo", "connected")
	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("device connection timed out after %v", timeout)
		}
		return fmt.Errorf("failed to connect to device: %v", err)
	}
	return nil
}

func parseMemInfo(meminfo string) string {
	lines := strings.Split(meminfo, "\n")
	var totalKB, freeKB, availableKB int
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			totalKB, _ = strconv.Atoi(strings.Fields(line)[1])
		} else if strings.HasPrefix(line, "MemFree:") {
			freeKB, _ = strconv.Atoi(strings.Fields(line)[1])
		} else if strings.HasPrefix(line, "MemAvailable:") {
			availableKB, _ = strconv.Atoi(strings.Fields(line)[1])
		}
	}
	usedKB := totalKB - availableKB
	totalGB := float64(totalKB) / 1048576.0
	usedGB := float64(usedKB) / 1048576.0
	freeGB := float64(freeKB) / 1048576.0
	return fmt.Sprintf("%.2f GB / %d kB (%.2f GB used, %.2f GB free)", totalGB, totalKB, usedGB, freeGB)
}

func parseCPUInfo(cpuinfo, cpuUsage string) string {
	lines := strings.Split(cpuinfo, "\n")
	var totalCores int
	for _, line := range lines {
		if strings.HasPrefix(line, "processor") {
			totalCores++
		}
	}

	usageFields := strings.Fields(cpuUsage)
	var usedCPU float64
	if len(usageFields) >= 4 {
		usedCPU, _ = strconv.ParseFloat(strings.TrimSuffix(usageFields[3], "%"), 64)
	}

	return fmt.Sprintf("%d cores (%.2f%% used)", totalCores, usedCPU)
}

func getDeviceInfo(deviceID string) []DeviceInfo {
	timeout := 5 * time.Second
	info := []DeviceInfo{
		{"Model", runAdbCommand(deviceID, "getprop ro.product.model", timeout)},
		{"Android Version", runAdbCommand(deviceID, "getprop ro.build.version.release", timeout)},
		{"API Level", runAdbCommand(deviceID, "getprop ro.build.version.sdk", timeout)},
		{"CPU ABI", runAdbCommand(deviceID, "getprop ro.product.cpu.abi", timeout)},
		{"Manufacturer", runAdbCommand(deviceID, "getprop ro.product.manufacturer", timeout)},
		{"Build Number", runAdbCommand(deviceID, "getprop ro.build.display.id", timeout)},
		{"Memory", parseMemInfo(runAdbCommand(deviceID, "cat /proc/meminfo", timeout))},
		{"CPU", parseCPUInfo(runAdbCommand(deviceID, "cat /proc/cpuinfo", timeout), runAdbCommand(deviceID, "top -n 1 | grep 'CPU:'", timeout))},
		{"Storage", runAdbCommand(deviceID, "df -h /data | tail -n 1 | awk '{print $2}'", timeout)},
		{"Free Storage", runAdbCommand(deviceID, "df -h /data | tail -n 1 | awk '{print $4}'", timeout)},
		{"Screen Resolution", runAdbCommand(deviceID, "wm size", timeout)},
		{"Screen Density", runAdbCommand(deviceID, "wm density", timeout)},
		{"Battery Level", runAdbCommand(deviceID, "dumpsys battery | grep level | awk '{print $2}'", timeout)},
		{"Fire OS Version", runAdbCommand(deviceID, "getprop ro.build.version.name", timeout)},
		{"Fire OS Build Number", runAdbCommand(deviceID, "getprop ro.build.version.number", timeout)},
	}

	return info
}

func formatOutput(info []DeviceInfo) string {
	var output strings.Builder
	maxWidth := 70 // Adjust this value to fit your terminal width

	// Title
	color.New(color.FgCyan, color.Bold).Fprintln(&output, "Device Information")
	output.WriteString(strings.Repeat("=", maxWidth) + "\n\n")

	// Group information
	groups := map[string][]string{
		"Device": {
			"Model", "Manufacturer", "Android Version", "API Level",
			"Build Number", "Fire OS Version", "Fire OS Build Number",
		},
		"Hardware": {
			"CPU", "CPU ABI", "Memory", "Storage", "Free Storage",
		},
		"Display": {
			"Screen Resolution", "Screen Density",
		},
		"Other": {
			"Battery Level",
		},
	}

	for groupName, properties := range groups {
		color.New(color.FgYellow, color.Bold).Fprintf(&output, "[ %s ]\n", groupName)
		for _, property := range properties {
			for _, item := range info {
				if item.Property == property {
					icon := getIcon(property)
					color.New(color.FgGreen).Fprintf(&output, "%-3s %-20s : ", icon, property)
					color.New(color.FgWhite).Fprintln(&output, item.Value)
					break
				}
			}
		}
		output.WriteString("\n")
	}

	return output.String()
}

func getIcon(property string) string {
	icons := map[string]string{
		"Model":                "📱",
		"Manufacturer":         "🏭",
		"Android Version":      "🤖",
		"API Level":            "🔢",
		"Build Number":         "🏗️",
		"Fire OS Version":      "🔥",
		"Fire OS Build Number": "🔥",
		"CPU":                  "💻",
		"CPU ABI":              "🧮",
		"Memory":               "💾",
		"Storage":              "💽",
		"Free Storage":         "🆓",
		"Screen Resolution":    "📺",
		"Screen Density":       "🔍",
		"Battery Level":        "🔋",
	}

	if icon, ok := icons[property]; ok {
		return icon
	}
	return "  "
}

func measureTime(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Printf("%s took %s\n", name, elapsed)
}

func getDetailedMemoryInfo(deviceID string) string {
	timeout := 5 * time.Second
	meminfo := runAdbCommand(deviceID, "cat /proc/meminfo", timeout)
	lines := strings.Split(meminfo, "\n")

	var output strings.Builder
	color.New(color.FgCyan, color.Bold).Fprintln(&output, "Detailed Memory Information")
	output.WriteString(strings.Repeat("=", 30) + "\n\n")

	formatSize := func(kb int) string {
		if kb > 1048576 {
			return fmt.Sprintf("%.2f GB", float64(kb)/1048576)
		} else if kb > 1024 {
			return fmt.Sprintf("%.2f MB", float64(kb)/1024)
		}
		return fmt.Sprintf("%d KB", kb)
	}

	memData := make(map[string]int)
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			key := strings.TrimSuffix(parts[0], ":")
			value, _ := strconv.Atoi(parts[1])
			memData[key] = value
		}
	}

	highlightedFields := []struct {
		key         string
		description string
	}{
		{"MemTotal", "Total RAM"},
		{"MemAvailable", "Available RAM"},
		{"MemFree", "Free RAM"},
		{"SwapTotal", "Total Swap"},
		{"SwapFree", "Free Swap"},
	}

	for _, field := range highlightedFields {
		if value, ok := memData[field.key]; ok {
			color.New(color.FgYellow, color.Bold).Fprintf(&output, "%-20s : ", field.description)
			color.New(color.FgWhite).Fprintln(&output, formatSize(value))
		}
	}

	output.WriteString("\n")

	// Calculate and display used memory
	usedMem := memData["MemTotal"] - memData["MemAvailable"]
	color.New(color.FgRed, color.Bold).Fprintf(&output, "%-20s : ", "Used RAM")
	color.New(color.FgWhite).Fprintln(&output, formatSize(usedMem))

	// Calculate and display used swap
	usedSwap := memData["SwapTotal"] - memData["SwapFree"]
	color.New(color.FgMagenta, color.Bold).Fprintf(&output, "%-20s : ", "Used Swap")
	color.New(color.FgWhite).Fprintln(&output, formatSize(usedSwap))

	output.WriteString("\nOther Memory Information:\n")
	output.WriteString(strings.Repeat("-", 25) + "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			key := strings.TrimSuffix(parts[0], ":")
			if !contains(highlightedFields, key) && key != "SwapFree" {
				value, err := strconv.Atoi(parts[1])
				if err == nil {
					color.New(color.FgGreen).Fprintf(&output, "%-20s : ", key)
					color.New(color.FgWhite).Fprintln(&output, formatSize(value))
				}
			}
		}
	}

	return output.String()
}

func contains(fields []struct{ key, description string }, key string) bool {
	for _, field := range fields {
		if field.key == key {
			return true
		}
	}
	return false
}

func main() {
	memoryFlag := flag.Bool("memory", false, "Show detailed memory information")
	flag.Parse()

	devices := getConnectedDevices()
	selectedDevice := selectDevice(devices)

	if *memoryFlag {
		fmt.Print(getDetailedMemoryInfo(selectedDevice))
		return
	}

	// If no flag is provided, show menu for information selection
	showInformationMenu(selectedDevice)
}

func showInformationMenu(deviceID string) {
	for {
		fmt.Println("\nWhat action would you like to perform?")
		fmt.Println("1. Show General Device Information")
		fmt.Println("2. Show Detailed Memory Information")
		fmt.Println("3. Reboot Device")
		fmt.Println("4. Start Application")
		fmt.Println("5. List Installed Applications")
		fmt.Println("6. Exit")

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter your choice (1-6): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			info := getDeviceInfo(deviceID)
			fmt.Print(formatOutput(info))
		case "2":
			fmt.Print(getDetailedMemoryInfo(deviceID))
		case "3":
			rebootDevice(deviceID)
		case "4":
			startApplication(deviceID)
		case "5":
			listInstalledApps(deviceID)
		case "6":
			fmt.Println("Exiting. Goodbye!")
			return
		default:
			fmt.Println("Invalid choice. Please try again.")
		}
	}
}

func rebootDevice(deviceID string) {
	fmt.Println("Rebooting device...")
	cmd := exec.Command("adb", "-s", deviceID, "reboot")
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error rebooting device: %v\n", err)
	} else {
		fmt.Println("Device is rebooting. Please wait...")
	}
}

func startApplication(deviceID string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the package name of the application to start: ")
	packageName, _ := reader.ReadString('\n')
	packageName = strings.TrimSpace(packageName)

	cmd := exec.Command("adb", "-s", deviceID, "shell", "monkey", "-p", packageName, "-c", "android.intent.category.LAUNCHER", "1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error starting application: %v\n", err)
		fmt.Println(string(output))
	} else {
		fmt.Printf("Application %s started successfully.\n", packageName)
	}
}

func listInstalledApps(deviceID string) {
	cmd := exec.Command("adb", "-s", deviceID, "shell", "pm", "list", "packages")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error listing installed applications: %v\n", err)
		return
	}

	fmt.Println("Installed Applications:")
	apps := strings.Split(string(output), "\n")
	for _, app := range apps {
		if strings.TrimSpace(app) != "" {
			fmt.Println(strings.TrimPrefix(app, "package:"))
		}
	}
}
