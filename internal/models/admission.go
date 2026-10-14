package models

import (
	// 引入時間處理套件，用於記錄資料建立與同步時間
	"time"
)

// 定義資料誠信狀態常數
const (
	// VerificationStatusVerifiedOfficial 代表已由官方來源取得並通過 SHA-256 與格式嚴格校驗
	VerificationStatusVerifiedOfficial = "verified_official"
	// VerificationStatusPartiallyVerified 代表部分欄位或年度仍待查證
	VerificationStatusPartiallyVerified = "partially_verified"
	// VerificationStatusUnverified 代表沒有足夠官方依據
	VerificationStatusUnverified = "unverified"
	// VerificationStatusSynthetic 代表測試用合成資料，正式環境嚴格禁止載入
	VerificationStatusSynthetic = "synthetic"
)

// DataSource 定義官方資料集之來源溯源與下載查核資訊
type DataSource struct {
	// ID 為資料庫主鍵
	ID int64 `json:"id" db:"id"`
	// SourceName 為官方機構名稱 (例如 "教育部統計處", "政府資料開放平臺")
	SourceName string `json:"source_name" db:"source_name"`
	// DatasetName 為資料集完整名稱 (例如 "大專校院各校科系別學生數")
	DatasetName string `json:"dataset_name" db:"dataset_name"`
	// URL 為原始下載網址或開放 API 端點
	URL string `json:"url" db:"url"`
	// DownloadedAt 為資料下載日期與時間
	DownloadedAt time.Time `json:"downloaded_at" db:"downloaded_at"`
	// AcademicYear 為資料所屬學年度 (民國年)
	AcademicYear int `json:"academic_year" db:"academic_year"`
	// HTTPStatus 為下載時之 HTTP 回應狀態碼 (例如 200)
	HTTPStatus int `json:"http_status" db:"http_status"`
	// FileHash 為原始檔案之 SHA-256 完整雜湊值，用於防竄改比對
	FileHash string `json:"file_hash" db:"file_hash"`
	// DataVersion 為資料版本或官方更新日期
	DataVersion string `json:"data_version" db:"data_version"`
	// VerificationStatus 為資料驗證狀態 (verified_official, partially_verified 等)
	VerificationStatus string `json:"verification_status" db:"verification_status"`
	// RecordCount 為此資料集成功解析與匯入之筆數
	RecordCount int `json:"record_count" db:"record_count"`
	// ErrorMessage 為下載或校驗時產生的警告或錯誤摘要
	ErrorMessage string `json:"error_message" db:"error_message"`
	// CreatedAt 為本筆記錄寫入時間
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// VerificationLog 定義資料管線校驗日誌與檢驗報告紀錄
type VerificationLog struct {
	// ID 為資料庫主鍵
	ID int64 `json:"id" db:"id"`
	// DatasetName 為所校驗之資料集名稱
	DatasetName string `json:"dataset_name" db:"dataset_name"`
	// AcademicYear 為學年度
	AcademicYear int `json:"academic_year" db:"academic_year"`
	// CheckStep 為目前校驗步驟 (如 "HTTP狀態檢查", "SHA256驗證", "欄位完整性檢查", "數值非負檢查")
	CheckStep string `json:"check_step" db:"check_step"`
	// Status 為該步驟之狀態 ("PASSED", "WARNING", "FAILED")
	Status string `json:"status" db:"status"`
	// Detail 為檢查細節或異常說明
	Detail string `json:"detail" db:"detail"`
	// TotalRowsChecked 為檢查總列數
	TotalRowsChecked int `json:"total_rows_checked" db:"total_rows_checked"`
	// ValidRowsCount 為符合規格之有效列數
	ValidRowsCount int `json:"valid_rows_count" db:"valid_rows_count"`
	// WarningRowsCount 為存在警告之列數 (如非官方代碼但有校名)
	WarningRowsCount int `json:"warning_rows_count" db:"warning_rows_count"`
	// ErrorRowsCount 為存在致命錯誤之列數 (如負數、欄位缺失)
	ErrorRowsCount int `json:"error_rows_count" db:"error_rows_count"`
	// CreatedAt 為紀錄建立時間戳記
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// AdmissionRecord 定義單筆高中升學大專校院系所之詳細紀錄 (包含完整資料誠信溯源欄位)
type AdmissionRecord struct {
	// ID 為資料庫主鍵編號
	ID int64 `json:"id" db:"id"`
	// AcademicYear 為學年度 (民國年，例如 110、111、112、113、114、115)
	AcademicYear int `json:"academic_year" db:"academic_year"`
	// HighSchoolCode 為高中學校代碼 (例如 "014301")
	HighSchoolCode string `json:"high_school_code" db:"high_school_code"`
	// HighSchoolName 為高中學校名稱 (例如 "臺北市立建國高級中學")
	HighSchoolName string `json:"high_school_name" db:"high_school_name"`
	// HighSchoolCity 為高中所在縣市 (例如 "臺北市", "新北市", "臺中市", "高雄市")
	HighSchoolCity string `json:"high_school_city" db:"high_school_city"`
	// HighSchoolType 為高中公私立性質 ("公立" 或 "私立")
	HighSchoolType string `json:"high_school_type" db:"high_school_type"`
	// UniversityCode 為錄取大專校院代碼 (例如 "0001")
	UniversityCode string `json:"university_code" db:"university_code"`
	// UniversityName 為錄取大專校院名稱 (例如 "國立臺灣大學")
	UniversityName string `json:"university_name" db:"university_name"`
	// UniversitySystem 為大學體系 ("一般大學" 或 "技專校院")
	UniversitySystem string `json:"university_system" db:"university_system"`
	// UniversityType 為大學公私立性質 ("國立" 或 "私立")
	UniversityType string `json:"university_type" db:"university_type"`
	// DisciplineGroup 為教育部 18 大學群 (例如 "資訊學群", "工程學群", "醫藥衛生學群")
	DisciplineGroup string `json:"discipline_group" db:"discipline_group"`
	// DepartmentName 為錄取學系或學程名稱 (例如 "資訊工程學系")
	DepartmentName string `json:"department_name" db:"department_name"`
	// StudentCount 為錄取或就讀之學生人數 (必須 >= 0)
	StudentCount int `json:"student_count" db:"student_count"`
	// SourceURL 為原始官方資料下載網址或開放 API 來源
	SourceURL string `json:"source_url" db:"source_url"`
	// SourceDataset 為所屬官方資料集完整名稱
	SourceDataset string `json:"source_dataset" db:"source_dataset"`
	// SourceDownloadedAt 為原始資料下載之時間戳記
	SourceDownloadedAt time.Time `json:"source_downloaded_at" db:"source_downloaded_at"`
	// SourceHash 為原始下載檔案之 SHA-256 驗證雜湊值
	SourceHash string `json:"source_hash" db:"source_hash"`
	// VerificationStatus 為資料誠信驗證狀態 (verified_official, partially_verified, unverified, synthetic)
	VerificationStatus string `json:"verification_status" db:"verification_status"`
	// CreatedAt 為記錄寫入資料庫之時間戳記
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// HighSchoolSummary 高中升學概況摘要 (用於 6 大 KPI 卡片與總覽)
type HighSchoolSummary struct {
	// HighSchoolName 為高中名稱
	HighSchoolName string `json:"high_school_name"`
	// City 為所在縣市
	City string `json:"city"`
	// SchoolType 為公私立
	SchoolType string `json:"school_type"`
	// TotalStudents 為總升學人數
	TotalStudents int `json:"total_students"`
	// NationalUnivStudents 為錄取國立大學人數
	NationalUnivStudents int `json:"national_univ_students"`
	// NationalRate 為國立大學錄取率 (百分比字串，例如 "78.5%")
	NationalRate string `json:"national_rate"`
	// TopUnivStudents 為錄取 5 大頂尖大學人數 (台大、政大、清大、成大、陽明交大)
	TopUnivStudents int `json:"top_univ_students"`
	// TopUnivRate 為頂尖大學錄取率 (百分比字串，例如 "42.8%")
	TopUnivRate string `json:"top_univ_rate"`
	// MedDentChineseStudents 為錄取醫牙中人數 (醫學系、牙醫學系、中醫學系)
	MedDentChineseStudents int `json:"med_dent_chinese_students"`
	// MedDentChineseRate 為醫牙中錄取率 (百分比字串，例如 "6.5%")
	MedDentChineseRate string `json:"med_dent_chinese_rate"`
	// PrivateUnivStudents 為錄取私立大學人數
	PrivateUnivStudents int `json:"private_univ_students"`
	// PrivateRate 為私立大學錄取率 (百分比字串)
	PrivateRate string `json:"private_rate"`
	// TechUnivStudents 為錄取技專校院 (科大) 人數
	TechUnivStudents int `json:"tech_univ_students"`
	// TechRate 為技專校院 (科大) 錄取率 (百分比字串)
	TechRate string `json:"tech_rate"`
}

// MultiSchoolComparisonRow 多校橫向對比統計列結構體 (用於多選比較表格與分組直方圖)
type MultiSchoolComparisonRow struct {
	// HighSchoolName 為高中學校名稱
	HighSchoolName string `json:"high_school_name"`
	// City 為高中所在縣市
	City string `json:"city"`
	// SchoolType 為高中公私立性質 ("公立" / "私立")
	SchoolType string `json:"school_type"`
	// TotalStudents 為該校總升學人數
	TotalStudents int `json:"total_students"`
	// NationalStudents 為錄取國立大學人數
	NationalStudents int `json:"national_students"`
	// NationalRate 為國立大學錄取率字串
	NationalRate string `json:"national_rate"`
	// NationalRateVal 為國立大學錄取率浮點數值 (供直方圖圖表使用)
	NationalRateVal float64 `json:"national_rate_val"`
	// TopUnivStudents 為錄取頂尖大學 (台清交成政) 人數
	TopUnivStudents int `json:"top_univ_students"`
	// TopUnivRate 為頂大錄取率字串
	TopUnivRate string `json:"top_univ_rate"`
	// TopUnivRateVal 為頂大錄取率浮點數值 (供直方圖圖表使用)
	TopUnivRateVal float64 `json:"top_univ_rate_val"`
	// MedDentChineseStudents 為錄取醫牙中 (醫學+牙醫+中醫) 人數
	MedDentChineseStudents int `json:"med_dent_chinese_students"`
	// MedDentChineseRate 為醫牙中錄取率字串
	MedDentChineseRate string `json:"med_dent_chinese_rate"`
	// MedDentChineseRateVal 為醫牙中錄取率浮點數值 (供直方圖圖表使用)
	MedDentChineseRateVal float64 `json:"med_dent_chinese_rate_val"`
	// PrivateStudents 為錄取私立大學人數
	PrivateStudents int `json:"private_students"`
	// PrivateRate 為私立大學錄取率字串
	PrivateRate string `json:"private_rate"`
	// TechStudents 為錄取技專校院 (科大) 人數
	TechStudents int `json:"tech_students"`
	// TechRate 為技專校院錄取率字串
	TechRate string `json:"tech_rate"`
}

// UniversityDestinationStat 高中錄取之各大學人數分佈
type UniversityDestinationStat struct {
	// UniversityName 為大學名稱
	UniversityName string `json:"university_name"`
	// UniversityType 為公私立 (國立/私立)
	UniversityType string `json:"university_type"`
	// UniversitySystem 為體系 (一般大學/技專校院)
	UniversitySystem string `json:"university_system"`
	// StudentCount 為錄取人數
	StudentCount int `json:"student_count"`
	// Percentage 為佔該高中總升學人數百分比
	Percentage float64 `json:"percentage"`
}

// GroupDistributionStat 高中升學之 18 大學群分佈
type GroupDistributionStat struct {
	// DisciplineGroup 為學群名稱
	DisciplineGroup string `json:"discipline_group"`
	// StudentCount 為升讀人數
	StudentCount int `json:"student_count"`
	// Percentage 為佔該高中升讀總數百分比
	Percentage float64 `json:"percentage"`
}

// UniversityOriginStat 大學錄取學生之來源高中排行
type UniversityOriginStat struct {
	// HighSchoolName 為來源高中名稱
	HighSchoolName string `json:"high_school_name"`
	// City 為高中縣市
	City string `json:"city"`
	// SchoolType 為高中公私立
	SchoolType string `json:"school_type"`
	// StudentCount 為錄取人數
	StudentCount int `json:"student_count"`
	// Percentage 為佔該大學或學系錄取總數百分比
	Percentage float64 `json:"percentage"`
}

// GroupTrendStat 18 大學群跨年度趨勢統計
type GroupTrendStat struct {
	// AcademicYear 為學年度
	AcademicYear int `json:"academic_year"`
	// DisciplineGroup 為學群名稱
	DisciplineGroup string `json:"discipline_group"`
	// StudentCount 為該學年度總錄取人數
	StudentCount int `json:"student_count"`
}

// TopDepartmentStat 熱門科系排行統計
type TopDepartmentStat struct {
	// UniversityName 為大學名稱
	UniversityName string `json:"university_name"`
	// DepartmentName 為學系名稱
	DepartmentName string `json:"department_name"`
	// DisciplineGroup 為所屬學群
	DisciplineGroup string `json:"discipline_group"`
	// StudentCount 為總升學人數
	StudentCount int `json:"student_count"`
}

// FilterParams 定義前端篩選表單傳入之後端查詢參數
type FilterParams struct {
	// AcademicYear 為指定學年度 (0 代表全部學年)
	AcademicYear int `query:"academic_year"`
	// HighSchool 為指定或搜尋之高中名稱
	HighSchool string `query:"high_school"`
	// HighSchoolCity 為高中所在縣市篩選
	HighSchoolCity string `query:"high_school_city"`
	// University 為指定或搜尋之大學名稱
	University string `query:"university"`
	// UniversitySystem 為大學體系篩選 ("一般大學", "技專校院")
	UniversitySystem string `query:"university_system"`
	// UniversityType 為大學公私立篩選 ("國立", "私立")
	UniversityType string `query:"university_type"`
	// DisciplineGroup 為 18 大學群篩選
	DisciplineGroup string `query:"discipline_group"`
	// Department 為科系名稱關鍵字搜尋
	Department string `query:"department"`
	// Keyword 為通用全文搜尋關鍵字
	Keyword string `query:"keyword"`
	// Page 為目前頁碼 (預設 1)
	Page int `query:"page"`
	// PageSize 為每頁筆數 (預設 20)
	PageSize int `query:"page_size"`
}

// SyncStatus 定義資料下載與同步作業之當前狀態
type SyncStatus struct {
	// TotalRecords 為目前資料庫內總記錄筆數
	TotalRecords int64 `json:"total_records"`
	// YearsAvailable 為資料庫中已包含之學年度列表
	YearsAvailable []int `json:"years_available"`
	// HighSchoolCount 為資料庫中不重複的高中總數
	HighSchoolCount int `json:"high_school_count"`
	// UniversityCount 為資料庫中不重複的大學總數
	UniversityCount int `json:"university_count"`
	// LastSyncTime 為最後一次同步成功的時間
	LastSyncTime time.Time `json:"last_sync_time"`
	// IsSyncing 為當前是否正在背景執行同步作業
	IsSyncing bool `json:"is_syncing"`
	// LastMessage 為最後執行狀態訊息
	LastMessage string `json:"last_message"`
}

// MedicalStatsSummary 醫牙中專區總體統計指標 (用於 4 大 KPI 卡片)
type MedicalStatsSummary struct {
	// TotalMedDentStudents 為醫牙中總錄取人數 (醫學系 + 牙醫學系 + 中醫學系)
	TotalMedDentStudents int `json:"total_med_dent_students"`
	// MedStudents 為醫學系總錄取人數
	MedStudents int `json:"med_students"`
	// DentStudents 為牙醫學系總錄取人數
	DentStudents int `json:"dent_students"`
	// ChineseMedStudents 為中醫學系總錄取人數
	ChineseMedStudents int `json:"chinese_med_students"`
	// NationalMedStudents 為錄取國立大學醫牙中人數 (如台大、陽明交大、成大、中興)
	NationalMedStudents int `json:"national_med_students"`
	// PrivateMedStudents 為錄取私立醫學大學醫牙中人數 (如北醫、長庚、高醫、中國醫、中山醫)
	PrivateMedStudents int `json:"private_med_students"`
	// NationalRate 為國立醫牙中錄取比例百分比字串 (例如 "36.5%")
	NationalRate string `json:"national_rate"`
}

// MedicalHighSchoolRank 全台高中醫牙中錄取人數排行結構
type MedicalHighSchoolRank struct {
	// Rank 為名次序號 (1, 2, 3...)
	Rank int `json:"rank"`
	// HighSchoolName 為高中學校名稱
	HighSchoolName string `json:"high_school_name"`
	// City 為高中所在縣市
	City string `json:"city"`
	// SchoolType 為高中公私立性質 ("公立" / "私立")
	SchoolType string `json:"school_type"`
	// MedCount 為錄取醫學系人數
	MedCount int `json:"med_count"`
	// DentCount 為錄取牙醫學系人數
	DentCount int `json:"dent_count"`
	// ChineseMedCount 為錄取中醫學系人數
	ChineseMedCount int `json:"chinese_med_count"`
	// TotalCount 為醫牙中總計錄取人數 (MedCount + DentCount + ChineseMedCount)
	TotalCount int `json:"total_count"`
}

// MedicalUnivStat 大學醫牙中錄取分佈結構 (用於大餅圖與清單)
type MedicalUnivStat struct {
	// UniversityName 為大學名稱
	UniversityName string `json:"university_name"`
	// DepartmentName 為學系名稱 ("醫學系", "牙醫學系", "中醫學系")
	DepartmentName string `json:"department_name"`
	// UniversityType 為大學性質 ("國立" / "私立")
	UniversityType string `json:"university_type"`
	// StudentCount 為該校該系總錄取人數
	StudentCount int `json:"student_count"`
	// Percentage 為佔該系別總錄取人數百分比
	Percentage float64 `json:"percentage"`
}

// DisciplineGroupSummary 學群深度分析之核心指標概況
type DisciplineGroupSummary struct {
	// DisciplineGroup 為學群名稱
	DisciplineGroup string `json:"discipline_group"`
	// TotalStudents 為該學群總錄取人數
	TotalStudents int `json:"total_students"`
	// DepartmentCount 為該學群涵蓋之獨立學系數量
	DepartmentCount int `json:"department_count"`
	// NationalStudents 為錄取國立大學人數
	NationalStudents int `json:"national_students"`
	// PrivateStudents 為錄取私立大學人數
	PrivateStudents int `json:"private_students"`
	// NationalRate 為國立大學佔比百分比
	NationalRate string `json:"national_rate"`
	// PublicHighSchoolRate 為公立高中生源佔比百分比
	PublicHighSchoolRate string `json:"public_high_school_rate"`
}

// DepartmentShareStat 學群內各學系人數佔比 (用於動態大餅圖)
type DepartmentShareStat struct {
	// DepartmentName 為學系名稱
	DepartmentName string `json:"department_name"`
	// StudentCount 為該學系錄取總人數
	StudentCount int `json:"student_count"`
	// Percentage 為佔該學群總人數百分比
	Percentage float64 `json:"percentage"`
}

// DisciplineHighSchoolStat 學群或系所錄取生源高中排行 (用於直方圖)
type DisciplineHighSchoolStat struct {
	// HighSchoolName 為高中名稱
	HighSchoolName string `json:"high_school_name"`
	// City 為所在縣市
	City string `json:"city"`
	// SchoolType 為公私立
	SchoolType string `json:"school_type"`
	// StudentCount 為錄取人數
	StudentCount int `json:"student_count"`
	// Percentage 為佔該學群或系所總錄取人數百分比
	Percentage float64 `json:"percentage"`
}

// FilteredAdmissionsResult 多維度條件篩選結果結構體 (包含條件符合統計與明細清單)
type FilteredAdmissionsResult struct {
	// ActiveConditionCount 為生效的篩選條件維度數量 (例如高中、大學、科系、學年共幾項條件生效)
	ActiveConditionCount int `json:"active_condition_count"`
	// ActiveConditions 為生效之各條件標籤與名稱清單
	ActiveConditions []string `json:"active_conditions"`
	// MatchedRecordsCount 為符合條件之資料記錄總筆數
	MatchedRecordsCount int `json:"matched_records_count"`
	// MatchedTotalStudents 為符合條件之錄取學生總人數
	MatchedTotalStudents int `json:"matched_total_students"`
	// NationalStudents 國立大學錄取人數
	NationalStudents int `json:"national_students"`
	// NationalRate 國立大學錄取比例字串
	NationalRate string `json:"national_rate"`
	// PrivateStudents 私立大學錄取人數
	PrivateStudents int `json:"private_students"`
	// PrivateRate 私立大學錄取比例字串
	PrivateRate string `json:"private_rate"`
	// TechStudents 技專校院 (科大) 錄取人數
	TechStudents int `json:"tech_students"`
	// TechRate 技專校院錄取比例字串
	TechRate string `json:"tech_rate"`
	// TopUnivStudents 頂尖大學 (台清交成政) 錄取人數
	TopUnivStudents int `json:"top_univ_students"`
	// TopUnivRate 頂大錄取比例字串
	TopUnivRate string `json:"top_univ_rate"`
	// MedDentChineseStudents 醫牙中 (醫學+牙醫+中醫) 錄取人數
	MedDentChineseStudents int `json:"med_dent_chinese_students"`
	// MedDentChineseRate 醫牙中錄取比例字串
	MedDentChineseRate string `json:"med_dent_chinese_rate"`
	// Records 為符合條件之詳細升學明細清單
	Records []AdmissionRecord `json:"records"`
}


