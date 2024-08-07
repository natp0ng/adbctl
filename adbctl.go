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
var showIcons bool

func init() {
	isDebug = os.Getenv("DEBUG") != ""
	//showIcons = os.Getenv("SHOW_ICONS") != "false"
	showIcons = false
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
	cmd := exec.Command("adb", "devices", "-l")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error running adb devices:", err)
		os.Exit(1)
	}

	lines := strings.Split(string(output), "\n")
	var devices []string
	for _, line := range lines[1:] { // Skip the first line (header)
		if strings.TrimSpace(line) != "" && !strings.HasSuffix(line, "offline") {
			deviceInfo := strings.Fields(line)
			if len(deviceInfo) > 0 {
				devices = append(devices, line)
			}
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
		return strings.Fields(devices[0])[0]
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
			return strings.Fields(devices[index-1])[0]
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

func parseStorageInfo(dfOutput string) string {
	lines := strings.Split(dfOutput, "\n")
	if len(lines) < 2 {
		return "n/a"
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return "n/a"
	}

	totalKB, _ := strconv.Atoi(fields[1])
	usedKB, _ := strconv.Atoi(fields[2])
	freeKB, _ := strconv.Atoi(fields[3])

	totalGB := float64(totalKB) / 1048576.0
	usedGB := float64(usedKB) / 1048576.0
	freeGB := float64(freeKB) / 1048576.0

	return fmt.Sprintf("%.2f GB / %d kB (%.2f GB used, %.2f GB free)", totalGB, totalKB, usedGB, freeGB)
}

func mapCPUABI(abi string) string {
	mapping := map[string]string{
		"armeabi":     "ARM EABI (32-bit)",
		"armeabi-v7a": "ARM EABI v7a (32-bit, with hardware floating-point support)",
		"arm64-v8a":   "ARM 64-bit (v8a)",
		"x86":         "Intel x86 (32-bit)",
		"x86_64":      "Intel x86_64 (64-bit)",
		"mips":        "MIPS (32-bit)",
		"mips64":      "MIPS 64-bit",
	}

	if humanReadable, ok := mapping[abi]; ok {
		return humanReadable
	}
	return abi
}

func mapFireOSModel(model string) string {
	mapping := map[string]struct {
		Name string
		Link string
	}{
		"AFTTOR001":   {"Panasonic OLED TV VIERA with Fire TV integration (2024)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=panasonic_fire_tv_2024_jp"},
		"AFTWYM01":    {"Panasonic OLED TV VIERA with Fire TV integration (2024)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=panasonic_fire_tv_2024_jp"},
		"AFTGOLDFF":   {"Panasonic Fire TV (2024)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv-emea.html?v=ftvedition_panasonic4k"},
		"AFTDEC012E":  {"Fire TV - TCL S4/S5/Q5/Q6 Series 4K UHD HDR LED (2024)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=tcl_s4s5q5q6_2024"},
		"AFTBTX4":     {"Redmi 108cm (43 inches) 4K Ultra HD smart LED Fire TV (2023)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=redmi_108_f_4k_uhd_2023"},
		"AFTMD002":    {"TCL Class S3 1080p LED Smart TV with Fire TV (2023)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=tclclass_s3_1080_2023"},
		"AFTKRT":      {"Fire TV Stick 4K Max - 2nd Gen (2023) - 16 GB", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-stick.html?v=ftvstick4kmax_gen2_16"},
		"AFTKM":       {"Fire TV Stick 4K - 2nd Gen (2023) - 8 GB", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-stick.html?v=ftvstick4k_gen2_8"},
		"AFTSHN02":    {"TCL 32\" FHD, 40\" FHD Fire TV (2023)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=tclsmart_fhd__led_2023"},
		"AFTMD001":    {"Fire TV - TCL S4 Series 4K UHD HDR LED (2023)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=tclsseries_4K_2023"},
		"AFTKA002":    {"Fire TV 2-Series (2023)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=2series2023"},
		"AFTKAUK002":  {"Fire TV 2-Series (2023)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=2series2023"},
		"AFTHA004":    {"Toshiba 4K UHD - Fire TV (2022)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=toshiba4k2022"},
		"AFTLBT962E2": {"BMW (2022)", "https://developer.amazon.com/docs/fire-tv/device-specifications-automotive.html?v=BMW2022"},
		"AEOHY":       {"Echo Show 15 (2021)", "https://developer.amazon.com/docs/fire-tv/device-specifications-echo-show.html?v=echoshow2021"},
		"AFTTIFF43":   {"Fire TV Omni QLED Series (2022)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=omniseries2"},
		"AFTGAZL":     {"Fire TV Cube - 3rd Gen (2022)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-cube.html?v=ftvcubegen3"},
		"AFTANNA0":    {"Xiaomi F2 4K - Fire TV (2022)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=firetvedition_xiaomi2022"},
		"AFTHA001":    {"Hisense U6 4K UHD - Fire TV (2022)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=firetvedition_hisense4k"},
		"AFTMON001":   {"Funai 4K - Fire TV (2022)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=firetvedition_funai4k2022"},
		"AFTMON002":   {"Funai 4K - Fire TV (2022)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=firetvedition_funai4k2022"},
		"AFTJULI1":    {"JVC 4K - Fire TV with Freeview Play (2021)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=firetvedition_jvc4kfp"},
		"AFTWMST22":   {"JVC 2K - Fire TV (2020)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=firetveditionuk_jvc2"},
		"AFTTIFF55":   {"Onida HD/FHD - Fire TV (2020)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=ftveditionin_onidahd2020"},
		"AFTWI001":    {"ok 4K - Fire TV (2020)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=ftveditionde_ok4k"},
		"AFTSSS":      {"Fire TV Stick - 3rd Gen (2020)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-stick.html?v=ftvstickgen3"},
		"AFTSS":       {"Fire TV Stick Lite - 1st Gen (2020)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-stick.html?v=ftvsticklite"},
		"AFTDCT31":    {"Toshiba 4K UHD - Fire TV (2020)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=ftveditiontoshiba4k_2020"},
		"AFTPR001":    {"AmazonBasics 4K - Fire TV (2020)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=ftveditionin_amazonbasics4k"},
		"AFTBU001":    {"AmazonBasics HD/FHD - Fire TV (2020)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=ftveditionin_amazonbasics2k"},
		"AFTLE":       {"Onida HD - Fire TV (2019)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=ftveditionin_onidahd"},
		"AFTR":        {"Fire TV Cube - 2nd Gen (2019)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-cube.html?v=ftvcubegen2"},
		"AFTEUFF014":  {"Grundig OLED 4K - Fire TV (2019)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=ftveditionde_grundigoled"},
		"AFTEU014":    {"Grundig Vision 7, 4K - Fire TV (2019)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=ftveditionde_grundigvision7"},
		"AFTSO001":    {"JVC 4K - Fire TV (2019)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=ftveditionuk_jvc4k"},
		// "AFTMM":       {"Nebula Soundbar - Fire TV Edition (2019)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-soundbar.html?v=ftvedition_nebula"},
		"AFTEU011":  {"Grundig Vision 6 HD - Fire TV (2019)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=ftveditionde_grundigvision6"},
		"AFTJMST12": {"Insignia 4K - Fire TV (2018)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=ftveditioninsignia4k"},
		"AFTA":      {"Fire TV Cube - 1st Gen (2018)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-cube.html?v=ftvcubegen1"},
		"AFTMM":     {"Fire TV Stick 4K - 1st Gen (2018)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-stick.html?v=ftvstick4k"},
		"AFTT":      {"Fire TV Stick - Basic Edition (2017)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-stick.html?v=ftvstickbasicedition"},
		"AFTRS":     {"Element 4K - Fire TV (2017)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=ftveditionelement"},
		"AFTN":      {"Fire TV - 3rd Gen (2017)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-pendant-box.html?v=ftvgen3"},
		"AFTS":      {"Fire TV - 2nd Gen (2015)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-pendant-box.html?v=ftvgen2"},
		"AFTM":      {"Fire TV Stick - 1st Gen (2014)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-stick.html?v=ftvstickgen1"},
		"AFTB":      {"Fire TV - 1st Gen (2014)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-pendant-box.html?v=ftvgen1"},
		// "AFTMM":       {"TCL Soundbar with Built-in Subwoofer - Fire TV Edition (2019)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-soundbar.html?v=ftvedition_tcl"},
		"AFTHA002": {"Toshiba V35 Series LED FHD/HD - Fire TV (2021)", "https://developer.amazon.com/docs/fire-tv/device-specifications-fire-tv-edition-smart-tv.html?v=firetvedition_toshibav35"},
	}

	if realName, ok := mapping[model]; ok {
		return fmt.Sprintf("%s (%s)", realName.Name, realName.Link)
	}
	return model
}

func getDeviceInfo(deviceID string) []DeviceInfo {
	timeout := 5 * time.Second
	info := []DeviceInfo{
		{"Model", mapFireOSModel(runAdbCommand(deviceID, "getprop ro.product.model", timeout))},
		{"Android Version", runAdbCommand(deviceID, "getprop ro.build.version.release", timeout)},
		{"API Level", runAdbCommand(deviceID, "getprop ro.build.version.sdk", timeout)},
		{"CPU ABI", mapCPUABI(runAdbCommand(deviceID, "getprop ro.product.cpu.abi", timeout))},
		{"Manufacturer", runAdbCommand(deviceID, "getprop ro.product.manufacturer", timeout)},
		{"Build Number", runAdbCommand(deviceID, "getprop ro.build.display.id", timeout)},
		{"Memory", parseMemInfo(runAdbCommand(deviceID, "cat /proc/meminfo", timeout))},
		{"CPU", parseCPUInfo(runAdbCommand(deviceID, "cat /proc/cpuinfo", timeout), runAdbCommand(deviceID, "top -n 1 | grep 'CPU:'", timeout))},
		{"Storage", parseStorageInfo(runAdbCommand(deviceID, "df -k /data", timeout))},
		{"Screen Resolution", runAdbCommand(deviceID, "wm size", timeout)},
		{"Screen Density", runAdbCommand(deviceID, "wm density", timeout)},
		{"Battery Level", runAdbCommand(deviceID, "dumpsys battery | grep level | awk '{print $2}'", timeout)},
		{"Fire OS Version", runAdbCommand(deviceID, "getprop ro.build.version.name", timeout)},
		{"Fire OS Build Number", runAdbCommand(deviceID, "getprop ro.build.version.number", timeout)},
		{"IP Address", runAdbCommand(deviceID, "ip addr show wlan0 | grep 'inet ' | awk '{print $2}' | cut -d/ -f1", timeout)},
		{"WiFi SSID", runAdbCommand(deviceID, "dumpsys wifi | grep 'mWifiInfo' | grep -o 'SSID:.*' | awk -F', ' '{print $1}' | sed 's/SSID: //'", timeout)},
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
			"IP Address", "WiFi SSID",
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
	if !showIcons {
		return "  "
	}
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

func main() {
	fmt.Println("Welcome to abdctl - Your Android Device Management Companion")
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
