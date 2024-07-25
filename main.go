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
)

type DeviceInfo struct {
	Property string
	Value    string
}

func runAdbCommandWithTimeout(deviceID, command string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	if deviceID != "" {
		cmd = exec.CommandContext(ctx, "adb", "-s", deviceID, "shell", command)
	} else {
		cmd = exec.CommandContext(ctx, "adb", "shell", command)
	}

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "Error: Command timed out"
		}
		return "n/a"
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "n/a"
	}
	return result
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

func formatMemory(totalKB, usedKB, freeKB int) string {
	totalGB := float64(totalKB) / 1048576.0
	usedGB := float64(usedKB) / 1048576.0
	freeGB := float64(freeKB) / 1048576.0
	return fmt.Sprintf("%.2f GB / %d kB (%.2f GB used, %.2f GB free)", totalGB, totalKB, usedGB, freeGB)
}

func getMemoryInfo(deviceID string, timeout time.Duration) string {
	meminfo := runAdbCommandWithTimeout(deviceID, "cat /proc/meminfo", timeout)
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
	return formatMemory(totalKB, usedKB, freeKB)
}

func getCPUInfo(deviceID string, timeout time.Duration) string {
	cpuinfo := runAdbCommandWithTimeout(deviceID, "cat /proc/cpuinfo", timeout)
	lines := strings.Split(cpuinfo, "\n")
	var totalCores int
	for _, line := range lines {
		if strings.HasPrefix(line, "processor") {
			totalCores++
		}
	}

	// Get CPU usage (this is an approximation)
	cpuUsage := runAdbCommandWithTimeout(deviceID, "top -n 1 | grep 'CPU:'", timeout)
	usageFields := strings.Fields(cpuUsage)
	var usedCPU float64
	if len(usageFields) >= 4 {
		usedCPU, _ = strconv.ParseFloat(strings.TrimSuffix(usageFields[3], "%"), 64)
	}

	return fmt.Sprintf("%d cores (%.2f%% used)", totalCores, usedCPU)
}

func getFireOSInfo(deviceID string, timeout time.Duration) (string, string) {
	fireOSVersion := runAdbCommandWithTimeout(deviceID, "getprop ro.build.version.name", timeout)
	fireOSBuildNumber := runAdbCommandWithTimeout(deviceID, "getprop ro.build.version.number", timeout)
	return fireOSVersion, fireOSBuildNumber
}

func getDeviceInfo(deviceID string) []DeviceInfo {
	timeout := 5 * time.Second
	info := []DeviceInfo{
		{"Model", runAdbCommandWithTimeout(deviceID, "getprop ro.product.model", timeout)},
		{"Android Version", runAdbCommandWithTimeout(deviceID, "getprop ro.build.version.release", timeout)},
		{"API Level", runAdbCommandWithTimeout(deviceID, "getprop ro.build.version.sdk", timeout)},
		{"CPU ABI", runAdbCommandWithTimeout(deviceID, "getprop ro.product.cpu.abi", timeout)},
		{"Manufacturer", runAdbCommandWithTimeout(deviceID, "getprop ro.product.manufacturer", timeout)},
		{"Build Number", runAdbCommandWithTimeout(deviceID, "getprop ro.build.display.id", timeout)},
		{"Memory", getMemoryInfo(deviceID, timeout)},
		{"CPU", getCPUInfo(deviceID, timeout)},
		{"Storage", runAdbCommandWithTimeout(deviceID, "df -h /data | tail -n 1 | awk '{print $2}'", timeout)},
		{"Free Storage", runAdbCommandWithTimeout(deviceID, "df -h /data | tail -n 1 | awk '{print $4}'", timeout)},
		{"Screen Resolution", runAdbCommandWithTimeout(deviceID, "wm size", timeout)},
		{"Screen Density", runAdbCommandWithTimeout(deviceID, "wm density", timeout)},
		{"Battery Level", runAdbCommandWithTimeout(deviceID, "dumpsys battery | grep level | awk '{print $2}'", timeout)},
	}

	// Check if the device is an Amazon device
	manufacturer := runAdbCommandWithTimeout(deviceID, "getprop ro.product.manufacturer", timeout)
	if strings.ToLower(manufacturer) == "amazon" {
		fireOSVersion, fireOSBuildNumber := getFireOSInfo(deviceID, timeout)
		info = append(info, DeviceInfo{"Fire OS Version", fireOSVersion})
		info = append(info, DeviceInfo{"Fire OS Build Number", fireOSBuildNumber})
	}

	return info
}

func formatOutput(info []DeviceInfo) string {
	var maxLength int
	for _, item := range info {
		if len(item.Property) > maxLength {
			maxLength = len(item.Property)
		}
	}

	var output strings.Builder
	output.WriteString("Device Information:\n")
	output.WriteString(strings.Repeat("=", 20) + "\n")
	for _, item := range info {
		output.WriteString(fmt.Sprintf("%-*s : %s\n", maxLength, item.Property, item.Value))
	}
	return output.String()
}

func main() {
	devices := getConnectedDevices()
	selectedDevice := selectDevice(devices)

	fmt.Printf("Checking connection to device %s...\n", selectedDevice)
	err := checkDeviceConnectivity(selectedDevice, 5*time.Second)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println("Please ensure the device is properly connected and try again.")
		os.Exit(1)
	}

	fmt.Printf("Fetching device information for %s...\n", selectedDevice)
	info := getDeviceInfo(selectedDevice)
	fmt.Print(formatOutput(info))
}
