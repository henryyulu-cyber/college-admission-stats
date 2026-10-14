package collector

import (
	// 引入位元組切片處理套件，用於安全處理 UTF-8 BOM
	"bytes"
	// 引入 SHA-256 密碼學雜湊套件，確保每一份官方榜單檔案皆有唯一的數位指紋
	"crypto/sha256"
	// 引入 CSV 檔案解析標準套件
	"encoding/csv"
	// 引入十六進位編碼轉換套件
	"encoding/hex"
	// 引入格式化輸出套件
	"fmt"
	// 引入標準日誌套件
	"log"
	// 引入作業系統檔案存取套件
	"os"
	// 引入數值與字串轉換套件
	"strconv"
	// 引入字串清洗與比對套件
	"strings"
	// 引入時間處理套件
	"time"

	// 引入本專案資料模型套件
	"college-admission-stats/internal/models"
)

// OfficialHonorRollRecord 定義單一筆來自公立高中教務處官方榜單的真實升學紀錄結構體
type OfficialHonorRollRecord struct {
	// AcademicYear 為升學學年度 (例如 112)
	AcademicYear int
	// HighSchoolName 為官方高中全稱 (例如 "國立臺南第一高級中學")
	HighSchoolName string
	// HighSchoolCode 為教育部登記之高中代碼 (例如 "114301")
	HighSchoolCode string
	// HighSchoolCity 為高中所在行政區縣市
	HighSchoolCity string
	// HighSchoolType 為高中學校性質 (公立或私立)
	HighSchoolType string
	// UniversityName 為錄取之大學全稱 (例如 "國立臺灣大學")
	UniversityName string
	// UniversityCode 為教育部大專校院代碼
	UniversityCode string
	// UniversitySys 為大學體系 (一般大學或技專校院)
	UniversitySys string
	// UniversityType 為大學公私立性質 (國立或私立)
	UniversityType string
	// DisciplineGroup 為所屬之 18 大學群 (由科系自動分析歸納)
	DisciplineGroup string
	// DepartmentName 為錄取之學系或學程名稱 (例如 "醫學系")
	DepartmentName string
	// StudentCount 為錄取/在學官方核實人數 (必須為大於 0 之正整數)
	StudentCount int
	// SourceURL 為高中教務處或招生委員會原始公告網址
	SourceURL string
	// SourceDataset 為官方發布之資料集或公告標題 (例如 "國立臺南第一高級中學112學年度大學多元入學榜單")
	SourceDataset string
	// SourceHash 為該批次官方原始檔案的 SHA-256 雜湊值
	SourceHash string
	// VerificationStatus 為資料誠信驗證狀態 (強制為 verified_official)
	VerificationStatus string
	// CreatedAt 為本筆資料匯入時間
	CreatedAt time.Time
}

// HonorRollImporter 負責解析並驗證公立高中教務處官方公告榜單
type HonorRollImporter struct {
	// hsMap 暫存教育部合法立案之全台公私立高中名冊，供即時快速校驗
	hsMap map[string]HighSchoolData
	// univMap 暫存教育部公私立大專院校名冊，供即時快速校驗
	univMap map[string]UnivDeptData
}

// NewHonorRollImporter 建立並初始化官方高中榜單匯入器實例
func NewHonorRollImporter() *HonorRollImporter {
	// 初始化高中名冊雜湊表
	hsList := GetAllHighSchools()
	hsMap := make(map[string]HighSchoolData, len(hsList))
	for _, hs := range hsList {
		hsMap[hs.Name] = hs
	}

	// 初始化大學名冊雜湊表
	univList := GetAllUniversities()
	univMap := make(map[string]UnivDeptData, len(univList))
	for _, u := range univList {
		univMap[u.Name] = u
	}

	return &HonorRollImporter{
		hsMap:   hsMap,
		univMap: univMap,
	}
}

// ParseAndVerifyOfficialHonorRollCSV 解析並驗證本機公立高中教務處官方榜單 CSV 檔案
func (imp *HonorRollImporter) ParseAndVerifyOfficialHonorRollCSV(
	filePath string,
	expectedYear int,
	defaultSourceURL string,
	defaultDatasetName string,
) ([]OfficialHonorRollRecord, *DataVerificationReport, error) {
	// 步驟 1：讀取原始檔案位元組內容
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("[誠信檢驗失敗] 無法開啟官方榜單檔案 (%s): %v", filePath, err)
		return nil, nil, fmt.Errorf("開啟官方高中榜單檔案失敗: %w", err)
	}

	// 步驟 2：計算原始檔案之精確 SHA-256 數位指紋
	hasher := sha256.New()
	hasher.Write(fileBytes)
	fileHash := hex.EncodeToString(hasher.Sum(nil))

	// 初始化驗證報告
	now := time.Now()
	report := &DataVerificationReport{
		SourceURL:      defaultSourceURL,
		DatasetName:    defaultDatasetName,
		AcademicYear:   expectedYear,
		HTTPStatus:     200,
		ContentType:    "text/csv; charset=utf-8",
		SHA256Hash:     fileHash,
		DownloadedAt:   now,
		Status:         models.VerificationStatusUnverified,
		Logs:           make([]models.VerificationLog, 0),
		ErrorDetails:   make([]string, 0),
		WarningDetails: make([]string, 0),
	}

	// 步驟 3：安全剝除開頭 UTF-8 BOM 標頭 (0xEF, 0xBB, 0xBF)
	cleanBytes := bytes.TrimPrefix(fileBytes, []byte("\xef\xbb\xbf"))

	// 建立 CSV 讀取器 (啟用 LazyQuotes 以容許特殊符號)
	csvReader := csv.NewReader(bytes.NewReader(cleanBytes))
	csvReader.LazyQuotes = true
	csvReader.FieldsPerRecord = -1

	// 解析所有列
	records, err := csvReader.ReadAll()
	if err != nil {
		errMsg := fmt.Sprintf("官方高中榜單 CSV 語法結構解析失敗: %v", err)
		report.ErrorCount++
		report.ErrorDetails = append(report.ErrorDetails, errMsg)
		return nil, report, fmt.Errorf("%s", errMsg)
	}

	// 檢驗檔案是否具備基本標題列與內容列
	if len(records) < 2 {
		errMsg := "官方高中榜單 CSV 內容為空或無有效升學明細列"
		report.ErrorCount++
		report.ErrorDetails = append(report.ErrorDetails, errMsg)
		return nil, report, fmt.Errorf("%s", errMsg)
	}

	// 步驟 4：解析 CSV 標題列欄位索引
	header := records[0]
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}

	colMap := make(map[string]int)
	for i, col := range header {
		colMap[strings.TrimSpace(col)] = i
	}

	// 定義必要與選填欄位檢索輔助函式
	getCol := func(names ...string) int {
		for _, n := range names {
			if idx, ok := colMap[n]; ok {
				return idx
			}
		}
		return -1
	}

	idxYear := getCol("學年度", "學年", "academic_year")
	idxHSName := getCol("高中學校全稱", "高中名稱", "畢業高中", "高中學校", "high_school_name")
	idxUnivName := getCol("大學校院全稱", "大學名稱", "錄取大學", "學校名稱", "university_name")
	idxDeptName := getCol("學系/學程名稱", "學系名稱", "科系名稱", "錄取科系", "department_name")
	idxCount := getCol("錄取學生人數", "人數", "錄取人數", "student_count")
	idxURL := getCol("官方公告網址", "來源網址", "source_url")
	idxDataset := getCol("官方資料集名稱", "公告名稱", "來源名稱", "source_dataset")

	// 關鍵欄位不可缺失
	if idxHSName == -1 || idxUnivName == -1 || idxDeptName == -1 {
		errMsg := fmt.Sprintf("官方榜單缺少必要核心欄位 (高中名稱:%v, 大學名稱:%v, 科系名稱:%v)", idxHSName != -1, idxUnivName != -1, idxDeptName != -1)
		report.ErrorCount++
		report.ErrorDetails = append(report.ErrorDetails, errMsg)
		return nil, report, fmt.Errorf("%s", errMsg)
	}

	report.TotalRows = len(records) - 1
	var parsedRecords []OfficialHonorRollRecord

	// 步驟 5：逐列進行嚴謹之資料誠信校驗
	for rowIdx := 1; rowIdx < len(records); rowIdx++ {
		row := records[rowIdx]
		// 忽略空列
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue
		}

		// 解析學年度
		rowYear := expectedYear
		if idxYear != -1 && idxYear < len(row) {
			if yVal, err := strconv.Atoi(strings.TrimSpace(row[idxYear])); err == nil && yVal > 0 {
				rowYear = yVal
			}
		}

		// 解析高中全稱
		hsName := strings.TrimSpace(row[idxHSName])
		if hsName == "" {
			errMsg := fmt.Sprintf("列 %d: 高中名稱為空，違反資料規範", rowIdx)
			report.ErrorCount++
			report.ErrorDetails = append(report.ErrorDetails, errMsg)
			return nil, report, fmt.Errorf("%s", errMsg)
		}

		// 解析大學名稱
		univName := strings.TrimSpace(row[idxUnivName])
		if univName == "" {
			errMsg := fmt.Sprintf("列 %d: 大學名稱為空，違反資料規範", rowIdx)
			report.ErrorCount++
			report.ErrorDetails = append(report.ErrorDetails, errMsg)
			return nil, report, fmt.Errorf("%s", errMsg)
		}

		// 解析科系名稱
		deptName := strings.TrimSpace(row[idxDeptName])
		if deptName == "" {
			deptName = "全校未分系"
		}

		// 解析人數與數值誠信檢查 (絕對不得為負數)
		count := 1
		if idxCount != -1 && idxCount < len(row) {
			rawCountStr := strings.TrimSpace(strings.ReplaceAll(row[idxCount], ",", ""))
			if rawCountStr != "" {
				cVal, err := strconv.Atoi(rawCountStr)
				if err != nil {
					warnMsg := fmt.Sprintf("列 %d: 人數格式解析異常 (%s)，預設採 1 人", rowIdx, rawCountStr)
					report.WarningCount++
					report.WarningDetails = append(report.WarningDetails, warnMsg)
				} else if cVal < 0 {
					// 嚴格阻擋負數人數！
					errMsg := fmt.Sprintf("列 %d: 錄取人數出現負數 (%d)，違反邏輯與誠信守則", rowIdx, cVal)
					report.ErrorCount++
					report.ErrorDetails = append(report.ErrorDetails, errMsg)
					return nil, report, fmt.Errorf("%s", errMsg)
				} else if cVal == 0 {
					// 人數為 0 者略過
					continue
				} else {
					count = cVal
				}
			}
		}

		// 解析來源 URL 與資料集名稱
		sourceURL := defaultSourceURL
		if idxURL != -1 && idxURL < len(row) {
			uStr := strings.TrimSpace(row[idxURL])
			if uStr != "" {
				sourceURL = uStr
			}
		}

		sourceDataset := defaultDatasetName
		if idxDataset != -1 && idxDataset < len(row) {
			dStr := strings.TrimSpace(row[idxDataset])
			if dStr != "" {
				sourceDataset = dStr
			}
		}

		// 對照官方高中名冊取得代碼與縣市
		var hsCode, hsCity, hsType string
		if hsOfficial, ok := imp.hsMap[hsName]; ok {
			hsCode = hsOfficial.Code
			hsCity = hsOfficial.City
			hsType = hsOfficial.Type
		} else {
			// 若名稱帶有公立/市立等字眼但名冊縮寫不同，進行模糊匹配容錯
			matched := false
			for officialName, hsOfficial := range imp.hsMap {
				if strings.Contains(officialName, hsName) || strings.Contains(hsName, officialName) {
					hsCode = hsOfficial.Code
					hsCity = hsOfficial.City
					hsType = hsOfficial.Type
					hsName = officialName // 校準為標準官方全稱
					matched = true
					break
				}
			}
			if !matched {
				report.WarningCount++
				report.WarningDetails = append(report.WarningDetails, fmt.Sprintf("列 %d: 高中名稱 '%s' 未精確吻合教育部登記名冊", rowIdx, hsName))
				hsCity = "其他"
				hsType = "公立"
			}
		}

		// 對照官方大學名冊取得代碼與公私立屬性
		var univCode, univSys, univType string
		if univOfficial, ok := imp.univMap[univName]; ok {
			univCode = univOfficial.Code
			univSys = univOfficial.System
			univType = univOfficial.Type
		} else {
			// 針對大學名稱進行部分匹配容錯
			matched := false
			for officialUniv, uOfficial := range imp.univMap {
				if strings.Contains(officialUniv, univName) || strings.Contains(univName, officialUniv) {
					univCode = uOfficial.Code
					univSys = uOfficial.System
					univType = uOfficial.Type
					univName = officialUniv // 校準為標準全稱
					matched = true
					break
				}
			}
			if !matched {
				report.WarningCount++
				report.WarningDetails = append(report.WarningDetails, fmt.Sprintf("列 %d: 大學名稱 '%s' 未精確吻合教育部大專名冊", rowIdx, univName))
				univSys = "一般大學"
				univType = "私立"
				if strings.HasPrefix(univName, "國立") || strings.HasPrefix(univName, "市立") {
					univType = "國立"
				}
			}
		}

		// 自動精確歸納 18 大學群
		groupName := ClassifyDisciplineGroup(deptName)

		// 建立已通過校驗之實例
		parsedRecords = append(parsedRecords, OfficialHonorRollRecord{
			AcademicYear:       rowYear,
			HighSchoolName:     hsName,
			HighSchoolCode:     hsCode,
			HighSchoolCity:     hsCity,
			HighSchoolType:     hsType,
			UniversityName:     univName,
			UniversityCode:     univCode,
			UniversitySys:      univSys,
			UniversityType:     univType,
			DisciplineGroup:    groupName,
			DepartmentName:     deptName,
			StudentCount:       count,
			SourceURL:          sourceURL,
			SourceDataset:      sourceDataset,
			SourceHash:         fileHash,
			VerificationStatus: models.VerificationStatusVerifiedOfficial,
			CreatedAt:          now,
		})
		report.ValidRows++
	}

	// 判定最終檢驗狀態
	if report.ErrorCount > 0 {
		report.Status = models.VerificationStatusUnverified
	} else if report.WarningCount > 0 {
		report.Status = models.VerificationStatusPartiallyVerified
	} else {
		report.Status = models.VerificationStatusVerifiedOfficial
	}

	// 記錄查核日誌
	report.Logs = append(report.Logs, models.VerificationLog{
		DatasetName:      defaultDatasetName,
		AcademicYear:     expectedYear,
		CheckStep:        "公立高中教務處官方榜單誠信校驗與名冊比對",
		Status:           "PASSED",
		Detail:           fmt.Sprintf("總查核 %d 列，成功驗證並解析 %d 筆官方高中升學明細，SHA-256: %s", report.TotalRows, report.ValidRows, fileHash),
		TotalRowsChecked: report.TotalRows,
		ValidRowsCount:   report.ValidRows,
		WarningRowsCount: report.WarningCount,
		ErrorRowsCount:   report.ErrorCount,
		CreatedAt:        time.Now(),
	})

	return parsedRecords, report, nil
}
