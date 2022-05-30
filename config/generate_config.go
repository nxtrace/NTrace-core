package config

import (
	"os"
	"io/ioutil"
	"fmt"
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

	if err = ioutil.WriteFile(path + "ntraceConfig.yml", []byte(content), 0644); err != nil{
		return err
	}

	return nil
}

func Generate() (*tracerConfig, error) {
	var leotoken string
	var iPInfoToken string
	var routePathEnable string
	
	fmt.Println("这是一个配置向导，我们会帮助您生成配置文件，它是一次性的，除非您主动要求重新生成，否则它将不会再出现")

	fmt.Println("请输入您的LeoMoeAPI Token，如果您没有，请到 Telegram Bot @NextTraceBot 获取一个")
	fmt.Scanln(&leotoken)
	if leotoken == "" {
		fmt.Println("检测到您的输入为空，您将使用公共Token。这意味着您将和所有使用此Token的客户端共用每分钟150次IP查询的额度")
		leotoken = "NextTraceDemo"
	}

	fmt.Println("请输入您的IPInfo Token，如果您不需要使用IPInfo，可以直接回车")
	fmt.Scanln(&iPInfoToken)

	token := Token{
		LeoMoeAPI: leotoken,
		IPInfo:    iPInfoToken,
	}

	var preference Preference
	fmt.Print("您是否希望在每次Traceroute结束后显示Route-Path图? (y/n)")
	fmt.Scanln(&routePathEnable)
	if routePathEnable == "n" || routePathEnable == "N" || routePathEnable == "no" || routePathEnable == "No" || routePathEnable == "NO" {
		preference = Preference{AlwaysRoutePath: false}
	} else {
		preference = Preference{AlwaysRoutePath: true}
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
		fmt.Println("配置文件创建成功")
		return &finalConfig, nil
	}
}
