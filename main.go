package main

import (
	"bufio"
	"context"
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

func main() {
	totalStart := time.Now()
	defer measureTime(totalStart, "Total execution")

	fmt.Println("ADB Info Tool")
	fmt.Println("=============")

	deviceStart := time.Now()
	devices := getConnectedDevices()
	selectedDevice := selectDevice(devices)
	measureTime(deviceStart, "Device selection")

	fmt.Printf("Checking connection to device %s...\n", selectedDevice)
	connectStart := time.Now()
	err := checkDeviceConnectivity(selectedDevice, 5*time.Second)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("Please ensure the device is properly connected and try again.")
		os.Exit(1)
	}
	measureTime(connectStart, "Connection check")

	fmt.Printf("Fetching device information for %s...\n", selectedDevice)
	infoStart := time.Now()
	info := getDeviceInfo(selectedDevice)
	measureTime(infoStart, "Information fetching")

	outputStart := time.Now()
	fmt.Print(formatOutput(info))
	measureTime(outputStart, "Output formatting")
}
