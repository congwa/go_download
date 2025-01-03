package updater

import (
	"encoding/json"
	"os"
	"time"
)

const (
	CHECK_INTERVAL = 12 * time.Hour
	LOCAL_FILE     = "./allinone"
	VERSION_FILE   = "./version.txt"
	LOG_DIR        = "./logs"
	CONFIG_FILE    = "./config.json"
)

var (
	API_URL    string
	SECRET_KEY string
)

type Config struct {
	ApiUrl    string `json:"api_url"`
	SecretKey string `json:"secret_key"`
}

func init() {
	config := loadConfig()
	API_URL = config.ApiUrl
	SECRET_KEY = config.SecretKey
}

func loadConfig() Config {
	config := Config{
		// 默认值
		ApiUrl:    "",
		SecretKey: "",
	}

	data, err := os.ReadFile(CONFIG_FILE)
	if err != nil {
		return config
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config
	}

	return config
}
