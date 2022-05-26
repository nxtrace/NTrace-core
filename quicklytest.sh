#!/bin/bash
PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:~/bin
export PATH
Green_font="\033[32m" && Red_font="\033[31m" && Font_suffix="\033[0m"
Info="${Green_font}[Info]${Font_suffix}"
Error="${Red_font}[Error]${Font_suffix}"
echo -e "${Green_font}
#======================================
# Project: NextTrace
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

check_mode() {
    echo -e "${Info} Nexttrace目前支持以下三种协议发起Traceroute请求:\n1.ICMP\n2.TCP(速度最快,但部分节点不支持)\n3.UDP\n(IPv6暂只支持ICMP模式)" && read -p "输入数字以选择:" node

    while [[ ! "${node}" =~ ^[1-3]$ ]]; do
        echo -e "${Error} 无效输入"
        echo -e "${Info} 请重新选择" && read -p "输入数字以选择:" node
    done

    [[ "${node}" == "1" ]] && TRACECMD="nexttrace"
    [[ "${node}" == "2" ]] && TRACECMD="nexttrace -T"
    [[ "${node}" == "3" ]] && TRACECMD="nexttrace -U"

    echo -e "${Info} 结果是否制表?(制表模式为非实时显示)" && read -r -p "输入y/n以选择模式:" input
    case $input in
            [yY][eE][sS] | [yY])
                TRACECMD=${TRACECMD}" -rdns -table"
                ;;
            [nN][oO] | [nN])
                TRACECMD=${TRACECMD}" -rdns -realtime"
                ;;
            *)
                TRACECMD=${TRACECMD}" -rdns -table"
                ;;
    esac
    
    return 0
    #未实现的功能:
    echo -e "${Info} 是否输出Route-Path?" && read -r -p "输入y/n以选择模式:" input
    case $input in
            [yY][eE][sS] | [yY])
                TRACECMD=${TRACECMD}" -report"
                ;;
            [nN][oO] | [nN])
                ;;
            *)
                TRACECMD=${TRACECMD}" -report"
                ;;
    esac
}

test_single() {
    echo -e "${Info} 请输入你要测试的目标 ip :"
    read -p "输入 ip 地址:" ip

    while [[ -z "${ip}" ]]; do
        echo -e "${Error} 无效输入"
        echo -e "${Info} 请重新输入" && read -p "输入 ip 地址:" ip
    done

    ${TRACECMD} ${ip} | grep -v -E 'NextTrace|XGadget-lab|Data\ Provider'

    repeat_test_single
}
repeat_test_single() {
    echo -e "${Info} 是否继续测试其他目标 ip ?"
    echo -e "1.是\n2.否"
    read -p "请选择:" whether_repeat_single
    while [[ ! "${whether_repeat_single}" =~ ^[1-2]$ ]]; do
        echo -e "${Error} 无效输入"
        echo -e "${Info} 请重新输入" && read -p "请选择:" whether_repeat_single
    done
    [[ "${whether_repeat_single}" == "1" ]] && test_single
    [[ "${whether_repeat_single}" == "2" ]] && echo -e "${Info} 退出脚本 ..." && exit 0
}

test_alternative() {
    select_alternative
    set_alternative
    result_alternative
}
select_alternative() {
    echo -e "${Info} 选择需要测速的目标网络: \n1.中国电信\n2.中国联通\n3.中国移动\n4.教育网"
    read -p "输入数字以选择:" ISP

    while [[ ! "${ISP}" =~ ^[1-4]$ ]]; do
        echo -e "${Error} 无效输入"
        echo -e "${Info} 请重新选择" && read -p "输入数字以选择:" ISP
    done
}
set_alternative() {
    [[ "${ISP}" == "1" ]] && node_1
    [[ "${ISP}" == "2" ]] && node_2
    [[ "${ISP}" == "3" ]] && node_3
    [[ "${ISP}" == "4" ]] && node_4
}
node_1() {
    echo -e "1.上海电信(天翼云)\n2.厦门电信CN2\n3.北京电信\n4.江苏电信\n5.广东深圳电信\n6.广州电信(天翼云)\n7.浙江电信" && read -p "输入数字以选择:" node

    while [[ ! "${node}" =~ ^[1-7]$ ]]; do
        echo -e "${Error} 无效输入"
        echo -e "${Info} 请重新选择" && read -p "输入数字以选择:" node
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
    echo -e "1.上海联通\n2.重庆联通\n3.北京联通\n4.安徽合肥联通\n5.江苏南京联通\n6.浙江杭州联通\n7.广东联通" && read -p "输入数字以选择:" node

    while [[ ! "${node}" =~ ^[1-7]$ ]]; do
        echo -e "${Error} 无效输入"
        echo -e "${Info} 请重新选择" && read -p "输入数字以选择:" node
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
    echo -e "1.上海移动\n2.四川成都移动\n3.北京移动\n4.浙江杭州移动\n5.广东移动\n6.江苏移动\n7.浙江移动" && read -p "输入数字以选择:" node

    while [[ ! "${node}" =~ ^[1-7]$ ]]; do
        echo -e "${Error} 无效输入"
        echo -e "${Info} 请重新选择" && read -p "输入数字以选择:" node
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
    echo -e "1.是\n2.否"
    read -p "请选择:" whether_repeat_alternative
    while [[ ! "${whether_repeat_alternative}" =~ ^[1-2]$ ]]; do
        echo -e "${Error} 无效输入"
        echo -e "${Info} 请重新输入" && read -p "请选择:" whether_repeat_alternative
    done
    [[ "${whether_repeat_alternative}" == "1" ]] && test_alternative
    [[ "${whether_repeat_alternative}" == "2" ]] && echo -e "${Info} 退出脚本 ..." && exit 0
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
    ${TRACECMD} $1 | grep -v -E 'NextTrace|XGadget-lab|Data\ Provider'
    echo -e "${Info} 测试路由 到 ${ISP_name} 完成 ！"
}

check_root
check_mode
echo -e "${Info} 选择你要使用的功能: "
echo -e "1.选择一个节点进行测试\n2.四网路由快速测试\n3.手动输入 ip 进行测试"
read -p "输入数字以选择:" function

while [[ ! "${function}" =~ ^[1-3]$ ]]; do
    echo -e "${Error} 缺少或无效输入"
    echo -e "${Info} 请重新选择" && read -p "输入数字以选择:" function
done

if [[ "${function}" == "1" ]]; then
    test_alternative
elif [[ "${function}" == "2" ]]; then
    test_all
else
    test_single
fi
