package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/proxy"
)

func getIPRange(startIP, endIP string) []string {
	start := ipToNumber(startIP)
	end := ipToNumber(endIP)

	var ips []string
	for current := start; current <= end; current++ {
		ip := numberToIP(current)
		ips = append(ips, ip)
	}

	return ips
}

func ipToNumber(ip string) uint32 {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return 0
	}

	var num uint32
	for i := 0; i < 4; i++ {
		p, err := strconv.Atoi(parts[i])
		if err != nil || p < 0 || p > 255 {
			return 0
		}
		num = (num << 8) + uint32(p)
	}

	return num
}

func numberToIP(num uint32) string {
	ip := make([]string, 4)
	for i := 3; i >= 0; i-- {
		ip[i] = strconv.Itoa(int(num & 0xFF))
		num >>= 8
	}
	return strings.Join(ip, ".")
}

func main() {
	fmt.Println("starting ...")
	createDir()
	var startIP, endIP string

	fmt.Print("Enter the start IP: ")
	fmt.Scan(&startIP)

	fmt.Print("Enter the end IP: ")
	fmt.Scan(&endIP)
	ips := getIPRange(startIP, endIP)
	taskChan := make(chan string, len(ips))
	const numWorkers = 75
	for i := 0; i < numWorkers; i++ {
		go worker(taskChan)
	}
	for _, ip := range ips {
		taskChan <- ip
	}
	close(taskChan)
	time.Sleep(120 * time.Second)
}

func worker(taskChan <-chan string) {
	for ip := range taskChan {
		port := rand.Intn(9001-1000) + 1000
		fileName := config2tmp(ip, port)
		configTest(ip, fileName, port)
	}
}

func createDir() {
	dirPath := "tmpconfig"
	permissions := os.FileMode(0755)
	err := os.MkdirAll(dirPath, permissions)
	if err != nil {
		panic(err)
	}
	// println("Directory created:", dirPath)
}
func str2int(str string) int {
	num, err := strconv.Atoi(str)
	if err != nil {
		fmt.Println("Error:", err)
		return 255
	}
	return num
}
func config2tmp(ip string, port int) string {
	config := readFile()
	replaced_ip := strings.ReplaceAll(config, "<IP>", ip)
	replaced_port := strings.ReplaceAll(replaced_ip, "<PORT>", strconv.Itoa(port))
	file_name := "tmpconfig/" + randomString(5) + ".json"
	CreateFile(file_name, replaced_port)
	return file_name
}
func readFile() string {
	filePath := "config.json"
	fileContents, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println("Error:", err)
		return ""
	}
	fileString := string(fileContents)
	return fileString
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create a byte slice with the specified length
	randomBytes := make([]byte, length)
	for i := range randomBytes {
		randomBytes[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(randomBytes)
}
func CreateFile(file_name string, data string) {
	fileName := file_name
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer file.Close() // Close the file at the end of the function
	_, err = file.WriteString(data)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

}

func runV2Ray(file_name string) *exec.Cmd {
	cmd := exec.Command("v2ray", "run", "-config", file_name)
	err := cmd.Start()
	if err != nil {
		fmt.Println("Error starting v2ray:", err)
		time.Sleep(5 * time.Second)
		return nil
	}
	return cmd
}

func gracefulTerminate(cmd *exec.Cmd) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	err := cmd.Process.Kill()
	if err != nil {
		fmt.Println("Error forcefully terminating the command:", err)
	}
	// select {
	// case <-signals:
	// 	fmt.Println("Received termination signal. Terminating the command...")
	// 	err := cmd.Process.Signal(syscall.SIGTERM)
	// 	if err != nil {
	// 		fmt.Println("Error terminating the command:", err)
	// 	}

	// 	err = cmd.Wait()
	// 	if err != nil {
	// 		fmt.Println("Error waiting for the command to exit:", err)
	// 	}
	// case <-time.After(0.5 * time.Second): // Timeout to avoid blocking indefinitely
	// 	fmt.Println("Termination timeout exceeded. Forcefully terminating the command...")
	// 	err := cmd.Process.Kill()
	// 	if err != nil {
	// 		fmt.Println("Error forcefully terminating the command:", err)
	// 	}
	// }
}

func configTest(ip string, file_name string, port int) {
	v2rayCmd := runV2Ray(file_name)
	if v2rayCmd == nil {
		return
	}
	time.Sleep(3 * time.Second)
	proxyURL := "127.0.0.1:" + strconv.Itoa(port)
	dialer, err := proxy.SOCKS5("tcp", proxyURL, nil, proxy.Direct)
	if err != nil {
		return
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			Dial: dialer.Dial,
		},
		Timeout: 1 * time.Second,
	}
	resp, err := httpClient.Get("https://ifconfig.me")
	if err != nil {
		fmt.Println("faild:", ip)
		os.Remove(file_name) //remove the file
		return
	}
	defer resp.Body.Close()
	appendToFile("result.txt", ip)
	fmt.Println("success:", ip)
	gracefulTerminate(v2rayCmd)
	os.Remove(file_name) //remove the file
}
func appendToFile(filename, content string) {
	file, _ := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	file.WriteString(content + "\n")
}
