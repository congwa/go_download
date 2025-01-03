package updater

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateHeaders 生成请求头
func GenerateHeaders() (*Headers, error) {
	currentTime := time.Now()
	timestamp := fmt.Sprintf("%d", currentTime.Unix())

	// 生成签名
	content := SECRET_KEY + timestamp
	hash := md5.Sum([]byte(content))
	sign := hex.EncodeToString(hash[:])

	// 打印加密后的日志
	Logf("签名内容(加密): %s", encryptLogContent(content))
	Logf("生成的签名(加密): %s", encryptLogContent(sign))

	return &Headers{
		Timestamp: timestamp,
		Sign:      sign,
	}, nil
}

// encryptLogContent 加密日志内容
func encryptLogContent(content string) string {
	// 创建 AES 密码块
	block, err := aes.NewCipher([]byte(SECRET_KEY))
	if err != nil {
		return fmt.Sprintf("[ENCRYPT_ERROR:%v]", err)
	}

	// 对内容进行PKCS7填充
	padding := aes.BlockSize - len(content)%aes.BlockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	data := append([]byte(content), padtext...)

	// 使用全0的IV
	iv := make([]byte, aes.BlockSize)

	// 加密
	encrypted := make([]byte, len(data))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(encrypted, data)

	// 返回base64编码的结果
	return base64.StdEncoding.EncodeToString(encrypted)
}

// decryptAESCBC AES解密
func decryptAESCBC(encryptedData []byte) ([]byte, error) {
	if len(encryptedData) < aes.BlockSize {
		return nil, fmt.Errorf("密文太短")
	}

	// 从密文中提取IV（前16字节）
	iv := encryptedData[:aes.BlockSize]
	ciphertext := encryptedData[aes.BlockSize:]

	// 使用密钥创建cipher
	key := []byte(SECRET_KEY)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 解密
	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(ciphertext))
	mode.CryptBlocks(decrypted, ciphertext)

	// 去除PKCS7填充
	return removePKCS7Padding(decrypted)
}

// removePKCS7Padding 移除PKCS7填充
func removePKCS7Padding(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("数据长度为0")
	}

	padding := int(data[length-1])
	if padding > aes.BlockSize || padding == 0 {
		return nil, fmt.Errorf("无效的填充")
	}

	for i := length - padding; i < length; i++ {
		if data[i] != byte(padding) {
			return nil, fmt.Errorf("填充无效")
		}
	}

	return data[:length-padding], nil
}
