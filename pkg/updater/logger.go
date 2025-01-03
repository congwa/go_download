package updater

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type LogManager struct {
	currentDay    string
	currentLogger *log.Logger
	Logfile       *os.File
}

// 全局变量
var logManager *LogManager

// 初始化日志管理器
func InitLogger() error {
	logManager = &LogManager{}
	if err := os.MkdirAll(LOG_DIR, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	if err := logManager.rotateLog(); err != nil {
		return fmt.Errorf("初始化日志失败: %v", err)
	}

	// 清理旧日志
	if err := cleanOldLogs(); err != nil {
		return fmt.Errorf("清理旧日志失败: %v", err)
	}

	return nil
}

// 日志轮转
func (lm *LogManager) rotateLog() error {
	currentDay := time.Now().Format("2006-01-02")

	// 如果日期没变且文件已打开，直接返回
	if currentDay == lm.currentDay && lm.Logfile != nil {
		return nil
	}

	// 关闭旧的日志文件
	if lm.Logfile != nil {
		lm.Logfile.Close()
	}

	// 打开新的日志文件
	logPath := filepath.Join(LOG_DIR, fmt.Sprintf("app_%s.log", currentDay))
	Logfile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	lm.currentDay = currentDay
	lm.Logfile = Logfile
	lm.currentLogger = log.New(Logfile, "", log.Ldate|log.Ltime|log.Lshortfile)

	return nil
}

// 清理旧日志
func cleanOldLogs() error {
	files, err := os.ReadDir(LOG_DIR)
	if err != nil {
		return err
	}

	var Logfiles []string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "app_") && strings.HasSuffix(file.Name(), ".log") {
			Logfiles = append(Logfiles, file.Name())
		}
	}

	// 按日期排序
	sort.Slice(Logfiles, func(i, j int) bool {
		return Logfiles[i] > Logfiles[j]
	})

	// 删除7天前的日志
	if len(Logfiles) > 7 {
		for _, file := range Logfiles[7:] {
			if err := os.Remove(filepath.Join(LOG_DIR, file)); err != nil {
				return err
			}
		}
	}

	return nil
}

// 写日志的辅助函数
func Logf(format string, v ...interface{}) {
	// 确保日志轮转正常
	if err := logManager.rotateLog(); err != nil {
		log.Printf("轮转日志失败: %v", err)
		return
	}

	// 格式化消息
	msg := fmt.Sprintf(format, v...)

	// 写入日志文件
	logManager.currentLogger.Output(2, msg)

	// 同时输出到控制台
	fmt.Printf("%s %s\n",
		time.Now().Format("2006/01/02 15:04:05"),
		msg,
	)
}

// LogStartupInfo 记录启动信息
func LogStartupInfo() {
	currentTime := time.Now()
	Logf("系统当前时间: %v", currentTime.Format("2006-01-02 15:04:05"))
	Logf("Unix时间戳: %d", currentTime.Unix())
}
