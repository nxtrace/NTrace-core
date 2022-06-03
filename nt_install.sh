#!/bin/bash

auto=False
#是否忽略一切警告，按默认执行
if [[ $1 == "--auto" ]]; then
  auto=True
  echo "自动运行中"
fi

usrPath="/usr/local/bin"

function red() {
  echo -e "\033[31m""${1}""\033[0m"
}

checkRootPermit() {
  [[ $EUID -ne 0 ]] && red "请使用sudo/root权限运行本脚本" && exit 1
}

checkSystemArch() {
  arch=$(uname -m)
  case $arch in
  'x86_64')
    archParam='amd64'
    ;;
  'mips')
    archParam='mips'
    ;;
  'arm64' | 'aarch64')
    archParam="arm64"
    ;;
  'armv7l')
    archParam='armv7'
    ;;
  'i386')
    archParam="386"
    ;;
  *)
    red "未知的系统架构，请联系开发者."
    exit 1
    ;;
  esac
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
    red "安装脚本暂不支持的操作系统: $OSTYPE"
    exit 1
    ;;
  esac
}

getLocation() {
  red "正在获取地理位置信息..."
  countryCode=$(curl -s "http://ip-api.com/line/?fields=countryCode")
  if [ "$countryCode" == "CN" ]; then
    if [[ $auto == True ]]; then
      URLprefix="https://ghproxy.com/"
    else
      read -r -p "检测到国内网络环境，是否使用镜像下载以加速(n/y)[y]" input
      case $input in
      [yY][eE][sS] | [yY])
        URLprefix="https://ghproxy.com/"
        ;;

      [nN][oO] | [nN])
        URLprefix=""
        red "您选择了不使用镜像，下载可能会变得异常缓慢，或者失败"
        ;;

      *)
        URLprefix="https://ghproxy.com/"
        ;;
      esac
    fi
  fi
}

ask_if() {
  local choice=""
  red "${1}"
  read -r choice
  [[ $choice == y ]] && return 0 || return 1
}

#检查脚本更新
check_script_update() {
  if [[ ${osDistribution} == "darwin" ]]; then
    [ "$(md5 <"${BASH_SOURCE[0]}")" == "$(curl -sL ${URLprefix}"https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/nt_install.sh" | md5)" ] && return 1 || return 0
  else
    [ "$(md5sum "${BASH_SOURCE[0]}" | awk '{print $1}')" == "$(md5sum <(curl -sL ${URLprefix}"https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/nt_install.sh") | awk '{print $1}')" ] && return 1 || return 0
  fi
}

#更新脚本
update_script() {
  if curl -sL -o "${BASH_SOURCE[0]}" ${URLprefix}"https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/nt_install.sh" || curl -sL -o "${BASH_SOURCE[0]}" ${URLprefix}"https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/nt_install.sh"; then
    red "nt_install.sh更新完成，正在重启脚本..."
    exec bash "${BASH_SOURCE[0]}" --auto
  else
    red "更新nt_install.sh失败！"
    exit 1
  fi
}

ask_update_script() {
  if check_script_update; then
    red "nt_install.sh可升级"
    [[ $auto == True ]] && update_script
    ask_if "是否升级脚本？(n/y)：[n]" && update_script
  else
    red "nt_install.sh已经是最新版本"
  fi
}

checkPackageManger() {
  if [[ ${osDistribution} == "darwin" ]]; then
    # brew update
    PACKAGE_MANAGEMENT_INSTALL='brew install'
    PACKAGE_MANAGEMENT_REMOVE='brew uninstall'
    return 0
  fi
  if [[ "$(which apt)" ]]; then
    apt-get update
    PACKAGE_MANAGEMENT_INSTALL='apt-get -y --no-install-recommends install'
    PACKAGE_MANAGEMENT_REMOVE='apt-get purge'
  elif [[ "$(which dnf)" ]]; then
    dnf check-update
    PACKAGE_MANAGEMENT_INSTALL='dnf -y install'
    PACKAGE_MANAGEMENT_REMOVE='dnf remove'
  elif [[ "$(which yum)" ]]; then
    PACKAGE_MANAGEMENT_INSTALL='yum -y install'
    PACKAGE_MANAGEMENT_REMOVE='yum remove'
  elif [[ "$(which zypper)" ]]; then
    zypper refresh
    PACKAGE_MANAGEMENT_INSTALL='zypper install -y --no-recommends'
    PACKAGE_MANAGEMENT_REMOVE='zypper remove'
  elif [[ "$(which pacman)" ]]; then
    PACKAGE_MANAGEMENT_INSTALL='pacman -Syu --noconfirm'
    PACKAGE_MANAGEMENT_REMOVE='pacman -Rsn'
  else
    red "error: The script does not support the package manager in this operating system."
    exit 1
  fi
}

install_software() {
  package_name="$1"
  which "$package_name" >/dev/null 2>&1 && return
  [[ ${osDistribution} == "darwin" ]] && echo -e "由于macOS brew的权限限制，请以非root权限执行下面一行提示的命令后再次运行本脚本(注意不要在该命令加sudo!):\nbrew update && ${PACKAGE_MANAGEMENT_INSTALL} $package_name " && exit 0
  red "${package_name} 正在安装中...(此步骤时间可能较长，请耐心等待)"
  if ${PACKAGE_MANAGEMENT_INSTALL} "$package_name"; then
    red "info: $package_name is installed."
  else
    red "error: Installation of $package_name failed, please check your network."
    exit 1
  fi
}

checkVersion() {
  which nexttrace >/dev/null 2>&1 || return
  red "正在检查版本..."
  version=$(curl -sL https://api.github.com/repos/xgadget-lab/nexttrace/releases/latest | jq -r '.tag_name')
  if [[ $version == "" ]]; then
    red "获取版本失败，请检查网络连接"
    exit 1
  fi
  currentVersion=$(nexttrace -V | head -n 1 | awk '{print $2}')
  if [[ $currentVersion == "$version" ]]; then
    red "当前版本已是最新版本"
    exit 0
  fi
  red "当前最新release版本：${version}"
  red "您当前的版本：${currentVersion}"
  if [[ $auto == True ]]; then
    return 0
  fi
  read -r -p "是否更新软件? (n/y)：[n]" input
  case $input in
  [yY][eE][sS] | [yY])
    return 0
    ;;
  [nN][oO] | [nN])
    red "您选择了取消更新，脚本即将退出"
    exit 1
    ;;
  *)
    red "您选择了取消更新，脚本即将退出"
    exit 1
    ;;
  esac
}

downloadBinrayFile() {
  red "正在获取最新版的 NextTrace 发行版文件信息..."
  # 简单说明一下，Github提供了一个API，可以获取最新发行版本的二进制文件下载地址（对应的是browser_download_url），根据刚刚测得的osDistribution、archParam，获取对应的下载地址
  # red nexttrace_${osDistribution}_${archParam}
  latestURL=$(curl -s https://api.github.com/repos/xgadget-lab/nexttrace/releases/latest | jq ".assets[] | select(.name == \"nexttrace_${osDistribution}_${archParam}\") | .browser_download_url")
  latestURL=${latestURL:1:$((${#latestURL} - 1 - 1))}
  # if [ "$countryCode" == "CN" ]; then
  #   if [[ $auto == True ]]; then
  #     latestURL="https://ghproxy.com/"$latestURL
  #   else
  #     read -r -p "检测到国内网络环境，是否使用镜像下载以加速(n/y)[y]" input
  #     case $input in
  #     [yY][eE][sS] | [yY])
  #       latestURL="https://ghproxy.com/"$latestURL
  #       ;;

  #     [nN][oO] | [nN])
  #       red "您选择了不使用镜像，下载可能会变得异常缓慢，或者失败"
  #       ;;

  #     *)
  #       latestURL="https://ghproxy.com/"$latestURL
  #       ;;
  #     esac
  #   fi
  # fi
  latestURL=$URLprefix$latestURL

  red "正在下载 NextTrace 二进制文件..."
  if wget -O ${downPath} "${latestURL}"; then
    changeMode
    mv ${downPath} ${usrPath}
    red "NextTrace 现在已经在您的系统中可用"
  else
    red "NextTrace 下载失败，请检查您的网络是否正常"
    exit 1
  fi
}

changeMode() {
  chmod +x ${downPath}
  [[ ${osDistribution} == "darwin" ]] && xattr -r -d com.apple.quarantine ${downPath}
}

runBinrayFileHelp() {
  if [ -e ${usrPath} ]; then
    ${usrPath}/nexttrace -h
  fi
  red "You may need to execute a command to remove dependent software: $PACKAGE_MANAGEMENT_REMOVE wget jq"
}

addCronTask() {
  read -r -p "是否添加自动更新任务？(n/y)：[n]" input
  case $input in
  [yY][eE][sS] | [yY])
    if [[ ${osDistribution} == "darwin" ]]; then
      crontab -l >crontab.bak 2>/dev/null
      sed -i '' '/nt_install.sh/d' crontab.bak
    elif [[ ${osDistribution} == "linux" ]]; then
      crontab -l >crontab.bak 2>/dev/null
      sed -i '/nt_install.sh/d' crontab.bak
    else
      red "暂不支持您的系统,无法自动添加crontab任务"
      return
    fi
    echo "1 1 * * * $(dirname "$(readlink -f "$0")")/nt_install.sh --auto >> /var/log/nt_install.log" >>crontab.bak
    crontab crontab.bak
    rm -f crontab.bak
    ;;
  [nN][oO] | [nN])
    red "您选择了不添加自动更新任务，您也可以通过命令 再次执行此脚本 手动更新"
    ;;
  *)
    red "您选择了不添加自动更新任务，您可以通过命令 再次执行此脚本 手动更新"
    ;;
  esac
}

# Check Procedure
checkRootPermit
checkSystemDistribution
checkSystemArch
getLocation
ask_update_script
checkPackageManger
install_software wget
install_software jq
checkVersion

# Download Procedure
downloadBinrayFile

# Run Procedure
runBinrayFileHelp
[[ $auto != True ]] && addCronTask
