#!/bin/bash

Green_font="\033[32m"
Yellow_font="\033[33m"
Red_font="\033[31m"
Font_suffix="\033[0m"
Info="${Green_font}[Info]${Font_suffix}"
Error="${Red_font}[Error]${Font_suffix}"
Tips="${Green_font}[Tips]${Font_suffix}"
Temp_path="/var/tmp/nexttrace"

checkRootPermit() {
    [[ $EUID -ne 0 ]] && echo -e "${Error} 请使用sudo/root权限运行本脚本" && exit 1
}

checkSystemArch() {
    arch=$(uname -m)
    if [[ $arch == "x86_64" ]]; then
    archParam="amd64"
    elif [[ $arch == "i386" ]]; then
    archParam="386"
    elif [[ $arch == "aarch64" ]]; then
    archParam="arm64"
    elif [[ $arch == "armv7l" ]] || [[ $arch == "armv7ml" ]]; then
    archParam="armv7"
    elif [[ $arch == "mips" ]]; then
    archParam="mips"
    fi
}

checkSystemDistribution() {
    case "$OSTYPE" in
    linux*)   
    osDistribution="linux"
    
    if [ ! -d "/usr/local" ]; 
    then 
    downPath="/usr/bin/nexttrace"
    else 
    downPath="/usr/local/bin/nexttrace"
    fi

    ;;
    *)
    echo "unknown: $OSTYPE"
    exit 1
    ;;
    esac
}

getLocation() {
    countryCode=$(curl -s "http://ip-api.com/line/?fields=countryCode")
}

installWgetPackage() {
    echo -e "${Info} wget 正在安装中..."
    # try apt
    apt -h &> /dev/null
    if [ $? -eq 0 ]; then
    # 先更新一下数据源，有些机器数据源比较老可能会404
    apt update -y &> /dev/null
    apt install wget -y &> /dev/null
    fi

    # try yum
    yum -h &> /dev/null
    if [ $? -eq 0 ]; then
    yum install wget -y &> /dev/null
    fi

    # try dnf
    dnf -h &> /dev/null
    if [ $? -eq 0 ]; then
    dnf install wget -y &> /dev/null
    fi

    # try pacman
    pacman -h &> /dev/null
    if [ $? -eq 0 ]; then
    pacman -Sy
    pacman -S wget
    fi

}

checkWgetPackage() {
    wget -h &> /dev/null
    if [ $? -ne 0 ]; then
		installWgetPackage
    fi
}

downloadBinrayFile() {
    echo -e "${Info} 获取最新版的 NextTrace 发行版文件信息"
    # 简单说明一下，Github提供了一个API，可以获取最新发行版本的二进制文件下载地址（对应的是browser_download_url），根据刚刚测得的osDistribution、archParam，获取对应的下载地址
    latestURL=$(curl -s https://api.github.com/repos/sjlleo/nexttrace/releases/latest | grep -i "browser_download_url.*${osDistribution}.*${archParam}" | awk -F '"' '{print $4}')
    
    if [ "$countryCode" == "CN" ]; then
        echo -e "${Info} 检测到国内环境，正在使用镜像下载"
        latestURL="https://ghproxy.com/"$latestURL
    fi
    
    echo -e "${Info} 正在下载 NextTrace 二进制文件..."
    wget -O ${Temp_path} ${latestURL} &> /dev/null
    if [ $? -eq 0 ];
    then
    changeMode
    mv ${Temp_path} ${downPath}
    echo -e "${Info} NextTrace 现在已经在您的系统中可用"
    else
    echo -e "${Error} NextTrace 下载失败，请检查您的网络是否正常"
    exit 1
    fi
}

changeMode() {
    chmod +x ${Temp_path} &> /dev/null
}

runBinrayFileHelp() {
    if [ -e ${downPath} ]; then
    ${downPath} --version
    echo -e "${Tips} 一切准备就绪！使用命令 nexttrace 1.1.1.1 开始您的第一次路由测试吧~ 更多进阶命令玩法可以用 nexttrace -h 查看哦\n       关于软件卸载，因为nexttrace是绿色版单文件，卸载只需输入命令 rm ${downPath} 即可"
    fi
}

# Check Procedure
checkRootPermit
checkSystemDistribution
checkSystemArch
checkWgetPackage

# Download Procedure
getLocation
downloadBinrayFile

# Run Procedure
runBinrayFileHelp
