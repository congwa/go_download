package updater

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// StartUpdateChecker 启动更新检查器
func StartUpdateChecker() error {
	ticker := time.NewTicker(CHECK_INTERVAL)
	for range ticker.C {
		if err := CheckAndUpdate(); err != nil {
			Logf("定时更新检查失败: %v", err)
		}
	}
	return nil
}

// CheckAndUpdate 检查并更新程序
func CheckAndUpdate() error {
	Logf("开始检查更新...")

	// 获取远程版本信息
	versionInfo, err := getRemoteVersion()
	if err != nil {
		return fmt.Errorf("获取远程版本失败: %v", err)
	}

	// 获取当前平台下载链接
	downloadUrl := getPlatformDownloadUrl(versionInfo)
	if downloadUrl == "" {
		return fmt.Errorf("没有适合当前平台的下载链接")
	}

	// 检查版本并更新
	return updateIfNeeded(versionInfo, downloadUrl)
}

// getRemoteVersion 获取远程版本信息
func getRemoteVersion() (*VersionInfo, error) {
	headers, err := GenerateHeaders()
	if err != nil {
		return nil, fmt.Errorf("生成请求头失败: %v", err)
	}

	req, err := http.NewRequest("GET", API_URL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Timestamp", headers.Timestamp)
	req.Header.Set("X-Sign", headers.Sign)
	req.Header.Set("User-Agent", "MyTV/1.0")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误(状态码:%d): %s", resp.StatusCode, string(body))
	}

	// 解密响应数据
	decodedData, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		return nil, fmt.Errorf("Base64解码失败: %v", err)
	}

	decryptedData, err := decryptAESCBC(decodedData)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %v", err)
	}

	var versionInfo VersionInfo
	if err := json.Unmarshal(decryptedData, &versionInfo); err != nil {
		return nil, fmt.Errorf("解析版本信息失败: %v", err)
	}

	return &versionInfo, nil
}

// getPlatformDownloadUrl 获取当前平台的下载链接
func getPlatformDownloadUrl(versionInfo *VersionInfo) string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	switch os {
	case "linux":
		switch arch {
		case "amd64":
			return versionInfo.Amd64
		case "arm64":
			return versionInfo.Arm64
		case "arm":
			return versionInfo.Arm
		}
	case "darwin":
		return versionInfo.Darwin
	}

	Logf("不支持的平台: %s/%s", os, arch)
	return ""
}

// updateIfNeeded 在需要时更新程序
func updateIfNeeded(versionInfo *VersionInfo, downloadUrl string) error {
	localVersion, err := readLocalVersion()
	needUpdate := err != nil || localVersion != versionInfo.Version

	if needUpdate {
		// 需要更新时的逻辑
		versionInfo.DownloadUrl = downloadUrl
		if err := downloadAndRun(versionInfo); err != nil {
			return fmt.Errorf("更新失败: %v", err)
		}
		if err := saveLocalVersion(versionInfo.Version); err != nil {
			return fmt.Errorf("保存版本信息失败: %v", err)
		}
		Logf("更新成功，新版本: %s", versionInfo.Version)
	} else {
		// 不需要更新时，检查端口状态
		Logf("当前版本已是最新: %s，检查服务状态", localVersion)
		// 使用新的封装函数
		waitWithCountdown(90, "开始等待90秒...")
		// 检查端口是否在监听
		if !isPortInUse(35455) {
			Logf("端口 35455 未被监听，启动服务")
			if err := ensureServiceRunning(); err != nil {
				return fmt.Errorf("启动服务失败: %v", err)
			}
		} else {
			Logf("服务正在运行，端口 35455 正常监听中")
		}
	}

	return nil
}

// waitWithCountdown 带倒计时的等待函数
func waitWithCountdown(seconds int, message string) {
	Logf(message)
	startTime := time.Now()
	for i := seconds; i > 0; i-- {
		elapsed := time.Since(startTime).Seconds()
		fmt.Printf("\r等待剩余时间: %d 秒 (已等待: %.0f 秒)", i, elapsed)
		time.Sleep(1 * time.Second)
	}
	fmt.Println() // 换行
	Logf("等待%d秒结束", seconds)
}

// 新增：确保服务运行的函数
func ensureServiceRunning() error {
	// 获取本地文件的绝对路径
	absPath, err := filepath.Abs(LOCAL_FILE)
	if err != nil {
		return fmt.Errorf("获取本地文件路径失败: %v", err)
	}

	// 检查文件是否存在
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("本地文件不存在: %v", err)
	}

	// 确保有执行权限
	if err := os.Chmod(absPath, 0755); err != nil {
		return fmt.Errorf("设置执行权限失败: %v", err)
	}
	waitWithCountdown(20, "等待20秒，确保端口 35455 的占用进程关闭")
	Logf("尝试关闭端口 35455 的占用进程,确保端口可用")
	if err := stopProcessByPort(35455); err != nil {
		return fmt.Errorf("端口 35455 被占用且无法关闭: %v", err)
	}

	// 创建完成信号通道
	done := make(chan error)

	go func() {
		Logf("开始执行本地文件: %s", absPath)
		if err := executeNewFile(absPath); err != nil {
			done <- fmt.Errorf("执行本地文件失败: %v", err)
			return
		}

		ip, err := getLocalIP()
		if err != nil {
			done <- fmt.Errorf("获取本地 IP 失败: %v", err)
			return
		}

		Logf("执行成功, 监听 %s:35455", ip)
		done <- nil
	}()

	// 等待执行完成或超时
	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("启动服务超时")
	}

	return nil
}

// downloadAndRun 下载并运行新版本
func downloadAndRun(info *VersionInfo) error {
	// 首先检查端口
	if isPortInUse(35455) {
		Logf("端口 35455 已被占用，尝试关闭占用进程...")
		if err := stopProcessByPort(35455); err != nil {
			return fmt.Errorf("无法关闭占用端口的进程: %v", err)
		}
	}

	Logf("开始下载文件...")
	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			DisableKeepAlives:     true,
			Proxy:                 http.ProxyFromEnvironment,
		},
	}

	// 先尝试使用代理下载
	proxyURL := "https://ghp.ci/" + info.DownloadUrl
	if err := tryDownload(client, proxyURL); err != nil {
		Logf("代理下载失败: %v，尝试直接下载", err)
		// 代理失败后尝试直接下载
		if err := tryDownload(client, info.DownloadUrl); err != nil {
			return fmt.Errorf("所有下载方式均失败: %v", err)
		}
	}

	Logf("下载完成")

	// 添加执行权限
	if err := os.Chmod(LOCAL_FILE, 0755); err != nil {
		return fmt.Errorf("设置执行权限失败: %v", err)
	}
	Logf("已设置执行权限")

	// 创建一个完成信号通道
	done := make(chan error)

	go func() {
		if err := StopProcessByName(); err != nil {
			done <- fmt.Errorf("停止旧进程失败: %v", err)
			return
		}

		// 获取文件的绝对路径
		absPath, err := filepath.Abs(LOCAL_FILE)
		if err != nil {
			done <- fmt.Errorf("获取文件绝对路径失败: %v", err)
			return
		}

		Logf("开始执行新文件: %s", absPath)
		if err := executeNewFile(absPath); err != nil {
			done <- fmt.Errorf("执行新文件失败: %v", err)
			return
		}

		ip, err := getLocalIP()
		if err != nil {
			done <- fmt.Errorf("获取本地 IP 失败: %v", err)
			return
		}

		Logf("执行成功, 监听 %s:35455", ip)
		done <- nil
	}()

	// 等待新程序启动完成或超时
	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-time.After(5000 * time.Second):
		return fmt.Errorf("启动新程序超时")
	}

	// 确保日志完全写入
	time.Sleep(1 * time.Second)

	return nil
}

// readLocalVersion 读取本地版本号
func readLocalVersion() (string, error) {
	data, err := os.ReadFile(VERSION_FILE)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(data)), nil
}

// saveLocalVersion 保存本地版本号
func saveLocalVersion(version string) error {
	return os.WriteFile(VERSION_FILE, []byte(version), 0644)
}

// tryDownload 处理下载逻辑
func tryDownload(client *http.Client, url string) error {
	Logf("尝试从 %s 下载", url)

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP状态码错误: %d", resp.StatusCode)
	}

	// 创建临时文件
	tmpFile := LOCAL_FILE + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer func() {
		out.Close()
		os.Remove(tmpFile) // 清理临时文件
	}()

	// 写入临时文件
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	// 确保文件完全写入
	if err := out.Sync(); err != nil {
		return fmt.Errorf("同步文件失败: %v", err)
	}
	out.Close()

	// 在重命名文件之前设置执行权限
	if err := os.Chmod(tmpFile, 0755); err != nil {
		return fmt.Errorf("设置文件权限失败: %v", err)
	}

	// 重命名临时文件为目标文件
	if err := os.Rename(tmpFile, LOCAL_FILE); err != nil {
		return fmt.Errorf("重命名文件失败: %v", err)
	}

	return nil
}

// ... (其他更新相关函数)
