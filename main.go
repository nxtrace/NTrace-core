package main

import (
	"traceroute/methods"
	"traceroute/methods/tcp"
	"traceroute/methods/udp"
    "os"
	"net"
	"time"
    "fmt"
    "net/http"
    "io/ioutil"
    "encoding/json"
    "strconv"
    "flag"
    "strings"
)

type IPGeoData struct {
	Asnumber string `json:"asnumber"`
	Country string `json:"country"`
	Prov string `json:"prov"`
	City string `json:"city"`
	District string `json:"district"`
	Owner string `json:"owner"`
	Isp string `json:"isp"`
}

type IPInSightData struct {
	IP              string `json:"ip"`
	Version         string `json:"version"`
	IsEuropeanUnion bool   `json:"is_european_union"`
	ContinentCode   string `json:"continent_code"`
	IddCode         string `json:"idd_code"`
	CountryCode     string `json:"country_code"`
	CountryName     string `json:"country_name"`
	RegionName      string `json:"region_name"`
	CityName        string `json:"city_name"`
	Latitude        float64    `json:"latitude"`
	Longitude       float64    `json:"longitude"`
}

type IPSBData struct {
	Organization    string  `json:"organization"`
	Longitude       float64 `json:"longitude"`
	City            string  `json:"city"`
	Timezone        string  `json:"timezone"`
	Isp             string  `json:"isp"`
	Offset          int     `json:"offset"`
	Region          string  `json:"region"`
	Asn             int     `json:"asn"`
	AsnOrganization string  `json:"asn_organization"`
	Country         string  `json:"country"`
	IP              string  `json:"ip"`
	Latitude        float64 `json:"latitude"`
	PostalCode      string  `json:"postal_code"`
	ContinentCode   string  `json:"continent_code"`
	CountryCode     string  `json:"country_code"`
	RegionCode      string  `json:"region_code"`
}

type IPInfoData struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Loc      string `json:"loc"`
	Org      string `json:"org"`
	Postal   string `json:"postal"`
	Timezone string `json:"timezone"`
}

var tcpSYNFlag = flag.Bool("T", false, "Use TCP SYN for tracerouting (default port is 80 in TCP, 53 in UDP)")
var port = flag.Int("p", 80, "Set SYN Traceroute Port")
var numMeasurements = flag.Int("q", 3, "Set the number of probes per each hop.")
var parallelRequests = flag.Int("r", 18, "Set ParallelRequests number. It should be 1 when there is a multi-routing.")
var maxHops = flag.Int("m", 30, "Set the max number of hops (max TTL to be reached).")
var dataOrigin = flag.String("d", "LeoMoeAPI", "Choose IP Geograph Data Provider [LeoMoeAPI, IP.SB, IPInfo, IPInsight]")


func main() {
    fmt.Println("BetterTrace v0.0.4 Alpha \nOwO Organiztion Leo (leo.moe) & Vincent (vincent.moe)")
    ip := domainLookUp(flagApply())

    fmt.Println("IP Geo Data Provider: " + *dataOrigin)

    if (ip.String() == flagApply()) {
        fmt.Printf("traceroute to %s, 30 hops max, 32 byte packets\n", ip.String())
    } else {
        fmt.Printf("traceroute to %s (%s), 30 hops max, 32 byte packets\n", ip.String(), flagApply())
    }
    

    if (*tcpSYNFlag) {
        tcpTraceroute := tcp.New(ip, methods.TracerouteConfig{
            MaxHops:          uint16(*maxHops),
            NumMeasurements:  uint16(*numMeasurements),
            ParallelRequests: uint16(*parallelRequests),
            Port:             *port,
            Timeout:          time.Second / 2,
        })
        res, _ := tcpTraceroute.Start()

        traceroutePrinter(ip, res)
    } else {
        if (*port == 80) {
            *port = 53
        }
        udpTraceroute := udp.New(ip, true, methods.TracerouteConfig{
            MaxHops:          uint16(*maxHops),
            NumMeasurements:  uint16(*numMeasurements),
            ParallelRequests: uint16(*parallelRequests),
            Port:             *port,
            Timeout:          2 * time.Second,
        })
        res, _ := udpTraceroute.Start()

        traceroutePrinter(ip, res)
    }
}

func traceroutePrinter(ip net.IP, res *map[uint16][]methods.TracerouteHop) {
    hopIndex := uint16(1)
    for ; hopIndex <= 29 ;  {
        for k,v := range *res {
            if (k == hopIndex) {
                fmt.Print(k)
                for _,v2 := range v {
                    ch := make(chan uint16)
                    go hopPrinter(hopIndex, ip, v2, ch)
                    hopIndex = <- ch
                }
                hopIndex = hopIndex + 1
                break
            }
        }
    }
}

func flagApply() string{
    flag.Parse()
    ipArg := flag.Args()
    if (flag.NArg() != 1) {
        fmt.Println("Args Error\nUsage : ./bettertrace [-T] [-d <dataOrigin> ] [ -m <hops> ] [ -p <port> ] [ -q <probes> ] [ -r <parallelrequests> ] <hostname>")
        os.Exit(2)
    }
    return ipArg[0]
}

func getIPGeoByIPInfo(ip string, c chan IPGeoData) {
    
    resp, err := http.Get("https://ipinfo.io/" + ip + "?token=42764a944dabd0")
    if err != nil {
        fmt.Println(err)
    }
    defer resp.Body.Close()
    body, _ := ioutil.ReadAll(resp.Body)
    
    iPInfoData := &IPInfoData{}
    err = json.Unmarshal(body,&iPInfoData)
    
    if err != nil {
        fmt.Println(err)
    }

    ipGeoData := IPGeoData{
        Country: iPInfoData.Country,
        City: iPInfoData.City,
        Prov: iPInfoData.Region}

    c <- ipGeoData
}

func getIPGeoByIPSB(ip string, c chan IPGeoData) {
    resp, err := http.Get("https://api.ip.sb/geoip/" + ip)
    if err != nil {
        fmt.Println(err)
    }
    defer resp.Body.Close()
    body, _ := ioutil.ReadAll(resp.Body)
    
    iPSBData := &IPSBData{}
    err = json.Unmarshal(body,&iPSBData)
    
    if err != nil {
        fmt.Println("您当前出口IP被IP.SB视为风控IP，请求被拒绝")
        c <- IPGeoData{}
    }

    ipGeoData := IPGeoData{
        Asnumber: strconv.Itoa(iPSBData.Asn),
        Isp: iPSBData.Isp,
        Country: iPSBData.Country,
        City: iPSBData.City,
        Prov: iPSBData.Region}

    c <- ipGeoData
}

func getIPGeoByIPInsight(ip string, c chan IPGeoData) {
    
    resp, err := http.Get("https://ipinsight.io/query?ip=" + ip)
    if err != nil {
        fmt.Println(err)
    }
    defer resp.Body.Close()
    body, _ := ioutil.ReadAll(resp.Body)
    
    iPInSightData := &IPInSightData{}
    err = json.Unmarshal(body,&iPInSightData)
    
    if err != nil {
        fmt.Println(err)
    }

    ipGeoData := IPGeoData{
        Country: iPInSightData.CountryName,
        City: iPInSightData.CityName,
        Prov: iPInSightData.RegionName}

    c <- ipGeoData
}

func getIPGeo(ip string, c chan IPGeoData) {
    resp, err := http.Get("https://api.leo.moe/ip/?ip=" + ip)
    if err != nil {
        fmt.Println(err)
    }
    defer resp.Body.Close()
    body, _ := ioutil.ReadAll(resp.Body)
    
    ipGeoData := IPGeoData{}
    err = json.Unmarshal(body,&ipGeoData)
    
    if err != nil {
        fmt.Println(err)
    }
    c <- ipGeoData
}

func domainLookUp(host string) net.IP {
    ips, err := net.LookupIP(host)
    if (err != nil) {
        fmt.Println("Domain" + host + "Lookup Fail.")
        os.Exit(1)
    }
    
    var ipSlice = []net.IP{}

    for _, ip := range ips {
        ipSlice = append(ipSlice, ip)
    }
    if (len(ipSlice) == 1) {
        return ipSlice[0]
    } else {
        fmt.Println("Please Choose the IP You Want To TraceRoute")
        for i, ip := range ipSlice {
            fmt.Printf("%d. %s\n",i, ip)
        }
        var index int
        fmt.Printf("Your Option: ")
        fmt.Scanln(&index)
        if (index >= len(ipSlice) || index < 0) {
            fmt.Println("Your Option is invalid")
            os.Exit(3)
        }
        return ipSlice[index]
    }
}

func hopPrinter(hopIndex uint16, ip net.IP, v2 methods.TracerouteHop, c chan uint16) {
    if (v2.Address == nil) {
        fmt.Println("\t*")
    } else {
        ip_str := fmt.Sprintf("%s", v2.Address)

        ptr, err := net.LookupAddr(ip_str)

        ch_b := make(chan IPGeoData)

        if (*dataOrigin == "LeoMoeAPI") {
            go getIPGeo(ip_str, ch_b)

        } else if (*dataOrigin == "IP.SB") {
            go getIPGeoByIPSB(ip_str, ch_b)

        } else if (*dataOrigin == "IPInfo") {
            go getIPGeoByIPInfo(ip_str, ch_b)

        } else if (*dataOrigin == "IPInsight") {
            go getIPGeoByIPInsight(ip_str, ch_b)

        } else {
            go getIPGeo(ip_str, ch_b)
        }
        
        iPGeoData := <-ch_b

        if (ip.String() == ip_str) {
            hopIndex = 30
            iPGeoData.Owner = iPGeoData.Isp
        }

        if (strings.Index(ip_str, "9.31.") == 0 || strings.Index(ip_str, "11.72.") == 0) {
            fmt.Printf("\t%-15s %.2fms * 局域网, 腾讯云\n", v2.Address, v2.RTT.Seconds()*1000)
            c <- hopIndex
            return
        }

        if (strings.Index(ip_str, "11.13.") == 0) {
            fmt.Printf("\t%-15s %.2fms * 局域网, 阿里云\n", v2.Address, v2.RTT.Seconds()*1000)
            c <- hopIndex
            return
        }



        if (iPGeoData.Owner == "") {
            iPGeoData.Owner = iPGeoData.Isp
        }

        if (iPGeoData.Asnumber == "") {
            iPGeoData.Asnumber = "*"
        } else {
            iPGeoData.Asnumber = "AS" + iPGeoData.Asnumber
        }

        if (iPGeoData.District != "") {
            iPGeoData.City = iPGeoData.City + ", " + iPGeoData.District
        }

        if (iPGeoData.Country == "") {
            fmt.Printf("\t%-15s %.2fms * 局域网\n", v2.Address, v2.RTT.Seconds()*1000)
            c <- hopIndex
            return
        }

        if (iPGeoData.Prov == "" && iPGeoData.City == "") {

            if err != nil {
                fmt.Printf("\t%-15s %.2fms %s %s, %s, %s 骨干网\n",v2.Address, v2.RTT.Seconds()*1000, iPGeoData.Asnumber, iPGeoData.Country, iPGeoData.Owner, iPGeoData.Owner)
            } else {
                fmt.Printf("\t%-15s (%s) %.2fms %s %s, %s, %s 骨干网\n",ptr[0], v2.Address, v2.RTT.Seconds()*1000, iPGeoData.Asnumber, iPGeoData.Country, iPGeoData.Owner, iPGeoData.Owner) 
            }                            
        } else {

            if err != nil {
                fmt.Printf("\t%-15s %.2fms %s %s, %s, %s, %s\n",v2.Address, v2.RTT.Seconds()*1000, iPGeoData.Asnumber, iPGeoData.Country, iPGeoData.Prov, iPGeoData.City, iPGeoData.Owner)
            } else {
                fmt.Printf("\t%-15s (%s) %.2fms %s %s, %s, %s, %s\n",ptr[0], v2.Address, v2.RTT.Seconds()*1000, iPGeoData.Asnumber, iPGeoData.Country, iPGeoData.Prov, iPGeoData.City, iPGeoData.Owner) 
            }
        }
    }
    c <- hopIndex
}
