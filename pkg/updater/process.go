package updater

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// 添加根据进程名停止进程的函数
func StopProcessByName() error {
	Logf("尝试关闭已运行的进程")

	// 获取当前进程的 PID
	currentPID := os.Getpid()
	Logf("当前进程 PID: %d", currentPID)

	// 使用 BusyBox 兼容的 ps 命令
	cmd := exec.Command("ps", "w")
	output, err := cmd.Output()
	if err != nil {
		Logf("执行 ps 命令失败: %v", err)
		return fmt.Errorf("执行 ps 命令失败: %v", err)
	}

	foundProcess := false
	lines := strings.Split(string(output), "\n")

	// 跳过标题行
	for _, line := range lines[1:] {
		// 检查行是否包含 "download_all"，而不是完全匹配
		if strings.Contains(strings.ToLower(line), "download_all") {
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				pid := fields[0]

				// 转换 PID 为整数进行比较
				pidInt, err := strconv.Atoi(pid)
				if err != nil {
					Logf("PID 转换失败: %v", err)
					continue
				}

				// 跳过当前进程
				if pidInt == currentPID {
					Logf("跳过当前进程 (PID:%s)", pid)
					continue
				}

				Logf("找到目标进程 (PID:%s), 完整进程信息: %s", pid, line)

				// 尝试使用 kill 命令终止进程
				killCmd := exec.Command("kill", "-9", pid)
				if err := killCmd.Run(); err != nil {
					Logf("关闭进程 (PID:%s) 失败: %v", pid, err)
				} else {
					Logf("已成功关闭进程 (PID:%s)", pid)
					foundProcess = true
				}
			}
		}
	}

	if !foundProcess {
		Logf("未找到其他需要关闭的进程")
	} else {
		// 如果找到并关闭了进程，等待一会确保进程完全终止
		time.Sleep(time.Second)
	}

	return nil
}

// 通过端口号停止进程
func stopProcessByPort(port int) error {
	switch runtime.GOOS {
	case "darwin": // macOS
		return stopPortProcessDarwin(port)
	case "linux":
		return stopPortProcessLinux(port)
	default:
		return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}
}

// macOS 下通过端口停止进程
func stopPortProcessDarwin(port int) error {
	// 使用 lsof 查找占用端口的进程
	cmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
	output, err := cmd.Output()
	if err != nil {
		// 如果没有找到进程，不报错
		return nil
	}

	if len(output) > 0 {
		pid := strings.TrimSpace(string(output))
		killCmd := exec.Command("kill", "-9", pid)
		if err := killCmd.Run(); err != nil {
			return fmt.Errorf("无法关闭占用端口的进程(PID:%s): %v", pid, err)
		}
		Logf("已关闭占用端口 %d 的进程(PID:%s)", port, pid)
		// 等待端口释放
		time.Sleep(1 * time.Second)
	}
	return nil
}

// Linux 下通过端口停止进程
func stopPortProcessLinux(port int) error {
	// 使用 ss 命令查找占用端口的进程
	cmd := exec.Command("ss", "-lptn", fmt.Sprintf("sport = :%d", port))
	output, err := cmd.Output()
	if err != nil {
		// 如果命令失败，尝试使用 netstat
		cmd = exec.Command("netstat", "-tunlp", "|", "grep", fmt.Sprintf(":%d", port))
		output, err = cmd.Output()
		if err != nil {
			// 如果都失败了，返回 nil（假设没有进程占用）
			return nil
		}
	}

	// 从输出中提取 PID
	if len(output) > 0 {
		// 使用正则表达式提取 PID
		re := regexp.MustCompile(`pid=(\d+)`)
		matches := re.FindStringSubmatch(string(output))
		if len(matches) > 1 {
			pid := matches[1]
			// 终止进程
			killCmd := exec.Command("kill", "-9", pid)
			if err := killCmd.Run(); err != nil {
				return fmt.Errorf("无法关闭占用端口的进程(PID:%s): %v", pid, err)
			}
			Logf("已关闭占用端口 %d 的进程(PID:%s)", port, pid)
			// 等待端口释放
			time.Sleep(1 * time.Second)
		}
	}
	return nil
}

// 修改 isPortInUse 函数，增加实际连接测试
func isPortInUse(port int) bool {
	// 首先尝试监听端口
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		listener.Close()
		return false
	}

	// 如果端口被占用，尝试连接测试服务是否真的在运行
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		// 连接失败，说明服务可能已经崩溃
		return false
	}
	conn.Close()
	return true
}

// 获取本地 IP 地址的函数
func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String(), nil
		}
	}
	return "", fmt.Errorf("未找到有效的本地 IP 地址")
}

func executeNewFile(filePath string) error {
	cmd := exec.Command(filePath)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("执行文件失败: %v, 输出: %s", err, output.String())
	}

	// 等待服务启动（最多等待5秒）
	if err := checkServiceHealth(5 * time.Second); err != nil {
		return fmt.Errorf("服务启动检查失败: %v", err)
	}

	ip, err := getLocalIP()
	if err != nil {
		Logf("获取本地 IP 地址失败: %v", err)
		return err
	}

	Logf("执行成功, 监听 %s:35455", ip)
	return nil
}

// 添加一个函数来检查服务是否正常响应
func checkServiceHealth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := "http://127.0.0.1:35455"
	client := &http.Client{
		Timeout: 2 * time.Second, // 单次请求超时
	}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return nil // 服务正常响应
		}
		time.Sleep(500 * time.Millisecond) // 等待500ms后重试
	}
	return fmt.Errorf("服务未在预期时间内响应")
}
