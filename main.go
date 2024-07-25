package main

import (
	"fmt"
	"os/exec"
	"strings"
)

type DeviceInfo struct {
	Property string
	Value    string
}

func runAdbCommand(command string) string {
	cmd := exec.Command("adb", strings.Split(command, " ")...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return strings.TrimSpace(string(output))
}

func getDeviceInfo() []DeviceInfo {
	return []DeviceInfo{
		{"Model", runAdbCommand("shell getprop ro.product.model")},
		{"Android Version", runAdbCommand("shell getprop ro.build.version.release")},
		{"API Level", runAdbCommand("shell getprop ro.build.version.sdk")},
		{"CPU ABI", runAdbCommand("shell getprop ro.product.cpu.abi")},
		{"Manufacturer", runAdbCommand("shell getprop ro.product.manufacturer")},
		{"Build Number", runAdbCommand("shell getprop ro.build.display.id")},
		{"Total RAM", runAdbCommand("shell cat /proc/meminfo | grep MemTotal")},
		{"CPU Cores", runAdbCommand("shell nproc")},
	}
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
	fmt.Println("Fetching device information...")
	info := getDeviceInfo()
	fmt.Print(formatOutput(info))
}
