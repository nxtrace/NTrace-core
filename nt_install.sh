#!/bin/bash

auto=False
#是否忽略一切警告，按默认执行
if [[ $1 == "--auto" ]]; then
    auto=True
fi

usrPath="/usr/local/bin"

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

    if [[ $arch == "arm64" ]]; then
        archParam="arm64"
    fi

    if [[ $archParam == "" ]]; then
        echo "未知的系统架构，请联系作者"
        exit 1
    fi
}

checkSystemDistribution() {
    case "$OSTYPE" in
    darwin*)
        osDistribution="darwin"
        downPath="/var/tmp/nexttrace"
        ;;
    linux*)
        osDistribution="linux"
        downPath="/var/tmp/nexttrace"
        ;;
    *)
        echo "unknown: $OSTYPE"
        exit 1
        ;;
    esac
}

getLocation() {
    echo "正在获取地理位置信息..."
    countryCode=$(curl -s "http://ip-api.com/line/?fields=countryCode")
}

installWgetPackage() {
    echo "wget 正在安装中..."
    # try apt
    # 是时候直接使用 APT 来管理包了
    apt-get -h &>/dev/null
    if [ $? -eq 0 ]; then
        # 先更新一下数据源，有些机器数据源比较老可能会404
        apt-get update -y &>/dev/null
        apt-get --no-install-recommends install wget -y &>/dev/null
        return 0
    fi

    # try yum
    yum -h &>/dev/null
    if [ $? -eq 0 ]; then
        yum -y update &>/dev/null
        yum install wget -y &>/dev/null
        return 0
    fi

    # try dnf
    dnf -h &>/dev/null
    if [ $? -eq 0 ]; then
        dnf check-update &>/dev/null
        dnf install wget -y &>/dev/null
        return 0
    fi

    # try pacman
    pacman -h &>/dev/null
    if [ $? -eq 0 ]; then
        pacman -Sy &>/dev/null
        pacman -S wget &>/dev/null
        return 0
    fi

    # try zypper
    zypper -h &>/dev/null
    if [ $? -eq 0 ]; then
        zypper refresh &>/dev/null
        zypper install -y --no-recommends wget &>/dev/null
        return 0
    fi

    # try brew
    brew -v &>/dev/null
    if [ $? -eq 0 ]; then
        brew update &>/dev/null
        brew install wget &>/dev/null
        return 0
    fi

    # 有的发行版自带的wget，只有 --help 参数
    wget --help &>/dev/null
    if [ $? -ne 0 ]; then
        echo "wget 安装失败"
        exit 1
    fi
}

installJqPackage() {
    echo "jq 正在安装中...(此步骤时间可能较长，请耐心等待)"
    # try apt
    apt-get -h &>/dev/null
    if [ $? -eq 0 ]; then
        # 先更新一下数据源，有些机器数据源比较老可能会404
        apt-get update -y &>/dev/null
        apt-get --no-install-recommends install jq -y &>/dev/null
        return 0
    fi

    # try yum
    yum -h &>/dev/null
    if [ $? -eq 0 ]; then
        yum -y update &>/dev/null
        yum install jq -y &>/dev/null
        return 0
    fi

    # try dnf
    dnf -h &>/dev/null
    if [ $? -eq 0 ]; then
        dnf check-update &>/dev/null
        dnf install jq -y &>/dev/null
        return 0
    fi

    # try zypper
    zypper -h &>/dev/null
    if [ $? -eq 0 ]; then
        zypper refresh &>/dev/null
        zypper install -y --no-recommends jq &>/dev/null
        return 0
    fi

    # try pacman
    pacman -h &>/dev/null
    if [ $? -eq 0 ]; then
        pacman -Sy &>/dev/null
        pacman -S jq &>/dev/null
        return 0
    fi

    # try brew
    brew -v &>/dev/null
    if [ $? -eq 0 ]; then
        brew update &>/dev/null
        brew install jq &>/dev/null
        return 0
    fi

    jq -h &>/dev/null
    if [ $? -ne 0 ]; then
        echo "jq 安装失败"
        exit 1
    fi
}

checkWgetPackage() {
    wget -h &>/dev/null
    if [ $? -ne 0 ]; then
        if [[ $auto == True ]]; then
            installWgetPackage
            return 0
        fi
        read -r -p "您还没有安装wget，是否安装? (y/n)" input

        case $input in
        [yY][eE][sS] | [yY])
            installWgetPackage
            ;;

        [nN][oO] | [nN])
            echo "您选择了取消安装，脚本即将退出"
            exit 1
            ;;

        *)
            installWgetPackage
            ;;
        esac
    fi
}

checkJqPackage() {
    jq -h &>/dev/null
    if [ $? -ne 0 ]; then
        if [[ $auto == True ]]; then
            installJqPackage
            return 0
        fi
        read -r -p "您还没有安装jq，是否安装? (y/n)" input

        case $input in
        [yY][eE][sS] | [yY])
            installJqPackage
            ;;

        [nN][oO] | [nN])
            echo "您选择了取消安装，脚本即将退出"
            exit 1
            ;;

        *)
            installJqPackage
            ;;
        esac
    fi
    return 1
}

checkVersion() {
    checkJqPackage
    echo "正在检查版本..."
    version=$(curl -sL https://api.github.com/repos/xgadget-lab/nexttrace/releases/latest | jq -r '.tag_name')
    if [[ $version == "" ]]; then
        echo "获取版本失败，请检查网络连接"
        exit 1
    fi
    currentVersion=$(nexttrace -V | head -n 1 | awk '{print $2}') &> /dev/null
    if [[ $currentVersion == $version ]]; then
        echo "当前版本已是最新版本"
        exit 0
    fi
    echo 当前最新release版本：${version}
    echo 您当前的版本：${currentVersion}
    if [[ $auto == True ]]; then
        return 0
    fi
    read -r -p "是否安装/更新软件? (y/n)" input
    case $input in
    [yY][eE][sS] | [yY])
        return 0
        ;;
    [nN][oO] | [nN])
        echo "您选择了取消安装/更新，脚本即将退出"
        exit 1
        ;;
    *)
        return 0
        ;;
    esac
}

downloadBinrayFile() {
    echo "正在获取最新版的 NextTrace 发行版文件信息..."
    checkJqPackage
    # 简单说明一下，Github提供了一个API，可以获取最新发行版本的二进制文件下载地址（对应的是browser_download_url），根据刚刚测得的osDistribution、archParam，获取对应的下载地址
    if [[ $? -eq 1 ]]; then
        # 支持 jq 不回退
        # echo nexttrace_${osDistribution}_${archParam}
        latestURL=$(curl -s https://api.github.com/repos/xgadget-lab/nexttrace/releases/latest | jq ".assets[] | select(.name == \"nexttrace_${osDistribution}_${archParam}\") | .browser_download_url")
        latestURL=${latestURL:1:-1}
    else
        # 不支持 jq，用户拒绝安装，回退 awk
        latestURL=$(curl -s https://api.github.com/repos/xgadget-lab/nexttrace/releases/latest | grep -i "browser_download_url.*${osDistribution}.*${archParam}" | awk -F '"' '{print $4}')
    fi
    if [ "$countryCode" == "CN" ]; then
        if [[ $auto == True ]]; then
            latestURL="https://ghproxy.com/"$latestURL
        else
            read -r -p "检测到国内网络环境，是否使用镜像下载以加速(y/n)" input
            case $input in
            [yY][eE][sS] | [yY])
                latestURL="https://ghproxy.com/"$latestURL
                ;;

            [nN][oO] | [nN])
                echo "您选择了不使用镜像，下载可能会变得异常缓慢，或者失败"
                ;;

            *)
                latestURL="https://ghproxy.com/"$latestURL
                ;;
            esac
        fi
    fi

    echo "正在下载 NextTrace 二进制文件..."
    wget -O ${downPath} ${latestURL} &>/dev/null
    if [ $? -eq 0 ]; then
        echo "NextTrace 现在已经在您的系统中可用"
        changeMode
        mv ${downPath} ${usrPath}
        if [[ ${osDistribution} == "darwin" ]]; then
            xattr -r -d com.apple.quarantine ${usrPath}/nexttrace
        fi
    else
        echo "NextTrace 下载失败，请检查您的网络是否正常"
        exit 1
    fi
}

changeMode() {
    chmod +x ${downPath} &>/dev/null
}

runBinrayFileHelp() {
    if [ -e ${usrPath} ]; then
        ${usrPath}/nexttrace -h
    fi
}

addCronTask() {
    if [[ $auto == True ]]; then
        return 0
    fi
    read -r -p "是否添加自动更新任务？(y/n)" input
    case $input in
    [yY][eE][sS] | [yY])
        if [[ ${osDistribution} == "darwin" ]]; then
            crontab -l >crontab.bak
            sed -i '' '/nt_install.sh/d' crontab.bak
        elif [[ ${osDistribution} == "linux" ]]; then
            crontab -l >crontab.bak
            sed -i '/nt_install.sh/d' crontab.bak
        else
            echo "暂不支持您的系统,无法自动添加crontab任务"
            return 0
        fi
        echo "1 1 * * * $(dirname $(readlink -f "$0"))/nt_install.sh --auto >> /var/log/nt_install.log" >>crontab.bak
        crontab crontab.bak
        rm -f crontab.bak
        ;;
    [nN][oO] | [nN])
        echo "您选择了不添加自动更新任务，您也可以通过命令 再次执行此脚本 手动更新"
        ;;
    *)
        echo "您选择了不添加自动更新任务，您可以通过命令 再次执行此脚本 手动更新"
        ;;
    esac
}

# Check Procedure
checkRootPermit
checkSystemDistribution
checkSystemArch
checkWgetPackage
checkJqPackage
checkVersion

# Download Procedure
getLocation
downloadBinrayFile

# Run Procedure
runBinrayFileHelp
addCronTask
