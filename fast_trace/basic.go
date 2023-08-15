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
		IP:      "ipv4.pek-4134.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.pek-4134.nexttrace-io-fasttrace-endpoint.win.",
	},

	CU169: ISPCollection{
		ISPName: CU169,
		IP:      "ipv4.pek-4837.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.pek-4837.nexttrace-io-fasttrace-endpoint.win.",
	},

	CU9929: ISPCollection{
		ISPName: CU9929,
		IP:      "ipv4.pek-9929.nexttrace-io-fasttrace-endpoint.win.",
	},

	CM: ISPCollection{
		ISPName: CM,
		IP:      "ipv4.pek-9808.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.pek-9808.nexttrace-io-fasttrace-endpoint.win.",
	},

	CMIN2: ISPCollection{
		ISPName: CMIN2,
		IP:      "ipv4.pek-58807.nexttrace-io-fasttrace-endpoint.win.",
	},

	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "ipv4.pek-4538.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.pek-4538.nexttrace-io-fasttrace-endpoint.win.",
	},
}

var Shanghai = BackBoneCollection{
	Location: "上海",
	CT163: ISPCollection{
		ISPName: CT163,
		IP:      "ipv4.sha-4134.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.sha-4134.nexttrace-io-fasttrace-endpoint.win.",
	},

	CTCN2: ISPCollection{
		ISPName: CTCN2,
		IP:      "ipv4.sha-4809.nexttrace-io-fasttrace-endpoint.win.",
	},

	CU169: ISPCollection{
		ISPName: CU169,
		IP:      "ipv4.sha-4837.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.sha-4837.nexttrace-io-fasttrace-endpoint.win.",
	},

	CU9929: ISPCollection{
		ISPName: CU9929,
		IP:      "ipv4.sha-9929.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.sha-9929.nexttrace-io-fasttrace-endpoint.win.",
	},

	CM: ISPCollection{
		ISPName: CM,
		IP:      "ipv4.sha-9808.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.sha-9808.nexttrace-io-fasttrace-endpoint.win.",
	},

	CMIN2: ISPCollection{
		ISPName: CMIN2,
		IP:      "ipv4.sha-58807.nexttrace-io-fasttrace-endpoint.win.",
	},

	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "ipv4.sha-4538.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.sha-4538.nexttrace-io-fasttrace-endpoint.win.",
	},
}

var Guangzhou = BackBoneCollection{
	Location: "广州",
	CT163: ISPCollection{
		ISPName: CT163,
		IP:      "ipv4.can-4134.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.can-4134.nexttrace-io-fasttrace-endpoint.win.",
	},

	CU169: ISPCollection{
		ISPName: CU169,
		IP:      "ipv4.can-4837.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.can-4837.nexttrace-io-fasttrace-endpoint.win.",
	},

	CM: ISPCollection{
		ISPName: CM,
		IP:      "ipv4.can-9808.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.can-9808.nexttrace-io-fasttrace-endpoint.win.",
	},
}

var Hangzhou = BackBoneCollection{
	Location: "杭州",
	CT163: ISPCollection{
		ISPName: CT163,
		IP:      "ipv4.hgh-4134.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.hgh-4134.nexttrace-io-fasttrace-endpoint.win.",
	},
	CU169: ISPCollection{
		ISPName: CU169,
		IP:      "ipv4.hgh-4837.nexttrace-io-fasttrace-endpoint.win.",
	},
	CM: ISPCollection{
		ISPName: CM,
		IP:      "ipv4.hgh-9808.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.hgh-9808.nexttrace-io-fasttrace-endpoint.win.",
	},
	// 浙江大学 教育网
	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "ipv4.hgh-4538.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.hgh-4538.nexttrace-io-fasttrace-endpoint.win.",
	},
}

var Hefei = BackBoneCollection{
	Location: "合肥",
	// 中国科学技术大学 教育网
	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "ipv4.hfe-4538.nexttrace-io-fasttrace-endpoint.win.",
		IPv6:    "ipv6.hfe-4538.nexttrace-io-fasttrace-endpoint.win.",
	},
	// 中国科学技术大学 科技网
	CST: ISPCollection{
		ISPName: "中国科学技术大学 科技网 AS7497",
		IP:      "ipv4.hfe-7497.nexttrace-io-fasttrace-endpoint.win.",
	},
}
