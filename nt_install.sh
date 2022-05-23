#!/bin/bash

checkRootPermit() {
    [[ $EUID -ne 0 ]] && echo "请使用sudo/root权限运行本脚本" && exit 1
}

checkSystemArch() {
    arch=$(uname -m)
    if [[ $arch == "x86_64" ]]; then
    archParam="amd64"
    fi

    if [[ $arch == "aarch64" ]]; then
    archParam="arm64"
    fi
}

checkSystemDistribution() {
    case "$OSTYPE" in
    darwin*)  
    osDistribution="darwin"
    downPath="nexttrace"
    ;; 
    linux*)   
    osDistribution="linux"
    downPath="/usr/local/bin/nexttrace"
    ;;
    *)
    echo "unknown: $OSTYPE"
    exit 1
    ;;
    esac
}

installWgetPackage() {
    # macOS should install wget originally. Nothing to do
    echo "wget 正在安装中..."
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
    read -r -p "您还没有安装wget，是否安装? (y/n)" input

    case $input in
    [yY][eE][sS]|[yY])
		installWgetPackage
		;;

    [nN][oO]|[nN])
		echo "您选择了取消安装，脚本即将退出"
        exit 1
       	;;

    *)
		installWgetPackage
		;;
    esac
    fi
}

downloadBinrayFile() {
    echo "获取最新版的 NextTrace 发行版文件信息"
    # 简单说明一下，Github提供了一个API，可以获取最新发行版本的二进制文件下载地址（对应的是browser_download_url），根据刚刚测得的osDistribution、archParam，获取对应的下载地址
    latestURL=$(curl -s https://api.github.com/repos/xgadget-lab/nexttrace/releases/latest | grep -i "browser_download_url.*${osDistribution}.*${archParam}" | awk -F '"' '{print $4}')
    echo "正在下载 NextTrace 二进制文件..."
    wget -O ${downPath} ${latestURL} &> /dev/null
    if [ $? -eq 0 ];
    then
    echo "NextTrace 现在已经在您的系统中可用"
    changeMode
    else
    echo "NextTrace 下载失败，请检查您的网络是否正常"
    exit 1
    fi
}

changeMode() {
    chmod +x ./nexttrace &> /dev/null
    chmod +x ${downPath} &> /dev/null
}

runBinrayFileHelp() {
    if [ -e ${downPath} ]; then
    ${downPath} -h
    fi
}

checkSystemDistribution
checkSystemArch
checkWgetPackage
downloadBinrayFile
runBinrayFileHelp
