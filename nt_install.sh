#!/bin/bash

if [ "$1" = "http" ]; then
    protocol="http"
else
    protocol="https"
fi


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
    elif [[ $arch == "i686" ]]; then
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

downloadBinrayFile() {
    echo -e "${Info} 获取最新版的 NextTrace 发行版文件信息"
    for i in {1..3}; do
        downloadUrls=$(curl -sLf ${protocol}://www.nxtrace.org/api/dist/core/nexttrace_${osDistribution}_${archParam} --connect-timeout 1.5)
        if [ $? -eq 0 ]; then
            break
        fi
    done
    if [ $? -eq 0 ]; then
        primaryUrl=$(echo ${downloadUrls} | awk -F '|' '{print $1}')
        backupUrl=$(echo ${downloadUrls} | awk -F '|' '{print $2}')
        echo -e "${Info} 正在尝试从 Primary 节点下载 NextTrace"
        for i in {1..3}; do
            curl -sLf ${primaryUrl} -o ${Temp_path} --connect-timeout 1.5
            if [ $? -eq 0 ]; then
                changeMode
                mv ${Temp_path} ${downPath}
                echo -e "${Info} NextTrace 现在已经在您的系统中可用"
                return
            fi
        done
        if [ -z ${backupUrl} ]; then
            echo -e "${Error} 从 Primary 节点下载失败，且 Backup 节点为空，无法下载 NextTrace"
            exit 1
        fi
        echo -e "${Error} 从 Primary 节点下载失败，正在尝试从 Backup 节点下载 NextTrace"
        for i in {1..3}; do
            curl -sLf ${backupUrl} -o ${Temp_path} --connect-timeout 1.5
            if [ $? -eq 0 ]; then
                changeMode
                mv ${Temp_path} ${downPath}
                echo -e "${Info} NextTrace 现在已经在您的系统中可用"
                return
            fi
        done
        echo -e "${Error} NextTrace 下载失败，请检查您的网络是否正常"
        exit 1
    else
        echo -e "${Error} 获取下载地址失败，请检查您的网络是否正常"
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

# Download Procedure
downloadBinrayFile

# Run Procedure
runBinrayFileHelp

