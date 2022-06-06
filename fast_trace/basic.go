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
	EDU      ISPCollection
	CST      ISPCollection
}

type ISPCollection struct {
	ISPName string
	IP      string
}

const (
	CT163  string = "电信 163 AS4134"
	CTCN2  string = "电信 CN2 AS4809"
	CU169  string = "联通 169 AS4837"
	CU9929 string = "联通 A网 AS9929"
	CM     string = "移动 骨干网 AS9808"
	EDU    string = "教育网 CERNET AS4538"
)

var TestIPsCollection = AllLocationCollection{
	Beijing:   Beijing,
	Shanghai:  Shanghai,
	Guangzhou: Guangzhou,
	Hangzhou:  Hangzhou,
	Hefei:     Hefei,
	Changsha:  Changsha,
}

var Beijing = BackBoneCollection{
	Location: "北京",
	CT163: ISPCollection{
		ISPName: CT163,
		IP:      "106.37.67.1",
	},

	CU169: ISPCollection{
		ISPName: CU169,
		IP:      "123.125.96.156",
	},

	CM: ISPCollection{
		ISPName: CM,
		IP:      "211.136.25.153",
	},

	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "101.6.15.130",
	},
}

var Shanghai = BackBoneCollection{
	Location: "上海",
	CT163: ISPCollection{
		ISPName: CT163,
		IP:      "101.226.28.198",
	},

	CTCN2: ISPCollection{
		ISPName: CTCN2,
		IP:      "58.32.4.1",
	},

	CU169: ISPCollection{
		ISPName: CU169,
		IP:      "139.226.206.150",
	},

	CU9929: ISPCollection{
		ISPName: CU9929,
		IP:      "210.13.86.1",
	},

	CM: ISPCollection{
		ISPName: CM,
		IP:      "120.204.34.85",
	},

	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "202.120.58.155",
	},
}

var Guangzhou = BackBoneCollection{
	Location: "广州",
	CT163: ISPCollection{
		ISPName: CT163,
		IP:      "106.37.67.1",
	},

	CU169: ISPCollection{
		ISPName: CU169,
		IP:      "123.125.96.156",
	},

	CM: ISPCollection{
		ISPName: CM,
		IP:      "120.198.26.254",
	},
}

var Hangzhou = BackBoneCollection{
	Location: "杭州",
	CT163: ISPCollection{
		ISPName: CT163,
		IP:      "61.164.23.196",
	},
	CU169: ISPCollection{
		ISPName: CU169,
		IP:      "60.12.244.1",
	},
	CM: ISPCollection{
		ISPName: CM,
		IP:      "112.17.224.98",
	},
	// 浙江大学 教育网
	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "210.32.2.1",
	},
}

var Hefei = BackBoneCollection{
	Location: "合肥",
	// 中国科学技术大学 教育网
	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "202.38.64.1",
	},
	// 中国科学技术大学 科技网
	CST: ISPCollection{
		ISPName: "中国科学技术大学 科技网 AS7497",
		IP:      "210.72.22.2",
	},
}

var Changsha = BackBoneCollection{
	Location: "长沙",
	// 中南大学 教育网
	EDU: ISPCollection{
		ISPName: EDU,
		IP:      "202.197.61.221",		
	},
}