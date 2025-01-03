package updater

// VersionInfo 版本信息结构体
type VersionInfo struct {
	Version     string `json:"version"`
	DownloadUrl string `json:"downloadUrl"`
	Amd64       string `json:"amd64"`
	Arm64       string `json:"arm64"`
	Arm         string `json:"arm"`
	Darwin      string `json:"darwin"`
}

// RequestHeaders 请求头结构体
type RequestHeaders struct {
	Timestamp string
	Sign      string
	UserAgent string
}

// Headers 请求头结构体
type Headers struct {
	Timestamp string
	Sign      string
}
