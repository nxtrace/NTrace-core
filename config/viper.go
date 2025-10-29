package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

func InitConfig() {
	// 配置文件名， 不加扩展
	viper.SetConfigName("nt_config") // name of config file (without extension)
	// 设置文件的扩展名
	viper.SetConfigType("yaml") // REQUIRED if the config file does not have the extension in the name
	// 查找配置文件所在路径
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" && homeDir != "" {
		xdgConfigHome = filepath.Join(homeDir, ".config")
	}

	configPaths := []string{
		"/etc/nexttrace",
		"/usr/local/etc/nexttrace",
	}

	if runtime.GOOS == "darwin" {
		configPaths = append(configPaths, "/opt/homebrew/etc/nexttrace")
	}

	if xdgConfigHome != "" {
		configPaths = append(configPaths, filepath.Join(xdgConfigHome, "nexttrace"))
	}

	if homeDir != "" {
		configPaths = append(configPaths,
			filepath.Join(homeDir, ".local", "share", "nexttrace"),
			filepath.Join(homeDir, ".nexttrace"),
			filepath.Join(homeDir, "nexttrace"),
			homeDir,
		)
	}

	configPaths = append(configPaths,
		"/usr/share/nexttrace",
		"/usr/local/share/nexttrace",
		".",
	)

	for _, path := range configPaths {
		viper.AddConfigPath(path)
	}

	// 配置默认值
	viper.SetDefault("ptrPath", "./ptr.csv")
	viper.SetDefault("geoFeedPath", "./geofeed.csv")

	// 开始查找并读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			fmt.Println("未能找到配置文件，我们将在您的运行目录为您创建 nt_config.yaml 默认配置")
			if err := viper.SafeWriteConfigAs("./nt_config.yaml"); err != nil {
				fmt.Println("创建默认配置文件失败:", err)
				return
			}
			if err := viper.ReadInConfig(); err != nil {
				fmt.Println("加载默认配置失败:", err)
			}
			return
		}

		fmt.Println("加载配置文件失败:", err)
		return
	}
}
