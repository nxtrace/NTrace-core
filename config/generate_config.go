package config

import (
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func writeFile(content []byte) error {
	var err error
	var path string
	path, err = configFromUserHomeDir()
	if err != nil {
		path, err = configFromRunDir()
		if err != nil {
			return err
		}
	}

	if exist, _ := pathExists(path); !exist {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			return err
		}
	}

	if err = ioutil.WriteFile(path+"ntraceConfig.yml", []byte(content), 0644); err != nil {
		return err
	}

	return nil
}

func AutoGenerate() (*tracerConfig, error) {
	token := Token{
		LeoMoeAPI: "NextTraceDemo",
		IPInfo:    "",
	}

	preference := Preference{
		AlwaysRoutePath:   false,
		TablePrintDefault: false,
		DataOrigin:        "LEOMOEAPI",
	}

	finalConfig := tracerConfig{
		Token:      token,
		Preference: preference,
	}

	yamlData, err := yaml.Marshal(&finalConfig)

	if err != nil {
		return nil, err
	}

	if err = writeFile(yamlData); err != nil {
		return nil, err
	} else {
		return &finalConfig, nil
	}
}

func Generate() (*tracerConfig, error) {
	var leotoken string
	var iPInfoToken string
	var tmpInput string

	fmt.Println("欢迎使用高阶自定义功能，这是一个配置向导，我们会帮助您生成配置文件。您的配置文件会被放在 ~/.nexttrace/ntraceConfig.yml 中，您也可以通过编辑这个文件来自定义配置。")

	fmt.Println("请输入您的LeoMoeAPI Token，您可以回车，以便继续使用公共Token")
	fmt.Scanln(&leotoken)
	if leotoken == "" {
		fmt.Println("检测到您的输入为空，您将使用公共Token。")
		leotoken = "NextTraceDemo"
	}

	fmt.Println("请输入您的IPInfo Token，如果您不需要使用IPInfo，可以直接回车")
	fmt.Scanln(&iPInfoToken)

	token := Token{
		LeoMoeAPI: leotoken,
		IPInfo:    iPInfoToken,
	}

	var preference Preference
	var AlwaysRoutePath bool
	var tablePrintDefault bool
	var dataOrigin string
	fmt.Print("我希望默认在路由跟踪完毕后，不绘制Route-Path图 (y/n) [y]")
	fmt.Scanln(&tmpInput)
	if tmpInput == "n" || tmpInput == "N" || tmpInput == "no" || tmpInput == "No" || tmpInput == "NO" {
		AlwaysRoutePath = true
	} else {
		AlwaysRoutePath = false
	}

	fmt.Print("我希望路由跟踪默认实时显示，而不使用制表模式 (y/n) [y]")
	fmt.Scanln(&tmpInput)
	if tmpInput == "n" || tmpInput == "N" || tmpInput == "no" || tmpInput == "No" || tmpInput == "NO" {
		tablePrintDefault = true
	} else {
		tablePrintDefault = false
	}

	fmt.Println("请选择默认的IP地理位置API数据源：\n1. LeoMoe\n2. IPInfo\n3. IPInsight\n4. IP.SB\n5. IP-API.COM")
	fmt.Print("请输入您的选择：")
	fmt.Scanln(&tmpInput)
	switch tmpInput {
	case "1":
		dataOrigin = "LEOMOEAPI"
	case "2":
		dataOrigin = "IPINFO"
	case "3":
		dataOrigin = "IPINSIGHT"
	case "4":
		dataOrigin = "IP.SB"
	case "5":
		dataOrigin = "IPAPI.COM"
	default:
		dataOrigin = "LEOMOEAPI"
	}

	preference = Preference{
		AlwaysRoutePath:   AlwaysRoutePath,
		TablePrintDefault: tablePrintDefault,
		DataOrigin:        dataOrigin,
	}

	finalConfig := tracerConfig{
		Token:      token,
		Preference: preference,
	}

	yamlData, err := yaml.Marshal(&finalConfig)

	if err != nil {
		return nil, err
	}

	if err = writeFile(yamlData); err != nil {
		return nil, err
	} else {
		fmt.Println("配置文件已经更新，在下次路由跟踪时，将会使用您的偏好。")
		return &finalConfig, nil
	}
}
