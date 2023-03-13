package fastTrace

type AllLocationCollection struct {
	Beijing   BackBoneCollection
	Shanghai  BackBoneCollection
	Guangzhou BackBoneCollection
	Hangzhou  BackBoneCollection
	Hefei     BackBoneCollection
	Changsha  BackBoneCollection
}

type BackBoneCollection struct {
	Location string
	CT163    ISPCollection
	CTCN2    ISPCollection
	CU169    ISPCollection
	CU9929   ISPCollection
	CM       ISPCollection
	CMIN2    ISPCollection
	EDU      ISPCollection
	CST      ISPCollection
}

type ISPCollection struct {
	ISPName string
	IP      string
	IPv6    string
}

const (
	CT163  string = "电信 163 AS4134"
	CTCN2  string = "电信 CN2 AS4809"
	CU169  string = "联通 169 AS4837"
	CU9929 string = "联通 A网 AS9929"
	CM     string = "移动 骨干网 AS9808"
	CMIN2  string = "移动 CMIN2 AS58807"
	EDU    string = "教育网 CERNET AS4538"
)

var TestIPsCollection = AllLocationCollection{
	Beijing:   Beijing,
	Shanghai:  Shanghai,
	Guangzhou: Guangzhou,
	Hangzhou:  Hangzhou,
	Hefei:     Hefei,
}

var Beijing = BackBoneCollection{
	Location: "北京",
	CT163: ISPCollection{
		ISPName: CT163,
		IP:      "106.37.67.1",
		IPv6:    "240e:40:e002:1:a:3ee3:c00:0",
	},

	CU169: ISPCollection{
		ISPName: CU169,
		IP:      "123.125.96.156",
		IPv6:    "2408:8000:1010:2::10",
	},

	CU9929: ISPCollection{
		ISPName: CU9929,
		IP:      "218.105.131.125",
	},

	CM: ISPCollection{
		ISPName: CM,
		IP:      "211.136.25.153",
		IPv6:    "2409:8000:3800:8::3",
	},

	CMIN2: ISPCollection{
		ISPName: CMIN2,
		IP:      "223.70.155.55",
	},

	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "101.6.15.130",
		IPv6:    "2001:da8:215:4078:250:56ff:fe97:654d",
	},
}

var Shanghai = BackBoneCollection{
	Location: "上海",
	CT163: ISPCollection{
		ISPName: CT163,
		IP:      "202.101.21.178",
		IPv6:    "240e:18:2:153::89",
	},

	CTCN2: ISPCollection{
		ISPName: CTCN2,
		IP:      "58.32.4.1",
	},

	CU169: ISPCollection{
		ISPName: CU169,
		IP:      "139.226.206.150",
		IPv6:    "2408:8000:9000:0:4000::437",
	},

	CU9929: ISPCollection{
		ISPName: CU9929,
		IP:      "210.13.86.1",
		IPv6:    "2408:8120:2::d6",
	},

	CM: ISPCollection{
		ISPName: CM,
		IP:      "120.204.34.85",
		IPv6:    "2409:801e:f0:1::4e1",
	},

	CMIN2: ISPCollection{
		ISPName: CMIN2,
		IP:      "183.194.134.1",
	},

	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "202.120.58.155",
		IPv6:    "2001:da8:8000:1:202:120:2:100",
	},
}

var Guangzhou = BackBoneCollection{
	Location: "广州",
	CT163: ISPCollection{
		ISPName: CT163,
		IP:      "14.116.225.60",
		IPv6:    "240e:f9:8010::3:110:1",
	},

	CU169: ISPCollection{
		ISPName: CU169,
		IP:      "157.18.0.22",
		IPv6:    "2408:8001:3161:4::1",
	},

	CM: ISPCollection{
		ISPName: CM,
		IP:      "120.198.26.254",
		IPv6:    "2409:8055:3008:1116::150",
	},
}

var Hangzhou = BackBoneCollection{
	Location: "杭州",
	CT163: ISPCollection{
		ISPName: CT163,
		IP:      "61.164.23.196",
		IPv6:    "240e:f3:c000:201::10",
	},
	CU169: ISPCollection{
		ISPName: CU169,
		IP:      "60.12.244.1",
		IPv6:    "",
	},
	CM: ISPCollection{
		ISPName: CM,
		IP:      "112.17.224.98",
		IPv6:    "2409:8028:840:2::11",
	},
	// 浙江大学 教育网
	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "210.32.2.1",
		IPv6:    "2001:da8:e000:1::1",
	},
}

var Hefei = BackBoneCollection{
	Location: "合肥",
	// 中国科学技术大学 教育网
	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "202.38.64.1",
		IPv6:    "2001:da8:d805:ffff:2::1",
	},
	// 中国科学技术大学 科技网
	CST: ISPCollection{
		ISPName: "中国科学技术大学 科技网 AS7497",
		IP:      "210.72.22.2",
	},
}
