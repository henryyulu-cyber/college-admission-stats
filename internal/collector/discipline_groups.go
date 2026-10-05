package collector

import (
	// 引入字串處理庫，用於關鍵字比對與大小寫無關轉換
	"strings"
)

// StandardGroups 定義教育部 18 大標準學群清單
var StandardGroups = []string{
	"資訊學群",
	"工程學群",
	"醫藥衛生學群",
	"數理化學群",
	"生命科學學群",
	"生物資源學群",
	"地球與環境學群",
	"建築與設計學群",
	"藝術學群",
	"社會與心理學群",
	"大眾傳播學群",
	"外語學群",
	"文史哲學群",
	"教育學群",
	"法政學群",
	"管理學群",
	"財經學群",
	"體育休閒學群",
}

// ClassifyDisciplineGroup 根據學系名稱進行智慧關鍵字比對，自動歸納至正確的 18 大學群
func ClassifyDisciplineGroup(deptName string) string {
	// 轉為小寫以便於處理英文字詞
	name := strings.ToLower(deptName)

	// 1. 醫藥衛生學群比對
	if strings.Contains(name, "醫學") || strings.Contains(name, "牙醫") || strings.Contains(name, "中醫") ||
		strings.Contains(name, "藥學") || strings.Contains(name, "護理") || strings.Contains(name, "物理治療") ||
		strings.Contains(name, "職能治療") || strings.Contains(name, "公共衛生") || strings.Contains(name, "醫事檢驗") ||
		strings.Contains(name, "放射") || strings.Contains(name, "營養") || strings.Contains(name, "聽力") ||
		strings.Contains(name, "語言治療") || strings.Contains(name, "獸醫") {
		// 若名稱中包含獸醫，部分分類屬生資，但臨床醫療常規亦納入醫藥生技，此處精準歸納為醫藥衛生或生物資源
		if strings.Contains(name, "獸醫") {
			return "生物資源學群"
		}
		return "醫藥衛生學群"
	}

	// 2. 資訊學群比對
	if strings.Contains(name, "資訊工程") || strings.Contains(name, "資工") || strings.Contains(name, "資訊管理") ||
		strings.Contains(name, "資管") || strings.Contains(name, "軟體") || strings.Contains(name, "人工智慧") ||
		strings.Contains(name, "資料科學") || strings.Contains(name, "資通訊") || strings.Contains(name, "計算機") ||
		strings.Contains(name, "數位多媒體") || strings.Contains(name, "雲端") || strings.Contains(name, "物聯網") ||
		strings.Contains(name, "資安") || strings.Contains(name, "資訊科技") || strings.Contains(name, "資訊傳播") {
		return "資訊學群"
	}

	// 3. 工程學群比對
	if strings.Contains(name, "電機") || strings.Contains(name, "電子") || strings.Contains(name, "機械") ||
		strings.Contains(name, "化工") || strings.Contains(name, "化學工程") || strings.Contains(name, "材料") ||
		strings.Contains(name, "土木") || strings.Contains(name, "光電") || strings.Contains(name, "航太") ||
		strings.Contains(name, "航空") || strings.Contains(name, "船舶") || strings.Contains(name, "造船") ||
		strings.Contains(name, "自動化") || strings.Contains(name, "機電") || strings.Contains(name, "系統工程") ||
		strings.Contains(name, "工業工程") || strings.Contains(name, "奈米") || strings.Contains(name, "核子") ||
		strings.Contains(name, "通訊工程") || strings.Contains(name, "生物醫學工程") || strings.Contains(name, "工程") {
		return "工程學群"
	}

	// 4. 數理化學群比對
	if strings.Contains(name, "數學") || strings.Contains(name, "物理") || strings.Contains(name, "化學") ||
		strings.Contains(name, "統計") || strings.Contains(name, "應用數學") || strings.Contains(name, "應用物理") ||
		strings.Contains(name, "應用化學") {
		return "數理化學群"
	}

	// 5. 生命科學學群比對
	if strings.Contains(name, "生命科學") || strings.Contains(name, "生科") || strings.Contains(name, "生物科技") ||
		strings.Contains(name, "生技") || strings.Contains(name, "分子生物") || strings.Contains(name, "基因") ||
		strings.Contains(name, "微生物") || strings.Contains(name, "生化科技") {
		return "生命科學學群"
	}

	// 6. 生物資源學群比對
	if strings.Contains(name, "農藝") || strings.Contains(name, "園藝") || strings.Contains(name, "森林") ||
		strings.Contains(name, "動物科學") || strings.Contains(name, "水產") || strings.Contains(name, "漁業") ||
		strings.Contains(name, "食品科學") || strings.Contains(name, "植物病理") || strings.Contains(name, "昆蟲") ||
		strings.Contains(name, "農業") || strings.Contains(name, "畜產") {
		return "生物資源學群"
	}

	// 7. 地球與環境學群比對
	if strings.Contains(name, "大氣") || strings.Contains(name, "地質") || strings.Contains(name, "地理") ||
		strings.Contains(name, "海洋") || strings.Contains(name, "地球") || strings.Contains(name, "環境工程") ||
		strings.Contains(name, "環工") || strings.Contains(name, "水資源") || strings.Contains(name, "環境科學") {
		return "地球與環境學群"
	}

	// 8. 建築與設計學群比對
	if strings.Contains(name, "建築") || strings.Contains(name, "工業設計") || strings.Contains(name, "視覺傳達") ||
		strings.Contains(name, "空間設計") || strings.Contains(name, "室內設計") || strings.Contains(name, "景觀") ||
		strings.Contains(name, "服裝設計") || strings.Contains(name, "時尚設計") || strings.Contains(name, "商品設計") ||
		strings.Contains(name, "設計") {
		return "建築與設計學群"
	}

	// 9. 藝術學群比對
	if strings.Contains(name, "音樂") || strings.Contains(name, "美術") || strings.Contains(name, "舞蹈") ||
		strings.Contains(name, "戲劇") || strings.Contains(name, "電影") || strings.Contains(name, "動畫") ||
		strings.Contains(name, "表演") || strings.Contains(name, "藝術") || strings.Contains(name, "國樂") {
		return "藝術學群"
	}

	// 10. 社會與心理學群比對
	if strings.Contains(name, "心理") || strings.Contains(name, "社會學") || strings.Contains(name, "社工") ||
		strings.Contains(name, "社會工作") || strings.Contains(name, "諮商") || strings.Contains(name, "人類學") ||
		strings.Contains(name, "輔導") || strings.Contains(name, "勞工") || strings.Contains(name, "犯罪") {
		return "社會與心理學群"
	}

	// 11. 大眾傳播學群比對
	if strings.Contains(name, "新聞") || strings.Contains(name, "廣播") || strings.Contains(name, "電視") ||
		strings.Contains(name, "廣告") || strings.Contains(name, "大眾傳播") || strings.Contains(name, "口語傳播") ||
		strings.Contains(name, "影音") || strings.Contains(name, "傳播") {
		return "大眾傳播學群"
	}

	// 12. 外語學群比對
	if strings.Contains(name, "外文") || strings.Contains(name, "英語") || strings.Contains(name, "英文") ||
		strings.Contains(name, "日語") || strings.Contains(name, "日文") || strings.Contains(name, "韓語") ||
		strings.Contains(name, "德文") || strings.Contains(name, "法文") || strings.Contains(name, "西班牙") ||
		strings.Contains(name, "應用外語") || strings.Contains(name, "外國語文") || strings.Contains(name, "語言") {
		return "外語學群"
	}

	// 13. 文史哲學群比對
	if strings.Contains(name, "中文") || strings.Contains(name, "中國文學") || strings.Contains(name, "歷史") ||
		strings.Contains(name, "哲學") || strings.Contains(name, "台灣文學") || strings.Contains(name, "台文") ||
		strings.Contains(name, "宗教") || strings.Contains(name, "國文") {
		return "文史哲學群"
	}

	// 14. 教育學群比對
	if strings.Contains(name, "教育") || strings.Contains(name, "幼兒教育") || strings.Contains(name, "幼保") ||
		strings.Contains(name, "特殊教育") || strings.Contains(name, "特教") || strings.Contains(name, "師資") ||
		strings.Contains(name, "學習與媒材") || strings.Contains(name, "教育科技") {
		return "教育學群"
	}

	// 15. 法政學群比對
	if strings.Contains(name, "法律") || strings.Contains(name, "法學") || strings.Contains(name, "政治") ||
		strings.Contains(name, "公共行政") || strings.Contains(name, "公行") || strings.Contains(name, "外交") ||
		strings.Contains(name, "行政") || strings.Contains(name, "地政") || strings.Contains(name, "財經法律") {
		return "法政學群"
	}

	// 16. 財經學群比對
	if strings.Contains(name, "經濟") || strings.Contains(name, "財務金融") || strings.Contains(name, "財金") ||
		strings.Contains(name, "金融") || strings.Contains(name, "會計") || strings.Contains(name, "財稅") ||
		strings.Contains(name, "保險") || strings.Contains(name, "風險管理") || strings.Contains(name, "計量財務") {
		return "財經學群"
	}

	// 17. 管理學群比對
	if strings.Contains(name, "企業管理") || strings.Contains(name, "企管") || strings.Contains(name, "國際企業") ||
		strings.Contains(name, "國企") || strings.Contains(name, "行銷") || strings.Contains(name, "運籌") ||
		strings.Contains(name, "物流") || strings.Contains(name, "航運") || strings.Contains(name, "觀光管理") ||
		strings.Contains(name, "餐旅管理") || strings.Contains(name, "管理") || strings.Contains(name, "工商") {
		return "管理學群"
	}

	// 18. 體育休閒學群比對
	if strings.Contains(name, "體育") || strings.Contains(name, "休閒") || strings.Contains(name, "運動") ||
		strings.Contains(name, "觀光") || strings.Contains(name, "餐旅") || strings.Contains(name, "健康休閒") {
		return "體育休閒學群"
	}

	// 若皆未匹配，回傳未分類學群
	return "其他學群"
}

// ClassifyUnivType 根據大學名稱自動判斷為「國立」或「私立」
func ClassifyUnivType(univName string) string {
	// 若校名包含「國立」、「市立」、「縣立」或「軍事/警官院校」即為公立/國立
	if strings.HasPrefix(univName, "國立") || strings.HasPrefix(univName, "市立") ||
		strings.HasPrefix(univName, "臺北市立") || strings.HasPrefix(univName, "高雄市立") ||
		strings.Contains(univName, "警察") || strings.Contains(univName, "國防") {
		return "國立"
	}
	// 其餘學校歸類為私立
	return "私立"
}

// ClassifyUnivSystem 根據大學名稱自動判斷為「一般大學」或「技專校院」
func ClassifyUnivSystem(univName string) string {
	// 若校名包含「科技大學」、「技術學院」、「專科」則為技專校院
	if strings.Contains(univName, "科技大學") || strings.Contains(univName, "技術學院") ||
		strings.Contains(univName, "專科") || strings.Contains(univName, "科大") {
		return "技專校院"
	}
	// 其餘學校歸類為一般大學
	return "一般大學"
}
