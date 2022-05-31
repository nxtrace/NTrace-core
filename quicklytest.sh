#!/bin/bash

Green_font="\033[32m" && Red_font="\033[31m" && Font_suffix="\033[0m"
Info="${Green_font}[Info]${Font_suffix}"
Error="${Red_font}[Error]${Font_suffix}"
echo -e "${Green_font}
#======================================
# Project: NextTrace https://github.com/xgadget-lab/nexttrace
# Copyright Notice:
# This script is ported from @KANIKIG https://github.com/KANIKIG/
# The developer team made some modifications to adapt to NextTrace under the GPL-3.0 LICENSE
# NextTrace:
#   XGadget-lab Leo (leo.moe) & Vincent (vincent.moe) & zhshch (xzhsh.ch)
#   IP Geo Data Provider: LeoMoeAPI
#======================================
${Font_suffix}"

check_root() {
  [[ "$(id -u)" != "0" ]] && echo -e "${Error} must be root user !" && exit 1
}

checkNexttrace() {
  if $(which nexttrace >/dev/null 2>&1); then
    echo -e "${Info} 您已安装NextTrace，是否检查更新？"
    if ask_if "输入n/y以选择:[n]"; then
      echo -e "${Info} 正在检查更新..."
    else
      return
    fi
  else
    echo -e "${Info} 您未安装NextTrace，正在开始安装..."
    mkdir ~/.nexttrace/
    cat >~/.nexttrace/ntraceConfig.yml <<EOF
Token:
  LeoMoeAPI: NextTraceDemo
  IPInfo: ""
Preference:
  AlwaysRoutePath: true
EOF
  fi
  if curl -sL -O ${URLprefix}"https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/nt_install.sh" || curl -sL -O ${URLprefix}"https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/nt_install.sh"; then
    bash nt_install.sh #--auto #>/dev/null
  fi
}

getLocation() {
  echo -e "${Info} 正在获取地理位置信息..."
  countryCode=$(curl -s "http://ip-api.com/line/?fields=countryCode")
  if [ "$countryCode" == "CN" ]; then
    read -r -p "检测到国内网络环境，是否使用镜像下载以加速(n/y)[y]" input
    case $input in
    [yY][eE][sS] | [yY])
      URLprefix="https://ghproxy.com/"
      ;;

    [nN][oO] | [nN])
      URLprefix=""
      echo -e "${Info} 您选择了不使用镜像，下载可能会变得异常缓慢，或者失败"
      ;;

    *)
      URLprefix="https://ghproxy.com/"
      ;;
    esac
  fi
}

ask_if() {
  local choice=""
  echo -e "${Info} $1"
  read -r choice
  [[ $choice == y ]] && return 0
  return 1
}

checkSystemDistribution() {
  case "$OSTYPE" in
  darwin*)
    osDistribution="darwin"
    ;;
  linux*)
    osDistribution="linux"
    ;;
  *)
    echo -e "${Info} unknown: $OSTYPE"
    exit 1
    ;;
  esac
}

#检查脚本更新
check_script_update() {
  if [[ ${osDistribution} == "darwin" ]]; then
    [ "$(md5 <"${BASH_SOURCE[0]}")" == "$(curl -sL ${URLprefix}"https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/quicklytest.sh" | md5)" ] && return 1 || return 0
  else
    [ "$(md5sum "${BASH_SOURCE[0]}" | awk '{print $1}')" == "$(md5sum <(curl -sL ${URLprefix}"https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/quicklytest.sh") | awk '{print $1}')" ] && return 1 || return 0
  fi
}

#更新脚本
update_script() {
  if curl -sL -o "${BASH_SOURCE[0]}" ${URLprefix}"https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/quicklytest.sh" || curl -sL -o "${BASH_SOURCE[0]}" ${URLprefix}"https://raw.githubusercontent.com/xgadget-lab/nexttrace/main/quicklytest.sh"; then
    echo -e "${Info} quickylytest.sh更新完成，正在重启脚本..."
    exec bash "${BASH_SOURCE[0]}"
  else
    echo -e "${Info} 更新quickylytest.sh失败！"
    exit 1
  fi
}

ask_update_script() {
  if check_script_update; then
    echo -e "${Info} quickylytest.sh可升级"
    ask_if "是否升级脚本？(n/y)[n]" && update_script
  else
    echo -e "${Info} quickylytest.sh已经是最新版本"
  fi
}

check_mode() {
  echo -e "${Info} Nexttrace目前支持以下三种协议发起Traceroute请求:\n1.ICMP\n2.TCP(速度最快,但部分节点不支持)\n3.UDP\n(IPv6暂只支持ICMP模式)" && read -r -p "输入数字以选择:" node

  while [[ ! "${node}" =~ ^[1-3]$ ]]; do
    echo -e "${Error} 无效输入"
    echo -e "${Info} 请重新选择" && read -r -p "输入数字以选择:" node
  done

  [[ "${node}" == "1" ]] && TRACECMD="nexttrace"
  [[ "${node}" == "2" ]] && TRACECMD="nexttrace -T"
  [[ "${node}" == "3" ]] && TRACECMD="nexttrace -U"

  echo -e "${Info} 结果是否制表?(制表模式为非实时显示)"
  if ask_if "输入n/y以选择模式:[n]"; then
    TRACECMD=${TRACECMD}" -rdns -table"
    #        ##Route-Path功能还未完善,临时替代:
    #        [[ "${node}" == "2" ]] && TRACECMD=${TRACECMD}" -report"
    #        ##
  else
    TRACECMD=${TRACECMD}" -rdns"
    #        ##Route-Path功能还未完善,临时替代:
    #        [[ "${node}" == "1" ]] && TRACECMD=${TRACECMD}" -report"
    #        [[ "${node}" == "2" ]] && TRACECMD=${TRACECMD}" -report"
    #        ##
  fi

  # echo -e "${Info} 是否输出Route-Path?"
  # ask_if "输入n/y以选择模式:[n]" && TRACECMD=${TRACECMD}" -report"

}

test_single() {
  echo -e "${Info} 请输入你要测试的目标 ip :"
  read -r -p "输入 ip 地址:" ip

  while [[ -z "${ip}" ]]; do
    echo -e "${Error} 无效输入"
    echo -e "${Info} 请重新输入" && read -r -p "输入 ip 地址:" ip
  done

  ${TRACECMD} "${ip}" | grep -v -E 'NextTrace|XGadget-lab|Data\ Provider'

  repeat_test_single
}

repeat_test_single() {
  echo -e "${Info} 是否继续测试其他目标 ip ?"
  if ask_if "输入n/y以选择:[n]"; then
    test_single
  else
    echo -e "${Info} 退出脚本 ..." && exit 0
  fi
}

test_alternative() {
  select_alternative
  set_alternative
  result_alternative
}

select_alternative() {
  echo -e "${Info} 选择需要测速的目标网络: \n1.中国电信\n2.中国联通\n3.中国移动\n4.教育网"
  read -r -p "输入数字以选择:" ISP

  while [[ ! "${ISP}" =~ ^[1-4]$ ]]; do
    echo -e "${Error} 无效输入"
    echo -e "${Info} 请重新选择" && read -r -p "输入数字以选择:" ISP
  done
}

set_alternative() {
  [[ "${ISP}" == "1" ]] && node_1
  [[ "${ISP}" == "2" ]] && node_2
  [[ "${ISP}" == "3" ]] && node_3
  [[ "${ISP}" == "4" ]] && node_4
}

node_1() {
  echo -e "1.上海电信(天翼云)\n2.厦门电信CN2\n3.北京电信\n4.江苏电信\n5.广东深圳电信\n6.广州电信(天翼云)\n7.浙江电信" && read -r -p "输入数字以选择:" node

  while [[ ! "${node}" =~ ^[1-7]$ ]]; do
    echo -e "${Error} 无效输入"
    echo -e "${Info} 请重新选择" && read -r -p "输入数字以选择:" node
  done

  [[ "${node}" == "1" ]] && ISP_name="上海电信" && ip=101.89.132.9
  [[ "${node}" == "2" ]] && ISP_name="厦门电信CN2" && ip=117.28.254.129
  [[ "${node}" == "3" ]] && ISP_name="北京电信" && ip=120.92.180.135
  [[ "${node}" == "4" ]] && ISP_name="江苏电信" && ip=221.229.173.233
  [[ "${node}" == "5" ]] && ISP_name="广东深圳电信" && ip=116.6.211.41
  [[ "${node}" == "6" ]] && ISP_name="广州电信(天翼云)" && ip=14.215.116.1
  [[ "${node}" == "7" ]] && ISP_name="浙江电信" && ip=115.236.169.86
}

node_2() {
  echo -e "1.上海联通\n2.重庆联通\n3.北京联通\n4.安徽合肥联通\n5.江苏南京联通\n6.浙江杭州联通\n7.广东联通" && read -r -p "输入数字以选择:" node

  while [[ ! "${node}" =~ ^[1-7]$ ]]; do
    echo -e "${Error} 无效输入"
    echo -e "${Info} 请重新选择" && read -r -p "输入数字以选择:" node
  done

  [[ "${node}" == "1" ]] && ISP_name="上海联通" && ip=220.196.252.174
  [[ "${node}" == "2" ]] && ISP_name="重庆联通" && ip=113.207.32.65
  [[ "${node}" == "3" ]] && ISP_name="北京联通" && ip=202.106.54.150
  [[ "${node}" == "4" ]] && ISP_name="安徽合肥联通" && ip=112.122.10.26
  [[ "${node}" == "5" ]] && ISP_name="江苏联通" && ip=112.85.231.129
  [[ "${node}" == "6" ]] && ISP_name="浙江联通" && ip=60.12.214.156
  [[ "${node}" == "7" ]] && ISP_name="广东联通" && ip=58.252.2.194
}

node_3() {
  echo -e "1.上海移动\n2.四川成都移动\n3.北京移动\n4.浙江杭州移动\n5.广东移动\n6.江苏移动\n7.浙江移动" && read -r -p "输入数字以选择:" node

  while [[ ! "${node}" =~ ^[1-7]$ ]]; do
    echo -e "${Error} 无效输入"
    echo -e "${Info} 请重新选择" && read -r -p "输入数字以选择:" node
  done

  [[ "${node}" == "1" ]] && ISP_name="上海移动" && ip=117.184.42.114
  [[ "${node}" == "2" ]] && ISP_name="四川成都移动" && ip=183.221.247.9
  [[ "${node}" == "3" ]] && ISP_name="北京移动" && ip=111.13.217.125
  [[ "${node}" == "4" ]] && ISP_name="浙江移动" && ip=183.246.69.139
  [[ "${node}" == "5" ]] && ISP_name="广东移动" && ip=221.179.44.57
  [[ "${node}" == "6" ]] && ISP_name="江苏移动" && ip=120.195.6.129
  [[ "${node}" == "7" ]] && ISP_name="浙江移动" && ip=183.246.69.139
}

node_4() {
  ISP_name="北京教育网" && ip=211.68.69.240
}

result_alternative() {
  echo -e "${Info} 测试路由 到 ${ISP_name} 中 ..."
  ${TRACECMD} ${ip} | grep -v -E 'NextTrace|XGadget-lab|Data\ Provider'
  echo -e "${Info} 测试路由 到 ${ISP_name} 完成 ！"

  repeat_test_alternative
}

repeat_test_alternative() {
  echo -e "${Info} 是否继续测试其他节点?"
  if ask_if "输入n/y以选择:[n]"; then
    test_alternative
  else
    echo -e "${Info} 退出脚本 ..." && exit 0
  fi
}

test_all() {
  result_all '116.6.211.41' '广东东莞CN2'

  result_all '101.95.110.149' '上海电信'

  result_all '112.85.231.129' '江苏徐州联通'

  result_all '120.199.239.1' '浙江杭州移动'

  result_all '211.68.69.240' '北京教育网'

  echo -e "${Info} 四网路由快速测试 已完成 ！"
}

result_all() {
  ISP_name=$2
  echo -e "${Info} 测试路由 到 ${ISP_name} 中 ..."
  ${TRACECMD} "${1}" | grep -v -E 'NextTrace|XGadget-lab|Data\ Provider'
  echo -e "${Info} 测试路由 到 ${ISP_name} 完成 ！"
}

check_root
checkSystemDistribution
getLocation
ask_update_script
checkNexttrace
check_mode
echo -e "${Info} 选择你要使用的功能: "
echo -e "1.选择一个节点进行测试\n2.四网路由快速测试\n3.手动输入 ip 进行测试"
read -r -p "输入数字以选择:" function

while [[ ! "${function}" =~ ^[1-3]$ ]]; do
  echo -e "${Error} 缺少或无效输入"
  echo -e "${Info} 请重新选择" && read -r -p "输入数字以选择:" function
done

if [[ "${function}" == "1" ]]; then
  test_alternative
elif [[ "${function}" == "2" ]]; then
  test_all
else
  test_single
fi
