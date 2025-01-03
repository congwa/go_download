package updater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

// IsValidVersion 检查版本号格式是否有效
func IsValidVersion(version string) bool {
	pattern := `^\d+\.\d+\.\d+$`
	matched, err := regexp.MatchString(pattern, version)
	if err != nil {
		return false
	}
	return matched
}

// SubmitVersion 提交版本号到服务器
func SubmitVersion(version string) error {
	if err := InitLogger(); err != nil {
		return fmt.Errorf("初始化日志失败: %v", err)
	}

	headers, err := GenerateHeaders()
	if err != nil {
		return fmt.Errorf("生成请求头失败: %v", err)
	}

	requestBody := map[string]string{
		"version": version,
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %v", err)
	}

	req, err := http.NewRequest("POST", API_URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("X-Timestamp", headers.Timestamp)
	req.Header.Set("X-Sign", headers.Sign)
	req.Header.Set("User-Agent", "MyTV/1.0")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		Logf("发送请求失败: %v", err)
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		Logf("读取响应失败: %v", err)
		return fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		Logf("服务器返回错误(状态码:%d): %s", resp.StatusCode, string(body))
		return fmt.Errorf("服务器返回错误(状态码:%d): %s", resp.StatusCode, string(body))
	}

	fmt.Println(string(body))
	Logf("版本提交成功: %s", version)
	return nil
}
