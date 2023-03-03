package config

import (
	"fmt"

	"github.com/spf13/viper"
)

func InitConfig() {

	// 配置文件名， 不加扩展
	viper.SetConfigName("nt_config") // name of config file (without extension)
	// 设置文件的扩展名
	viper.SetConfigType("yaml") // REQUIRED if the config file does not have the extension in the name
	// 查找配置文件所在路径
	viper.AddConfigPath("/etc/bin/nexttrace/")
	viper.AddConfigPath("/usr/local/bin/nexttrace/")
	// 在当前路径进行查找
	viper.AddConfigPath(".")
	// viper.AddConfigPath("./config/")

	// 配置默认值
	viper.SetDefault("ptrPath", "./ptr.csv")
	viper.SetDefault("geoFeedPath", "./geofeed.csv")

	// 开始查找并读取配置文件
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		fmt.Println("未能找到配置文件，我们将在您的运行目录为您创建 nt_config.yaml 默认配置")
		err := viper.SafeWriteConfigAs("./nt_config.yaml")
		if err != nil {
			return
		}
	}

	err = viper.ReadInConfig()
	if err != nil {
		return
	}
}
