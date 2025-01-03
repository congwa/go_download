package main

import (
	"fmt"
	"go_auto_download/pkg/updater"
	"log"
	"os"
)

func Logf(s string, err error) {
	updater.Logf(s, err)
}

func main() {
	// 先初始化日志
	if err := updater.InitLogger(); err != nil {
		log.Printf("初始化日志失败: %v", err)
		return
	}

	// 检查是否是版本查询模式
	if len(os.Args) > 1 && os.Args[1] == "-v" {
		if len(os.Args) < 3 {
			log.Fatalf("请提供版本号参数")
			return
		}

		版本 := os.Args[2]
		// 验证版本号格式
		if !updater.IsValidVersion(版本) {
			log.Fatalf("无效的版本号格式，请使用 x.y.z 格式（如：1.0.0）")
			return
		}

		// 限制版本号长度
		if len(版本) > 20 {
			log.Fatalf("版本号过长")
			return
		}

		// 执行版本查询
		if err := updater.SubmitVersion(版本); err != nil {
			log.Fatalf("版本查询失败: %v", err)
		}
		return
	}

	// 先检查并关闭旧进程
	if err := updater.StopProcessByName(); err != nil {
		Logf("关闭旧进程失败: %v", err)
	}

	// 正常更新检查逻辑
	if err := run(); err != nil {
		Logf("程序运行错误: %v", err)
		os.Exit(1)
	}
}

// run 封装主要的运行逻辑
func run() error {
	// 初始化日志
	if err := updater.InitLogger(); err != nil {
		return fmt.Errorf("初始化日志失败: %v", err)
	}

	// 记录启动信息
	updater.LogStartupInfo()

	// 首次运行立即检查
	if err := updater.CheckAndUpdate(); err != nil {
		Logf("首次更新检查失败: %v", err)
	}

	// 定时检查更新
	return updater.StartUpdateChecker()
}
