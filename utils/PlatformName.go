package utils

// AccountTypeStr 平台类型 -> 中文名。
// 独立成文件保证零依赖：global、web/* 都能安全引用，避免包循环。
var AccountTypeStr = map[string]string{
	"XUEXITONG": "学习通",
	"YINGHUA":   "英华学堂",
	"CANGHUI":   "仓辉实训",
	"ENAEA":     "学习公社",
	"CQIE":      "重庆工程学院",
	"KETANGX":   "码上研训",
	"ICVE":      "智慧职教",
	"QSXT":      "青书学堂",
	"WELEARN":   "WeLearn",
	"HQKJ":      "海旗科技",
}

// AccountTypeName 平台中文名，未知类型原样返回
func AccountTypeName(accountType string) string {
	if name, ok := AccountTypeStr[accountType]; ok {
		return name
	}
	return accountType
}

// IsKnownAccountType 是否为已支持的平台类型
func IsKnownAccountType(accountType string) bool {
	_, ok := AccountTypeStr[accountType]
	return ok
}
