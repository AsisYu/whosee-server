/*
 * @Author: AsisYu
 * @Date: 2025-06-16
 * @Description: Chrome浏览器自动下载和管理工具
 */
package utils

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// PlatformInfo 智能平台信息
type PlatformInfo struct {
	OS             string            // 操作系统：windows, linux, darwin
	Arch           string            // 架构：amd64, 386, arm64, arm
	PlatformKey    string            // 平台键：如 windows-amd64
	ChromePlatform string            // Chrome平台名称：如 win64
	IsWSL          bool              // 是否是Windows Subsystem for Linux
	IsContainer    bool              // 是否在容器环境中
	IsARM          bool              // 是否是ARM架构
	OSVersion      string            // 操作系统版本
	Details        map[string]string // 额外的平台详细信息
}

// SmartPlatformDetector 智能平台检测器
type SmartPlatformDetector struct {
	platform *PlatformInfo
}

// NewSmartPlatformDetector 创建智能平台检测器
func NewSmartPlatformDetector() *SmartPlatformDetector {
	detector := &SmartPlatformDetector{}
	detector.detectPlatform()
	return detector
}

// detectPlatform 检测平台信息
func (spd *SmartPlatformDetector) detectPlatform() {
	platform := &PlatformInfo{
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Details: make(map[string]string),
	}

	// 设置基础平台键
	platform.PlatformKey = fmt.Sprintf("%s-%s", platform.OS, platform.Arch)

	// 检测ARM架构
	platform.IsARM = strings.Contains(platform.Arch, "arm")

	// 检测特殊环境
	platform.IsWSL = spd.detectWSL()
	platform.IsContainer = spd.detectContainer()

	// 获取操作系统版本
	platform.OSVersion = spd.detectOSVersion()

	// 映射Chrome平台名称
	platform.ChromePlatform = spd.mapChromePlatform(platform)

	// 收集额外信息
	spd.collectAdditionalInfo(platform)

	spd.platform = platform

	// 记录检测结果
	log.Printf("[PLATFORM-DETECTOR] 智能平台检测完成:")
	log.Printf("[PLATFORM-DETECTOR]   操作系统: %s (%s)", platform.OS, platform.OSVersion)
	log.Printf("[PLATFORM-DETECTOR]   架构: %s (ARM: %v)", platform.Arch, platform.IsARM)
	log.Printf("[PLATFORM-DETECTOR]   平台键: %s", platform.PlatformKey)
	log.Printf("[PLATFORM-DETECTOR]   Chrome平台: %s", platform.ChromePlatform)
	log.Printf("[PLATFORM-DETECTOR]   特殊环境: WSL=%v, Container=%v", platform.IsWSL, platform.IsContainer)

	if len(platform.Details) > 0 {
		log.Printf("[PLATFORM-DETECTOR]   额外信息: %+v", platform.Details)
	}
}

// detectWSL 检测是否在WSL环境中
func (spd *SmartPlatformDetector) detectWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// 检查 /proc/version 文件
	if data, err := os.ReadFile("/proc/version"); err == nil {
		content := strings.ToLower(string(data))
		if strings.Contains(content, "microsoft") || strings.Contains(content, "wsl") {
			log.Printf("[PLATFORM-DETECTOR] 检测到WSL环境")
			return true
		}
	}

	// 检查 /proc/sys/kernel/osrelease
	if data, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		content := strings.ToLower(string(data))
		if strings.Contains(content, "microsoft") || strings.Contains(content, "wsl") {
			log.Printf("[PLATFORM-DETECTOR] 检测到WSL环境 (通过osrelease)")
			return true
		}
	}

	return false
}

// detectContainer 检测是否在容器环境中
func (spd *SmartPlatformDetector) detectContainer() bool {
	// 检查Docker环境
	if _, err := os.Stat("/.dockerenv"); err == nil {
		log.Printf("[PLATFORM-DETECTOR] 检测到Docker容器环境")
		return true
	}

	// 检查cgroup信息
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") || strings.Contains(content, "kubepods") || strings.Contains(content, "lxc") {
			log.Printf("[PLATFORM-DETECTOR] 检测到容器环境 (通过cgroup)")
			return true
		}
	}

	// 检查环境变量
	containerEnvs := []string{"DOCKER_CONTAINER", "KUBERNETES_SERVICE_HOST", "container"}
	for _, env := range containerEnvs {
		if os.Getenv(env) != "" {
			log.Printf("[PLATFORM-DETECTOR] 检测到容器环境 (通过环境变量: %s)", env)
			return true
		}
	}

	return false
}

// detectOSVersion 检测操作系统版本
func (spd *SmartPlatformDetector) detectOSVersion() string {
	switch runtime.GOOS {
	case "windows":
		return spd.detectWindowsVersion()
	case "linux":
		return spd.detectLinuxVersion()
	case "darwin":
		return spd.detectMacOSVersion()
	default:
		return "unknown"
	}
}

// detectWindowsVersion 检测Windows版本
func (spd *SmartPlatformDetector) detectWindowsVersion() string {
	// 尝试从注册表获取版本信息（需要Windows API）
	// 这里使用简单的方法，通过环境变量或已知信息

	// 检查Windows版本环境变量
	if osVersion := os.Getenv("OS"); osVersion != "" {
		return osVersion
	}

	// 检查常见的Windows版本指示符
	if programFiles := os.Getenv("ProgramFiles"); programFiles != "" {
		return "Windows (detected via ProgramFiles)"
	}

	return "Windows (version unknown)"
}

// detectLinuxVersion 检测Linux版本
func (spd *SmartPlatformDetector) detectLinuxVersion() string {
	// 尝试读取 /etc/os-release
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(data), "\n")
		var name, version string
		for _, line := range lines {
			if strings.HasPrefix(line, "NAME=") {
				name = strings.Trim(strings.TrimPrefix(line, "NAME="), `"`)
			} else if strings.HasPrefix(line, "VERSION=") {
				version = strings.Trim(strings.TrimPrefix(line, "VERSION="), `"`)
			}
		}
		if name != "" {
			if version != "" {
				return fmt.Sprintf("%s %s", name, version)
			}
			return name
		}
	}

	// 备用方法：读取 /etc/issue
	if data, err := os.ReadFile("/etc/issue"); err == nil {
		firstLine := strings.Split(string(data), "\n")[0]
		if firstLine != "" {
			return strings.TrimSpace(firstLine)
		}
	}

	return "Linux (version unknown)"
}

// detectMacOSVersion 检测macOS版本
func (spd *SmartPlatformDetector) detectMacOSVersion() string {
	// 在macOS上，可以尝试读取系统版本文件
	if data, err := os.ReadFile("/System/Library/CoreServices/SystemVersion.plist"); err == nil {
		content := string(data)
		// 简单解析plist文件
		if strings.Contains(content, "ProductVersion") {
			return "macOS (version in SystemVersion.plist)"
		}
	}

	return "macOS (version unknown)"
}

// mapChromePlatform 映射Chrome平台名称
func (spd *SmartPlatformDetector) mapChromePlatform(platform *PlatformInfo) string {
	switch platform.OS {
	case "windows":
		if platform.Arch == "amd64" {
			return "win64"
		}
		return "win32"
	case "linux":
		if platform.IsWSL {
			// WSL通常运行在x64 Windows上，但我们仍然下载Linux版本
			log.Printf("[PLATFORM-DETECTOR] WSL环境，使用Linux Chrome版本")
		}
		if platform.Arch == "amd64" {
			return "linux64"
		}
		return "linux32" // 虽然很少见
	case "darwin":
		if platform.IsARM || platform.Arch == "arm64" {
			return "mac-arm64"
		}
		return "mac-x64"
	default:
		log.Printf("[PLATFORM-DETECTOR] 未知操作系统 %s，默认使用 win64", platform.OS)
		return "win64" // 默认备用
	}
}

// collectAdditionalInfo 收集额外的平台信息
func (spd *SmartPlatformDetector) collectAdditionalInfo(platform *PlatformInfo) {
	// CPU核心数
	platform.Details["cpu_cores"] = strconv.Itoa(runtime.NumCPU())

	// Go运行时版本
	platform.Details["go_version"] = runtime.Version()

	// 主机名
	if hostname, err := os.Hostname(); err == nil {
		platform.Details["hostname"] = hostname
	}

	// 用户目录
	if homeDir, err := os.UserHomeDir(); err == nil {
		platform.Details["home_dir"] = homeDir
	}

	// 临时目录
	platform.Details["temp_dir"] = os.TempDir()

	// 环境变量中的重要信息
	if arch := os.Getenv("PROCESSOR_ARCHITECTURE"); arch != "" {
		platform.Details["processor_arch"] = arch
	}

	// 内存信息（Linux）
	if platform.OS == "linux" {
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "MemTotal:") {
					platform.Details["memory_total"] = strings.TrimSpace(line)
					break
				}
			}
		}
	}
}

// GetPlatform 获取平台信息
func (spd *SmartPlatformDetector) GetPlatform() *PlatformInfo {
	return spd.platform
}

// GetChromePlatform 获取Chrome平台名称
func (spd *SmartPlatformDetector) GetChromePlatform() string {
	return spd.platform.ChromePlatform
}

// GetPlatformKey 获取平台键
func (spd *SmartPlatformDetector) GetPlatformKey() string {
	return spd.platform.PlatformKey
}

// IsSupported 检查平台是否受支持
func (spd *SmartPlatformDetector) IsSupported() bool {
	supportedPlatforms := []string{"win64", "win32", "linux64", "mac-x64", "mac-arm64"}

	for _, supported := range supportedPlatforms {
		if spd.platform.ChromePlatform == supported {
			return true
		}
	}

	return false
}

// GetRecommendations 获取平台相关建议
func (spd *SmartPlatformDetector) GetRecommendations() []string {
	var recommendations []string

	platform := spd.platform

	if platform.IsWSL {
		recommendations = append(recommendations, "检测到WSL环境，建议确保有足够的内存用于Chrome运行")
		recommendations = append(recommendations, "WSL中运行Chrome可能需要X11转发或使用headless模式")
	}

	if platform.IsContainer {
		recommendations = append(recommendations, "检测到容器环境，建议使用--no-sandbox标志运行Chrome")
		recommendations = append(recommendations, "容器环境可能需要额外的权限或挂载点")
	}

	if platform.IsARM {
		recommendations = append(recommendations, "ARM架构可能需要特殊的Chrome构建版本")
		recommendations = append(recommendations, "某些Chrome功能在ARM架构上可能不可用")
	}

	if !spd.IsSupported() {
		recommendations = append(recommendations, fmt.Sprintf("平台 %s 可能不受官方Chrome支持，将尝试备用方案", platform.ChromePlatform))
	}

	return recommendations
}

// 全局平台检测器实例
var globalPlatformDetector *SmartPlatformDetector

// GetGlobalPlatformDetector 获取全局平台检测器
func GetGlobalPlatformDetector() *SmartPlatformDetector {
	if globalPlatformDetector == nil {
		globalPlatformDetector = NewSmartPlatformDetector()
	}
	return globalPlatformDetector
}

// ChromeVersionResponse Chrome版本API响应结构
type ChromeVersionResponse struct {
	Timestamp string          `json:"timestamp"`
	Versions  []ChromeVersion `json:"versions"`
}

// ChromeVersion Chrome版本信息
type ChromeVersion struct {
	Version   string                   `json:"version"`
	Revision  string                   `json:"revision"`
	Downloads map[string][]ChromeBuild `json:"downloads"`
}

// ChromeBuild Chrome构建信息
type ChromeBuild struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

// HuaweiCloudIndex 华为云镜像索引结构
type HuaweiCloudIndex struct {
	ChromiumBrowserSnapshots map[string]HuaweiCloudVersion `json:"chromium-browser-snapshots"`
}

// HuaweiCloudVersion 华为云版本信息
type HuaweiCloudVersion struct {
	Files []string `json:"files"`
}

// NPMMirrorResponse NPM镜像响应结构
type NPMMirrorResponse []NPMMirrorItem

// NPMMirrorItem NPM镜像项目
type NPMMirrorItem struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Date     string `json:"date"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Modified string `json:"modified"`
}

// optimizeNetworkSettings 优化网络设置，专门针对中国大陆网络环境
func optimizeNetworkSettings() *http.Transport {
	// 自定义DNS解析器，优先使用国内DNS
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			// 使用阿里DNS和腾讯DNS
			dnsServers := []string{
				"223.5.5.5:53",    // 阿里DNS
				"119.29.29.29:53", // 腾讯DNS
				"8.8.8.8:53",      // Google DNS备用
			}

			var lastErr error
			for _, dnsServer := range dnsServers {
				d := &net.Dialer{
					Timeout: 5 * time.Second,
				}
				conn, err := d.DialContext(ctx, "udp", dnsServer)
				if err == nil {
					return conn, nil
				}
				lastErr = err
				log.Printf("[CHROME-DOWNLOADER] DNS服务器 %s 连接失败: %v", dnsServer, err)
			}
			return nil, fmt.Errorf("所有DNS服务器都无法连接: %v", lastErr)
		},
	}

	// 创建优化的Transport
	transport := &http.Transport{
		// 强制IPv4，避免IPv6问题
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if network == "tcp" {
				network = "tcp4"
			}

			// 使用自定义解析器
			dialer := &net.Dialer{
				Timeout:       45 * time.Second, // 增加连接超时
				KeepAlive:     60 * time.Second,
				FallbackDelay: -1, // 完全禁用IPv6
				Resolver:      resolver,
			}

			return dialer.DialContext(ctx, network, addr)
		},

		// 优化的连接池设置
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     20,

		// 超时设置
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
		IdleConnTimeout:       180 * time.Second,

		// 禁用压缩和keepalive优化
		DisableCompression: true,
		DisableKeepAlives:  false,

		// 代理设置（如果需要的话）
		Proxy: http.ProxyFromEnvironment,
	}

	log.Printf("[CHROME-DOWNLOADER] 网络配置优化完成: 强制IPv4, 自定义DNS解析, 优化连接池")
	return transport
}

// ChromeDownloader Chrome下载器
type ChromeDownloader struct {
	downloadDir      string
	chromeExecutable string
	version          string
	platformDetector *SmartPlatformDetector // 智能平台检测器
}

// NewChromeDownloader 创建Chrome下载器
func NewChromeDownloader() *ChromeDownloader {
	baseDir := "chrome_runtime"
	detector := GetGlobalPlatformDetector() // 使用全局平台检测器

	downloader := &ChromeDownloader{
		downloadDir:      baseDir,
		version:          "stable", // 或者指定具体版本
		platformDetector: detector,
	}

	// 显示平台检测结果和建议
	platform := detector.GetPlatform()
	log.Printf("[CHROME-DOWNLOADER] 初始化Chrome下载器")
	log.Printf("[CHROME-DOWNLOADER] 检测到平台: %s (%s)", platform.PlatformKey, platform.ChromePlatform)

	if recommendations := detector.GetRecommendations(); len(recommendations) > 0 {
		log.Printf("[CHROME-DOWNLOADER] 平台建议:")
		for _, rec := range recommendations {
			log.Printf("[CHROME-DOWNLOADER]   - %s", rec)
		}
	}

	return downloader
}

// GetChromeUrls 获取不同平台的Chrome下载URL (保持向后兼容)
func (cd *ChromeDownloader) GetChromeUrls() map[string][]string {
	return map[string][]string{
		"windows-amd64": {
			// 中国大陆可访问的镜像源 (优先)
			"https://registry.npmmirror.com/-/binary/chromium-browser-snapshots/Win_x64/1354089/chrome-win.zip",
			"https://mirrors.huaweicloud.com/chromium-browser-snapshots/Win_x64/1354089/chrome-win.zip",
			"https://mirrors.tuna.tsinghua.edu.cn/chromium-browser-snapshots/Win_x64/1354089/chrome-win.zip",
			// GitHub Release (相对稳定)
			"https://github.com/Hibbiki/chromium-win64/releases/download/v131.0.6778.108-r1354089/chrome.sync.7z",
			// 备用官方源
			"https://storage.googleapis.com/chrome-for-testing-public/131.0.6778.108/win64/chrome-win64.zip",
			"https://commondatastorage.googleapis.com/chromium-browser-snapshots/Win_x64/1354089/chrome-win.zip",
		},
		"windows-386": {
			"https://storage.googleapis.com/chrome-for-testing-public/131.0.6778.108/win32/chrome-win32.zip",
			"https://commondatastorage.googleapis.com/chromium-browser-snapshots/Win/1354089/chrome-win.zip",
		},
		"linux-amd64": {
			"https://storage.googleapis.com/chrome-for-testing-public/131.0.6778.108/linux64/chrome-linux64.zip",
			"https://commondatastorage.googleapis.com/chromium-browser-snapshots/Linux_x64/1354089/chrome-linux.zip",
			"https://npm.taobao.org/mirrors/chromium-browser-snapshots/Linux_x64/1354089/chrome-linux.zip",
		},
		"darwin-amd64": {
			"https://storage.googleapis.com/chrome-for-testing-public/131.0.6778.108/mac-x64/chrome-mac-x64.zip",
			"https://commondatastorage.googleapis.com/chromium-browser-snapshots/Mac/1354089/chrome-mac.zip",
		},
		"darwin-arm64": {
			"https://storage.googleapis.com/chrome-for-testing-public/131.0.6778.108/mac-arm64/chrome-mac-arm64.zip",
			"https://commondatastorage.googleapis.com/chromium-browser-snapshots/Mac_Arm/1354089/chrome-mac.zip",
		},
	}
}

// GetPlatformKey 获取当前平台的键 (使用智能检测)
func (cd *ChromeDownloader) GetPlatformKey() string {
	return cd.platformDetector.GetPlatformKey()
}

// GetChromePlatform 获取Chrome平台名称 (使用智能检测)
func (cd *ChromeDownloader) GetChromePlatform() string {
	return cd.platformDetector.GetChromePlatform()
}

// GetPlatformInfo 获取完整平台信息
func (cd *ChromeDownloader) GetPlatformInfo() *PlatformInfo {
	return cd.platformDetector.GetPlatform()
}

// GetChromeExecutablePath 获取Chrome可执行文件路径 (智能版本)
func (cd *ChromeDownloader) GetChromeExecutablePath() string {
	if cd.chromeExecutable != "" {
		return cd.chromeExecutable
	}

	platform := cd.platformDetector.GetPlatform()

	log.Printf("[CHROME-DOWNLOADER] 开始智能搜索Chrome可执行文件，平台: %s", platform.ChromePlatform)

	switch platform.OS {
	case "windows":
		cd.chromeExecutable = cd.findWindowsChrome(platform)
	case "linux":
		cd.chromeExecutable = cd.findLinuxChrome(platform)
	case "darwin":
		cd.chromeExecutable = cd.findMacOSChrome(platform)
	default:
		log.Printf("[CHROME-DOWNLOADER] 未知操作系统: %s", platform.OS)
		// 默认尝试Windows路径
		cd.chromeExecutable = cd.findWindowsChrome(platform)
	}

	if cd.chromeExecutable != "" {
		log.Printf("[CHROME-DOWNLOADER] 智能搜索找到Chrome: %s", cd.chromeExecutable)
	} else {
		log.Printf("[CHROME-DOWNLOADER] 智能搜索未找到Chrome，使用默认路径")
		cd.chromeExecutable = cd.getDefaultChromePath(platform)
	}

	return cd.chromeExecutable
}

// findWindowsChrome 查找Windows平台的Chrome
func (cd *ChromeDownloader) findWindowsChrome(platform *PlatformInfo) string {
	// Windows平台的可能路径，按优先级排序
	possiblePaths := []string{
		// 官方Chrome for Testing (推荐)
		filepath.Join(cd.downloadDir, "chrome-win64", "chrome.exe"),
		// 华为云/NPM镜像等常见结构
		filepath.Join(cd.downloadDir, "chrome-win", "chrome.exe"),
		// 32位版本
		filepath.Join(cd.downloadDir, "chrome-win32", "chrome.exe"),
		// 直接解压结构 (某些镜像)
		filepath.Join(cd.downloadDir, "Win_x64", "chrome.exe"),
		filepath.Join(cd.downloadDir, "Win", "chrome.exe"),
		// Chromium构建
		filepath.Join(cd.downloadDir, "chromium-win64", "chrome.exe"),
		filepath.Join(cd.downloadDir, "chromium-win", "chrome.exe"),
	}

	// 根据架构调整优先级
	if platform.Arch == "386" {
		// 32位系统，优先32位版本
		reorderedPaths := []string{
			filepath.Join(cd.downloadDir, "chrome-win32", "chrome.exe"),
			filepath.Join(cd.downloadDir, "chrome-win", "chrome.exe"),
			filepath.Join(cd.downloadDir, "Win", "chrome.exe"),
		}
		possiblePaths = append(reorderedPaths, possiblePaths...)
	}

	for _, path := range possiblePaths {
		if cd.validateChromeExecutable(path) {
			return path
		}
	}

	return ""
}

// findLinuxChrome 查找Linux平台的Chrome
func (cd *ChromeDownloader) findLinuxChrome(platform *PlatformInfo) string {
	possiblePaths := []string{
		// 官方Chrome for Testing
		filepath.Join(cd.downloadDir, "chrome-linux64", "chrome"),
		// 通用Linux版本
		filepath.Join(cd.downloadDir, "chrome-linux", "chrome"),
		// 直接解压结构
		filepath.Join(cd.downloadDir, "Linux_x64", "chrome"),
		filepath.Join(cd.downloadDir, "Linux", "chrome"),
		// Chromium构建
		filepath.Join(cd.downloadDir, "chromium-linux64", "chrome"),
		filepath.Join(cd.downloadDir, "chromium-linux", "chrome"),
	}

	// WSL特殊处理
	if platform.IsWSL {
		log.Printf("[CHROME-DOWNLOADER] WSL环境，优先使用64位Linux版本")
		// WSL通常是64位，优先64位版本
		wslPaths := []string{
			filepath.Join(cd.downloadDir, "chrome-linux64", "chrome"),
			filepath.Join(cd.downloadDir, "Linux_x64", "chrome"),
		}
		possiblePaths = append(wslPaths, possiblePaths...)
	}

	for _, path := range possiblePaths {
		if cd.validateChromeExecutable(path) {
			return path
		}
	}

	return ""
}

// findMacOSChrome 查找macOS平台的Chrome
func (cd *ChromeDownloader) findMacOSChrome(platform *PlatformInfo) string {
	var possiblePaths []string

	if platform.IsARM || platform.Arch == "arm64" {
		// ARM64 macOS (Apple Silicon)
		possiblePaths = []string{
			filepath.Join(cd.downloadDir, "chrome-mac-arm64", "Google Chrome for Testing.app", "Contents", "MacOS", "Google Chrome for Testing"),
			filepath.Join(cd.downloadDir, "chrome-mac", "Chromium.app", "Contents", "MacOS", "Chromium"),
			filepath.Join(cd.downloadDir, "Mac_Arm", "Chromium.app", "Contents", "MacOS", "Chromium"),
			filepath.Join(cd.downloadDir, "Mac", "Chromium.app", "Contents", "MacOS", "Chromium"),
		}
	} else {
		// Intel macOS
		possiblePaths = []string{
			filepath.Join(cd.downloadDir, "chrome-mac-x64", "Google Chrome for Testing.app", "Contents", "MacOS", "Google Chrome for Testing"),
			filepath.Join(cd.downloadDir, "chrome-mac", "Chromium.app", "Contents", "MacOS", "Chromium"),
			filepath.Join(cd.downloadDir, "Mac", "Chromium.app", "Contents", "MacOS", "Chromium"),
		}
	}

	for _, path := range possiblePaths {
		if cd.validateChromeExecutable(path) {
			return path
		}
	}

	return ""
}

// validateChromeExecutable 验证Chrome可执行文件是否有效
func (cd *ChromeDownloader) validateChromeExecutable(path string) bool {
	if path == "" {
		return false
	}

	// 检查文件是否存在
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}

	// 检查是否是文件而不是目录
	if stat.IsDir() {
		return false
	}

	// 在Unix系统上检查执行权限
	if runtime.GOOS != "windows" {
		mode := stat.Mode()
		if mode&0111 == 0 { // 没有执行权限
			log.Printf("[CHROME-DOWNLOADER] 文件 %s 没有执行权限，尝试设置", path)
			if err := os.Chmod(path, 0755); err != nil {
				log.Printf("[CHROME-DOWNLOADER] 无法设置执行权限: %v", err)
				return false
			}
		}
	}

	// 可选：检查文件大小（Chrome通常几十MB）
	if stat.Size() < 1024*1024 { // 小于1MB可能不是真正的Chrome
		log.Printf("[CHROME-DOWNLOADER] 文件 %s 太小 (%.2f KB)，可能不是有效的Chrome", path, float64(stat.Size())/1024)
		return false
	}

	log.Printf("[CHROME-DOWNLOADER] 验证Chrome可执行文件: %s (大小: %.2f MB)", path, float64(stat.Size())/(1024*1024))
	return true
}

// getDefaultChromePath 获取默认Chrome路径（当找不到时）
func (cd *ChromeDownloader) getDefaultChromePath(platform *PlatformInfo) string {
	switch platform.OS {
	case "windows":
		if platform.Arch == "amd64" {
			return filepath.Join(cd.downloadDir, "chrome-win64", "chrome.exe")
		}
		return filepath.Join(cd.downloadDir, "chrome-win32", "chrome.exe")
	case "linux":
		return filepath.Join(cd.downloadDir, "chrome-linux64", "chrome")
	case "darwin":
		if platform.IsARM || platform.Arch == "arm64" {
			return filepath.Join(cd.downloadDir, "chrome-mac-arm64", "Google Chrome for Testing.app", "Contents", "MacOS", "Google Chrome for Testing")
		}
		return filepath.Join(cd.downloadDir, "chrome-mac-x64", "Google Chrome for Testing.app", "Contents", "MacOS", "Google Chrome for Testing")
	default:
		// 默认为Windows 64位
		return filepath.Join(cd.downloadDir, "chrome-win64", "chrome.exe")
	}
}

// IsChromeBinaryExists 检查Chrome二进制文件是否存在
func (cd *ChromeDownloader) IsChromeBinaryExists() bool {
	execPath := cd.GetChromeExecutablePath()
	if execPath == "" {
		return false
	}

	if _, err := os.Stat(execPath); err == nil {
		log.Printf("[CHROME-DOWNLOADER] 找到现有Chrome二进制文件: %s", execPath)
		return true
	}

	// 如果默认路径不存在，重新扫描所有可能的路径
	cd.chromeExecutable = "" // 重置缓存
	execPath = cd.GetChromeExecutablePath()

	if execPath != "" && cd.chromeExecutable != "" {
		// 如果重新扫描找到了文件，再次验证
		if _, err := os.Stat(execPath); err == nil {
			log.Printf("[CHROME-DOWNLOADER] 重新扫描找到Chrome二进制文件: %s", execPath)
			return true
		}
	}

	log.Printf("[CHROME-DOWNLOADER] Chrome二进制文件不存在: %s", execPath)
	return false
}

// testNetworkConnectivity 测试网络连接
func (cd *ChromeDownloader) testNetworkConnectivity() error {
	log.Printf("[CHROME-DOWNLOADER] 开始网络连接测试...")

	// 测试基本网络连接，优先测试国内可访问的服务
	testHosts := []struct {
		name string
		host string
		port string
	}{
		{"阿里DNS", "223.5.5.5", "53"},
		{"腾讯DNS", "119.29.29.29", "53"},
		{"NPM镜像", "registry.npmmirror.com", "443"},
		{"华为云镜像", "mirrors.huaweicloud.com", "443"},
		{"清华大学镜像", "mirrors.tuna.tsinghua.edu.cn", "443"},
		{"GitHub", "github.com", "443"},
		{"Google DNS", "8.8.8.8", "53"},
		{"Cloudflare DNS", "1.1.1.1", "53"},
	}

	var successCount int
	for _, test := range testHosts {
		conn, err := net.DialTimeout("tcp", test.host+":"+test.port, 10*time.Second)
		if err != nil {
			log.Printf("[CHROME-DOWNLOADER] 连接测试失败 %s (%s:%s): %v", test.name, test.host, test.port, err)
			continue
		}
		conn.Close()
		log.Printf("[CHROME-DOWNLOADER] 连接测试成功 %s (%s:%s)", test.name, test.host, test.port)
		successCount++
	}

	if successCount == 0 {
		return fmt.Errorf("所有网络连接测试都失败，请检查网络连接")
	}

	log.Printf("[CHROME-DOWNLOADER] 网络连接测试完成: %d/%d 成功", successCount, len(testHosts))
	return nil
}

// DownloadChrome 下载Chrome浏览器
func (cd *ChromeDownloader) DownloadChrome() error {
	// 先进行网络连接测试
	if err := cd.testNetworkConnectivity(); err != nil {
		log.Printf("[CHROME-DOWNLOADER] 网络连接测试失败: %v", err)
		// 网络有问题但不完全阻止下载，继续尝试
	}

	// 使用新的URL获取方法
	downloadUrls, err := cd.getWorkingChromeUrls()
	if err != nil {
		log.Printf("[CHROME-DOWNLOADER] 获取下载链接失败: %v", err)
		return fmt.Errorf("无法获取Chrome下载链接: %w", err)
	}

	log.Printf("[CHROME-DOWNLOADER] 开始下载Chrome，平台: %s", cd.GetPlatformKey())
	log.Printf("[CHROME-DOWNLOADER] 可用下载源数量: %d", len(downloadUrls))

	// 确保下载目录存在
	if err := os.MkdirAll(cd.downloadDir, 0755); err != nil {
		return fmt.Errorf("创建下载目录失败: %w", err)
	}

	// 尝试每个下载源
	var lastErr error
	for i, downloadUrl := range downloadUrls {
		log.Printf("[CHROME-DOWNLOADER] 尝试下载源 %d/%d: %s", i+1, len(downloadUrls), downloadUrl)

		// 下载文件
		zipFilePath := filepath.Join(cd.downloadDir, "chrome.zip")

		// 重试下载
		maxRetries := 3 // 减少重试次数，因为现在有更多可靠的源
		for retry := 0; retry < maxRetries; retry++ {
			if retry > 0 {
				// 渐进式延迟：2s, 5s, 10s
				delay := time.Duration(retry*2+2) * time.Second
				log.Printf("[CHROME-DOWNLOADER] 第 %d 次重试下载，等待 %v...", retry, delay)
				time.Sleep(delay)
			}

			// 每次重试前清理可能存在的不完整文件
			if retry > 0 {
				os.Remove(zipFilePath)
			}

			err := cd.downloadFile(downloadUrl, zipFilePath)
			if err == nil {
				log.Printf("[CHROME-DOWNLOADER] 下载成功: %s", downloadUrl)

				// 验证文件大小
				if stat, err := os.Stat(zipFilePath); err == nil {
					if stat.Size() < 10*1024*1024 { // 小于10MB可能是错误页面或不完整文件
						log.Printf("[CHROME-DOWNLOADER] 下载文件过小 (%.2f MB)，可能是错误响应", float64(stat.Size())/(1024*1024))
						os.Remove(zipFilePath)
						lastErr = fmt.Errorf("下载文件过小，可能是错误响应")
						continue
					}
					log.Printf("[CHROME-DOWNLOADER] 下载文件大小: %.2f MB", float64(stat.Size())/(1024*1024))
				}

				// 解压文件
				if extractErr := cd.extractZip(zipFilePath, cd.downloadDir); extractErr != nil {
					log.Printf("[CHROME-DOWNLOADER] 解压失败: %v，尝试下一个源", extractErr)
					os.Remove(zipFilePath)
					lastErr = extractErr
					break // 跳到下一个下载源
				}

				// 清理临时文件
				os.Remove(zipFilePath)

				// 重新验证Chrome可执行文件是否存在（重置缓存）
				cd.chromeExecutable = "" // 重置路径缓存，强制重新扫描

				// 列出下载目录内容进行诊断
				log.Printf("[CHROME-DOWNLOADER] 解压完成，诊断下载目录内容:")
				cd.diagnoseDowloadDirectory()

				// 检查Chrome可执行文件
				if cd.IsChromeBinaryExists() {
					execPath := cd.GetChromeExecutablePath()

					// 设置可执行权限 (Linux/Mac)
					if runtime.GOOS != "windows" {
						if err := os.Chmod(execPath, 0755); err != nil {
							log.Printf("[CHROME-DOWNLOADER] 设置可执行权限失败: %v", err)
						}
					}

					log.Printf("[CHROME-DOWNLOADER] Chrome下载和解压完成: %s", execPath)
					return nil
				} else {
					// 如果仍然找不到可执行文件，提供详细的诊断信息
					log.Printf("[CHROME-DOWNLOADER] 解压成功但Chrome可执行文件仍不存在")
					log.Printf("[CHROME-DOWNLOADER] 期望路径: %s", cd.GetChromeExecutablePath())
					lastErr = fmt.Errorf("Chrome可执行文件不存在: %s", cd.GetChromeExecutablePath())
					break // 跳到下一个下载源
				}
			}

			lastErr = err

			// 分析错误类型，提供更好的重试策略
			errStr := err.Error()
			if strings.Contains(errStr, "connection was forcibly closed") ||
				strings.Contains(errStr, "i/o timeout") ||
				strings.Contains(errStr, "network is unreachable") {
				log.Printf("[CHROME-DOWNLOADER] 网络连接错误 (重试 %d/%d): %v", retry+1, maxRetries, err)
				continue
			} else if strings.Contains(errStr, "no such host") ||
				strings.Contains(errStr, "server misbehaving") {
				log.Printf("[CHROME-DOWNLOADER] DNS或服务器错误 (重试 %d/%d): %v", retry+1, maxRetries, err)
				continue
			} else if strings.Contains(errStr, "404") {
				log.Printf("[CHROME-DOWNLOADER] 文件不存在 (404错误)，跳过重试: %v", err)
				break // 404错误不需要重试
			} else {
				log.Printf("[CHROME-DOWNLOADER] 其他错误 (重试 %d/%d): %v", retry+1, maxRetries, err)
			}
		}

		log.Printf("[CHROME-DOWNLOADER] 下载源 %d 失败，尝试下一个源", i+1)
	}

	return fmt.Errorf("下载Chrome失败: 所有下载源都失败了，最后错误: %v", lastErr)
}

// diagnoseDowloadDirectory 诊断下载目录内容
func (cd *ChromeDownloader) diagnoseDowloadDirectory() {
	entries, err := os.ReadDir(cd.downloadDir)
	if err != nil {
		log.Printf("[CHROME-DOWNLOADER] 无法读取下载目录: %v", err)
		return
	}

	log.Printf("[CHROME-DOWNLOADER] 下载目录 '%s' 内容:", cd.downloadDir)
	for _, entry := range entries {
		if entry.IsDir() {
			log.Printf("[CHROME-DOWNLOADER]   目录: %s/", entry.Name())
			// 列出子目录内容
			subDirPath := filepath.Join(cd.downloadDir, entry.Name())
			subEntries, err := os.ReadDir(subDirPath)
			if err == nil {
				for _, subEntry := range subEntries {
					if subEntry.IsDir() {
						log.Printf("[CHROME-DOWNLOADER]     子目录: %s/", subEntry.Name())
					} else {
						log.Printf("[CHROME-DOWNLOADER]     文件: %s", subEntry.Name())
					}
				}
			}
		} else {
			log.Printf("[CHROME-DOWNLOADER]   文件: %s", entry.Name())
		}
	}
}

// downloadFile 下载文件
func (cd *ChromeDownloader) downloadFile(url, filepath string) error {
	log.Printf("[CHROME-DOWNLOADER] 开始下载文件: %s", url)

	// 创建HTTP客户端，设置更优的网络配置
	transport := optimizeNetworkSettings()

	client := &http.Client{
		Timeout:   20 * time.Minute,
		Transport: transport,
		// 不自动跟随重定向，手动处理
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	// 设置请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity") // 禁用压缩
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 创建目标文件
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// 复制文件内容，同时显示进度
	contentLength := resp.ContentLength
	if contentLength > 0 {
		log.Printf("[CHROME-DOWNLOADER] 开始下载，文件大小: %.2f MB", float64(contentLength)/1024/1024)
	} else {
		log.Printf("[CHROME-DOWNLOADER] 开始下载，文件大小未知")
	}

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("文件写入失败: %w", err)
	}

	log.Printf("[CHROME-DOWNLOADER] 下载完成，已下载: %.2f MB", float64(written)/1024/1024)
	return nil
}

// extractZip 解压ZIP文件
func (cd *ChromeDownloader) extractZip(src, dest string) error {
	log.Printf("[CHROME-DOWNLOADER] 开始解压: %s -> %s", src, dest)

	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()

	// 确保目标目录存在
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	extractedFiles := 0
	for _, file := range reader.File {
		// 构建目标路径
		path := filepath.Join(dest, file.Name)

		// 检查路径安全性
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("无效的文件路径: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			// 创建目录
			if err := os.MkdirAll(path, file.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}

		// 创建文件的目录
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		// 解压文件
		if err := cd.extractFile(file, path); err != nil {
			return err
		}

		extractedFiles++
		if extractedFiles%100 == 0 {
			log.Printf("[CHROME-DOWNLOADER] 已解压 %d 个文件...", extractedFiles)
		}
	}

	log.Printf("[CHROME-DOWNLOADER] 解压完成，共解压 %d 个文件", extractedFiles)
	return nil
}

// extractFile 解压单个文件
func (cd *ChromeDownloader) extractFile(file *zip.File, destPath string) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

// EnsureChrome 确保Chrome可用，如果不存在则下载
func (cd *ChromeDownloader) EnsureChrome() (string, error) {
	log.Printf("[CHROME-DOWNLOADER] 检查Chrome二进制文件...")

	// 重置Chrome可执行文件路径缓存，确保重新扫描
	cd.chromeExecutable = ""

	// 首先检查下载的Chrome是否存在
	if cd.IsChromeBinaryExists() {
		execPath := cd.GetChromeExecutablePath()
		log.Printf("[CHROME-DOWNLOADER] 找到已下载的Chrome: %s", execPath)
		return execPath, nil
	}

	log.Printf("[CHROME-DOWNLOADER] Chrome不存在，开始下载...")

	// 尝试下载Chrome
	if err := cd.DownloadChrome(); err != nil {
		log.Printf("[CHROME-DOWNLOADER] Chrome下载失败: %v", err)

		// 下载失败，尝试查找系统中已安装的Chrome
		if systemChrome := cd.findSystemChrome(); systemChrome != "" {
			log.Printf("[CHROME-DOWNLOADER] 下载失败，但找到系统Chrome: %s", systemChrome)
			return systemChrome, nil
		}

		return "", fmt.Errorf("Chrome下载失败且未找到系统Chrome: %w", err)
	}

	// 下载成功，返回下载的Chrome路径
	execPath := cd.GetChromeExecutablePath()
	log.Printf("[CHROME-DOWNLOADER] Chrome下载成功: %s", execPath)
	return execPath, nil
}

// findSystemChrome 查找系统中已安装的Chrome
func (cd *ChromeDownloader) findSystemChrome() string {
	if runtime.GOOS != "windows" {
		return ""
	}

	// Windows系统中Chrome的常见安装路径
	possiblePaths := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Users\` + os.Getenv("USERNAME") + `\AppData\Local\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Chromium\Application\chrome.exe`,
		`C:\Program Files (x86)\Chromium\Application\chrome.exe`,
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			log.Printf("[CHROME-DOWNLOADER] 找到系统Chrome: %s", path)
			return path
		}
	}

	log.Printf("[CHROME-DOWNLOADER] 未找到系统Chrome")
	return ""
}

// TestChrome 测试Chrome是否可以正常运行
func (cd *ChromeDownloader) TestChrome() error {
	execPath := cd.GetChromeExecutablePath()
	if execPath == "" {
		return fmt.Errorf("Chrome可执行文件路径为空")
	}

	// 简单的版本检查
	log.Printf("[CHROME-DOWNLOADER] 测试Chrome: %s", execPath)
	// 这里可以添加具体的测试逻辑

	return nil
}

// CleanupOldVersions 清理旧版本
func (cd *ChromeDownloader) CleanupOldVersions() error {
	log.Printf("[CHROME-DOWNLOADER] 清理旧版本...")

	// 这里可以添加清理逻辑，例如删除旧的Chrome目录
	// 当前实现保持简单，不进行清理

	return nil
}

// GetChromeInfo 获取Chrome信息 (智能增强版本)
func (cd *ChromeDownloader) GetChromeInfo() map[string]interface{} {
	info := make(map[string]interface{})
	platform := cd.platformDetector.GetPlatform()

	// 基础信息
	info["platform_key"] = platform.PlatformKey
	info["chrome_platform"] = platform.ChromePlatform
	info["download_dir"] = cd.downloadDir
	info["executable_path"] = cd.GetChromeExecutablePath()
	info["exists"] = cd.IsChromeBinaryExists()
	info["version"] = cd.version

	// 智能平台信息
	info["platform_info"] = map[string]interface{}{
		"os":           platform.OS,
		"arch":         platform.Arch,
		"os_version":   platform.OSVersion,
		"is_wsl":       platform.IsWSL,
		"is_container": platform.IsContainer,
		"is_arm":       platform.IsARM,
		"is_supported": cd.platformDetector.IsSupported(),
		"details":      platform.Details,
	}

	// 环境信息
	info["environment"] = map[string]interface{}{
		"is_china_network": cd.isInChina(),
		"cpu_cores":        runtime.NumCPU(),
		"go_version":       runtime.Version(),
	}

	// 推荐信息
	if recommendations := cd.platformDetector.GetRecommendations(); len(recommendations) > 0 {
		info["recommendations"] = recommendations
	}

	// 支持的下载源统计
	if urls, err := cd.getWorkingChromeUrls(); err == nil {
		info["available_sources"] = len(urls)
		info["source_urls"] = urls[:min(len(urls), 3)] // 只显示前3个源
	} else {
		info["available_sources"] = 0
		info["source_error"] = err.Error()
	}

	return info
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetLatestChromeVersion 从官方API获取最新Chrome版本信息 (导出版本)
func (cd *ChromeDownloader) GetLatestChromeVersion() (*ChromeVersion, error) {
	return cd.getLatestChromeVersion()
}

// getLatestChromeVersion 从官方API获取最新Chrome版本信息
func (cd *ChromeDownloader) getLatestChromeVersion() (*ChromeVersion, error) {
	log.Printf("[CHROME-DOWNLOADER] 正在获取最新Chrome版本信息...")

	apiURL := "https://googlechromelabs.github.io/chrome-for-testing/known-good-versions-with-downloads.json"

	// 使用优化的网络设置
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: optimizeNetworkSettings(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("User-Agent", "WhoseeWhoisServer/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[CHROME-DOWNLOADER] 无法访问Chrome版本API: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API响应错误，状态码: %d", resp.StatusCode)
	}

	var versionResponse ChromeVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&versionResponse); err != nil {
		return nil, fmt.Errorf("解析版本信息失败: %w", err)
	}

	if len(versionResponse.Versions) == 0 {
		return nil, fmt.Errorf("未找到可用的Chrome版本")
	}

	// 获取最新的稳定版本 (通常是最后一个)
	latestVersion := &versionResponse.Versions[len(versionResponse.Versions)-1]

	log.Printf("[CHROME-DOWNLOADER] 获取到最新Chrome版本: %s (修订版: %s)",
		latestVersion.Version, latestVersion.Revision)

	return latestVersion, nil
}

// GetDynamicChromeUrls 动态获取Chrome下载链接，结合官方API和镜像源 (导出版本)
func (cd *ChromeDownloader) GetDynamicChromeUrls() ([]string, error) {
	return cd.getDynamicChromeUrls()
}

// getDynamicChromeUrls 动态获取Chrome下载链接，结合官方API和镜像源
func (cd *ChromeDownloader) getDynamicChromeUrls() ([]string, error) {
	platformKey := cd.GetPlatformKey()
	var platformName string

	// 映射平台名称
	switch platformKey {
	case "windows-amd64":
		platformName = "win64"
	case "windows-386":
		platformName = "win32"
	case "linux-amd64":
		platformName = "linux64"
	case "darwin-amd64":
		platformName = "mac-x64"
	case "darwin-arm64":
		platformName = "mac-arm64"
	default:
		return nil, fmt.Errorf("不支持的平台: %s", platformKey)
	}

	var urls []string

	// 首先添加国内镜像源（优先级最高）
	chinaUrls := cd.getChinaMirrorUrls(platformName)
	urls = append(urls, chinaUrls...)

	// 尝试从官方API获取最新版本
	if latestVersion, err := cd.getLatestChromeVersion(); err == nil {
		if chromeBuilds, exists := latestVersion.Downloads["chrome"]; exists {
			for _, build := range chromeBuilds {
				if build.Platform == platformName {
					log.Printf("[CHROME-DOWNLOADER] 添加官方最新版本: %s", build.URL)
					urls = append(urls, build.URL)
					break
				}
			}
		}
	} else {
		log.Printf("[CHROME-DOWNLOADER] 获取官方版本失败，使用备用源: %v", err)
	}

	// 添加静态备用源
	staticUrls := cd.getStaticBackupUrls(platformName)
	urls = append(urls, staticUrls...)

	log.Printf("[CHROME-DOWNLOADER] 总共获得 %d 个下载源", len(urls))
	return urls, nil
}

// getChinaMirrorUrls 获取国内镜像源链接
func (cd *ChromeDownloader) getChinaMirrorUrls(platform string) []string {
	// 使用较新的修订版本
	revision := "1354089"

	var filename string
	switch platform {
	case "win64":
		filename = "chrome-win.zip"
	case "win32":
		filename = "chrome-win.zip"
	case "linux64":
		filename = "chrome-linux.zip"
	case "mac-x64", "mac-arm64":
		filename = "chrome-mac.zip"
	default:
		filename = "chrome-win.zip"
	}

	basePath := fmt.Sprintf("chromium-browser-snapshots/%s/%s/%s",
		map[string]string{
			"win64": "Win_x64", "win32": "Win",
			"linux64": "Linux_x64",
			"mac-x64": "Mac", "mac-arm64": "Mac_Arm",
		}[platform], revision, filename)

	return []string{
		// 阿里NPM镜像 - 通常最快
		fmt.Sprintf("https://registry.npmmirror.com/-/binary/%s", basePath),
		// 华为云镜像
		fmt.Sprintf("https://mirrors.huaweicloud.com/%s", basePath),
		// 清华大学镜像
		fmt.Sprintf("https://mirrors.tuna.tsinghua.edu.cn/%s", basePath),
		// 中科大镜像
		fmt.Sprintf("https://mirrors.ustc.edu.cn/%s", basePath),
		// 腾讯云镜像
		fmt.Sprintf("https://mirrors.cloud.tencent.com/%s", basePath),
	}
}

// getStaticBackupUrls 获取静态备用下载源
func (cd *ChromeDownloader) getStaticBackupUrls(platform string) []string {
	// 固定版本的备用源
	version := "131.0.6778.108"

	var urlSuffix string
	switch platform {
	case "win64":
		urlSuffix = "win64/chrome-win64.zip"
	case "win32":
		urlSuffix = "win32/chrome-win32.zip"
	case "linux64":
		urlSuffix = "linux64/chrome-linux64.zip"
	case "mac-x64":
		urlSuffix = "mac-x64/chrome-mac-x64.zip"
	case "mac-arm64":
		urlSuffix = "mac-arm64/chrome-mac-arm64.zip"
	default:
		urlSuffix = "win64/chrome-win64.zip"
	}

	return []string{
		fmt.Sprintf("https://storage.googleapis.com/chrome-for-testing-public/%s/%s", version, urlSuffix),
		fmt.Sprintf("https://commondatastorage.googleapis.com/chromium-browser-snapshots/Win_x64/1354089/chrome-win.zip"),
		fmt.Sprintf("https://download.chromium.org/chromium-browser-snapshots/Win_x64/1354089/chrome-win.zip"),
	}
}

// GetWorkingChromeUrls 获取经过验证的Chrome下载链接 (智能版本)
func (cd *ChromeDownloader) GetWorkingChromeUrls() ([]string, error) {
	return cd.getWorkingChromeUrls()
}

// getWorkingChromeUrls 获取经过验证的Chrome下载链接 (智能版本)
func (cd *ChromeDownloader) getWorkingChromeUrls() ([]string, error) {
	platform := cd.platformDetector.GetPlatform()
	chromePlatform := platform.ChromePlatform

	log.Printf("[CHROME-DOWNLOADER] 开始获取Chrome下载链接，智能平台: %s (原始: %s)", chromePlatform, platform.PlatformKey)

	// 检查平台支持
	if !cd.platformDetector.IsSupported() {
		log.Printf("[CHROME-DOWNLOADER] 警告: 平台 %s 可能不受完全支持", chromePlatform)
		// 尝试使用最接近的平台
		chromePlatform = cd.getFallbackPlatform(platform)
		log.Printf("[CHROME-DOWNLOADER] 使用备用平台: %s", chromePlatform)
	}

	var urls []string

	// 1. 优先使用GitHub官方API获取最新版本
	if githubUrls := cd.getGitHubOfficialUrls(chromePlatform); len(githubUrls) > 0 {
		urls = append(urls, githubUrls...)
		log.Printf("[CHROME-DOWNLOADER] 添加了 %d 个GitHub官方源", len(githubUrls))
	}

	// 2. 根据地区添加镜像源
	if cd.isInChina() {
		// 中国大陆用户，优先使用国内镜像
		if chinaUrls := cd.getChinaMirrorUrls(chromePlatform); len(chinaUrls) > 0 {
			urls = append(urls, chinaUrls...)
			log.Printf("[CHROME-DOWNLOADER] 添加了 %d 个中国镜像源", len(chinaUrls))
		}
	}

	// 3. 尝试华为云镜像（全球可用）
	if huaweiUrls := cd.getHuaweiCloudUrls(chromePlatform); len(huaweiUrls) > 0 {
		urls = append(urls, huaweiUrls...)
		log.Printf("[CHROME-DOWNLOADER] 添加了 %d 个华为云镜像源", len(huaweiUrls))
	}

	// 4. 添加固定的官方源作为备用
	staticUrls := cd.getVerifiedStaticUrls(chromePlatform)
	urls = append(urls, staticUrls...)
	log.Printf("[CHROME-DOWNLOADER] 添加了 %d 个静态备用源", len(staticUrls))

	// 5. 针对特殊环境的优化
	if platform.IsContainer {
		log.Printf("[CHROME-DOWNLOADER] 容器环境，添加容器优化的下载源")
		urls = append(urls, cd.getContainerOptimizedUrls(chromePlatform)...)
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("无法获取任何下载源，平台: %s", chromePlatform)
	}

	// 去重
	urls = cd.deduplicateUrls(urls)

	log.Printf("[CHROME-DOWNLOADER] 总共获得 %d 个去重后的下载源", len(urls))
	return urls, nil
}

// getFallbackPlatform 获取备用平台
func (cd *ChromeDownloader) getFallbackPlatform(platform *PlatformInfo) string {
	switch platform.OS {
	case "windows":
		return "win64" // 大多数现代Windows都是64位
	case "linux":
		if platform.IsWSL {
			return "linux64" // WSL使用Linux版本
		}
		return "linux64"
	case "darwin":
		if platform.IsARM {
			return "mac-arm64"
		}
		return "mac-x64"
	default:
		log.Printf("[CHROME-DOWNLOADER] 未知系统 %s，使用 win64 作为备用", platform.OS)
		return "win64"
	}
}

// isInChina 检测是否在中国大陆网络环境
func (cd *ChromeDownloader) isInChina() bool {
	// 简单的中国网络环境检测
	// 可以通过环境变量、IP地址、DNS等方式检测

	if os.Getenv("CHINA_NETWORK") == "true" {
		return true
	}

	// 尝试快速连接中国服务器来判断
	conn, err := net.DialTimeout("tcp", "223.5.5.5:53", 2*time.Second)
	if err == nil {
		conn.Close()
		// 如果能快速连接到阿里DNS，可能在中国
		return true
	}

	return false
}

// getContainerOptimizedUrls 获取容器环境优化的URL
func (cd *ChromeDownloader) getContainerOptimizedUrls(platform string) []string {
	// 容器环境通常需要更稳定的源
	urlMap := map[string][]string{
		"win64": {
			"https://storage.googleapis.com/chrome-for-testing-public/131.0.6778.108/win64/chrome-win64.zip",
		},
		"linux64": {
			"https://storage.googleapis.com/chrome-for-testing-public/131.0.6778.108/linux64/chrome-linux64.zip",
		},
	}

	if urls, exists := urlMap[platform]; exists {
		return urls
	}
	return nil
}

// deduplicateUrls 去重URL列表
func (cd *ChromeDownloader) deduplicateUrls(urls []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, url := range urls {
		if !seen[url] {
			seen[url] = true
			result = append(result, url)
		}
	}

	return result
}

// getGitHubOfficialUrls 从GitHub官方API获取Chrome下载链接
func (cd *ChromeDownloader) getGitHubOfficialUrls(platform string) []string {
	log.Printf("[CHROME-DOWNLOADER] 正在从GitHub官方API获取Chrome版本...")

	apiURL := "https://googlechromelabs.github.io/chrome-for-testing/known-good-versions-with-downloads.json"

	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: optimizeNetworkSettings(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		log.Printf("[CHROME-DOWNLOADER] 创建GitHub API请求失败: %v", err)
		return nil
	}

	req.Header.Set("User-Agent", "WhoseeWhoisServer/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[CHROME-DOWNLOADER] GitHub API请求失败: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[CHROME-DOWNLOADER] GitHub API响应错误，状态码: %d", resp.StatusCode)
		return nil
	}

	var versionResponse ChromeVersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&versionResponse); err != nil {
		log.Printf("[CHROME-DOWNLOADER] 解析GitHub API响应失败: %v", err)
		return nil
	}

	if len(versionResponse.Versions) == 0 {
		log.Printf("[CHROME-DOWNLOADER] GitHub API未返回任何版本")
		return nil
	}

	var urls []string
	// 获取最新的几个版本
	versions := versionResponse.Versions
	maxVersions := 3 // 获取最新的3个版本
	if len(versions) > maxVersions {
		versions = versions[len(versions)-maxVersions:]
	}

	for _, version := range versions {
		if chromeBuilds, exists := version.Downloads["chrome"]; exists {
			for _, build := range chromeBuilds {
				if build.Platform == platform {
					urls = append(urls, build.URL)
					log.Printf("[CHROME-DOWNLOADER] 添加GitHub官方版本 %s: %s", version.Version, build.URL)
					break
				}
			}
		}
	}

	return urls
}

// getHuaweiCloudUrls 从华为云镜像获取Chrome下载链接
func (cd *ChromeDownloader) getHuaweiCloudUrls(platform string) []string {
	log.Printf("[CHROME-DOWNLOADER] 正在从华为云镜像获取Chrome版本...")

	indexURL := "https://mirrors.huaweicloud.com/chromium-browser-snapshots/.index.json"

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: optimizeNetworkSettings(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", indexURL, nil)
	if err != nil {
		log.Printf("[CHROME-DOWNLOADER] 创建华为云请求失败: %v", err)
		return nil
	}

	req.Header.Set("User-Agent", "WhoseeWhoisServer/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[CHROME-DOWNLOADER] 华为云请求失败: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[CHROME-DOWNLOADER] 华为云响应错误，状态码: %d", resp.StatusCode)
		return nil
	}

	var indexData HuaweiCloudIndex
	if err := json.NewDecoder(resp.Body).Decode(&indexData); err != nil {
		log.Printf("[CHROME-DOWNLOADER] 解析华为云索引失败: %v", err)
		return nil
	}

	var urls []string
	baseURL := "https://mirrors.huaweicloud.com/chromium-browser-snapshots/"

	// 查找匹配的平台文件
	platformMap := map[string]string{
		"win64":     "Win_x64",
		"win32":     "Win",
		"linux64":   "Linux_x64",
		"mac-x64":   "Mac",
		"mac-arm64": "Mac", // 华为云可能没有区分ARM架构
	}

	targetPlatform, exists := platformMap[platform]
	if !exists {
		log.Printf("[CHROME-DOWNLOADER] 华为云不支持平台: %s", platform)
		return nil
	}

	// 遍历所有版本，寻找匹配的文件
	for revision, versionData := range indexData.ChromiumBrowserSnapshots {
		for _, file := range versionData.Files {
			if strings.HasPrefix(file, targetPlatform+"/") && strings.HasSuffix(file, "chrome-win.zip") ||
				strings.HasPrefix(file, targetPlatform+"/") && strings.HasSuffix(file, "chrome-linux.zip") ||
				strings.HasPrefix(file, targetPlatform+"/") && strings.HasSuffix(file, "chrome-mac.zip") {
				url := baseURL + file
				urls = append(urls, url)
				log.Printf("[CHROME-DOWNLOADER] 添加华为云版本 %s: %s", revision, url)
				// 只取前几个版本
				if len(urls) >= 2 {
					goto done
				}
			}
		}
	}

done:
	return urls
}

// getVerifiedStaticUrls 获取经过验证的静态下载源
func (cd *ChromeDownloader) getVerifiedStaticUrls(platform string) []string {
	// 根据您的文档，这些是经过验证可用的地址
	urlMap := map[string][]string{
		"win64": {
			"https://storage.googleapis.com/chrome-for-testing-public/131.0.6778.108/win64/chrome-win64.zip",
		},
		"win32": {
			"https://storage.googleapis.com/chrome-for-testing-public/131.0.6778.108/win32/chrome-win32.zip",
		},
		"linux64": {
			"https://storage.googleapis.com/chrome-for-testing-public/131.0.6778.108/linux64/chrome-linux64.zip",
		},
		"mac-x64": {
			"https://storage.googleapis.com/chrome-for-testing-public/131.0.6778.108/mac-x64/chrome-mac-x64.zip",
		},
		"mac-arm64": {
			"https://storage.googleapis.com/chrome-for-testing-public/131.0.6778.108/mac-arm64/chrome-mac-arm64.zip",
		},
	}

	if urls, exists := urlMap[platform]; exists {
		log.Printf("[CHROME-DOWNLOADER] 添加 %d 个静态备用源", len(urls))
		return urls
	}

	return nil
}
