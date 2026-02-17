package config

import (
	"time"

	"github.com/saltbo/zpan/internal/pkg/ldap"
	"github.com/saltbo/zpan/internal/pkg/logger"
	"github.com/saltbo/zpan/internal/pkg/provider"
	"github.com/spf13/viper"
)

// Initialize 在应用启动时调用，统一初始化所有通过 viper 读取的配置常量
// 此函数应在 Wire 依赖注入之前调用，以确保所有常量都被正确初始化
func Initialize() {
	// 初始化日志
	initLogger()

	// 初始化存储相关的预签名 URL 过期时间
	initStorageExpiration()

	// 初始化 LDAP 配置（如果启用）
	initLDAP()
}

// initStorageExpiration 初始化云存储预签名 URL 的过期时间
// 从配置中读取 storage.upload_expiration 和 storage.download_expiration
// 如果未配置，则使用默认值（上传 10 分钟，下载 5 分钟）
func initStorageExpiration() {
	uploadExp := viper.GetInt("storage.upload_expiration")
	if uploadExp == 0 {
		uploadExp = 600 // 默认 10 分钟
	}
	downloadExp := viper.GetInt("storage.download_expiration")
	if downloadExp == 0 {
		downloadExp = 300 // 默认 5 分钟
	}

	if uploadExp > 0 {
		provider.SetDefaultUploadExpiration(time.Duration(uploadExp) * time.Second)
	}
	if downloadExp > 0 {
		provider.SetDefaultDownloadExpiration(time.Duration(downloadExp) * time.Second)
	}
}

// initLogger 初始化日志系统
// 从配置中读取日志级别，若未配置则使用默认值 "info"
func initLogger() {
	logLevel := viper.GetString("loglevel")
	if logLevel == "" {
		logLevel = "info"
	}
	logger.Init(logLevel)
}

// initLDAP 初始化 LDAP 认证器（若启用）
// 该函数会验证 LDAP 配置的有效性
func initLDAP() {
	if viper.GetBool("ldap.enabled") {
		ldap.Init()
	}
}
