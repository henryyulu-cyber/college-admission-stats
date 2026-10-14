package collector

import (
	// 引入 SHA-256 雜湊演算法套件
	"crypto/sha256"
	// 引入 SQL 抽象資料庫套件，提供標準資料庫操作介面
	"database/sql"
	// 引入格式化 I/O 套件，用於字串組合與格式化輸出
	"fmt"
	// 引入標準日誌套件，用於記錄系統運行資訊與除錯
	"log"
	// 引入作業系統與路徑套件，用於檔案存取與校驗路徑處理
	"os"
	"path/filepath"
	// 引入字串處理套件，用於 SQL 組合與比對
	"strings"
	// 引入並發同步互斥鎖套件，確保執行緒安全
	"sync"
	// 引入時間處理套件，用於記錄同步時間點
	"time"

	// 引入本專案 models 模型定義
	"college-admission-stats/internal/models"
)

// Downloader 定義官方數據下載、誠信校驗與入庫管線管理器
type Downloader struct {
	// DB 為 SQLite 資料庫連線實例指標
	DB *sql.DB
	// DataDir 為官方資料檔案存放目錄
	DataDir string
	// Verifier 為官方資料誠信驗證器
	Verifier *DataVerifier
	// mu 用於保護同步狀態，防止並發重複執行下載
	mu sync.Mutex
	// isRunning 標記當前是否正在執行數據擷取
	isRunning bool
	// lastSyncTime 記錄最後一次成功同步的時間
	lastSyncTime time.Time
	// lastMessage 記錄最後執行結果摘要
	lastMessage string
}

// NewDownloader 建立並初始化 Downloader 實例指標
func NewDownloader(db *sql.DB, dataDir string) *Downloader {
	// 回傳初始化後的 Downloader 物件指標，同時配置資料誠信驗證器
	return &Downloader{
		DB:          db,
		DataDir:     dataDir,
		Verifier:    NewDataVerifier(),
		lastMessage: "系統已就緒，遵守資料誠信規範，嚴格拒絕合成或隨機假資料",
	}
}

// GetStatus 回傳當前資料庫的統計與同步狀態 (包含官方核實狀態)
func (d *Downloader) GetStatus() (*models.SyncStatus, error) {
	// 上鎖防止並發讀寫衝突
	d.mu.Lock()
	defer d.mu.Unlock()

	// 查詢總紀錄筆數 (排除 synthetic 資料)
	var totalRecords int64
	err := d.DB.QueryRow("SELECT COUNT(*) FROM admissions WHERE verification_status != 'synthetic'").Scan(&totalRecords)
	if err != nil {
		log.Printf("[錯誤] 查詢 admissions 總筆數失敗: %v", err)
		return nil, err
	}

	// 查詢現有學年度清單
	rows, err := d.DB.Query("SELECT DISTINCT academic_year FROM admissions WHERE verification_status != 'synthetic' ORDER BY academic_year ASC")
	if err != nil {
		log.Printf("[錯誤] 查詢學年度清單失敗: %v", err)
		return nil, err
	}
	defer rows.Close()

	var years []int
	for rows.Next() {
		var yr int
		if err := rows.Scan(&yr); err == nil {
			years = append(years, yr)
		}
	}

	// 查詢涵蓋高中職總數
	var hsCount int
	_ = d.DB.QueryRow("SELECT COUNT(DISTINCT high_school_name) FROM admissions WHERE verification_status != 'synthetic'").Scan(&hsCount)

	// 查詢涵蓋大學院校總數
	var univCount int
	_ = d.DB.QueryRow("SELECT COUNT(DISTINCT university_name) FROM admissions WHERE verification_status != 'synthetic'").Scan(&univCount)

	// 組裝並回傳狀態結構體
	return &models.SyncStatus{
		TotalRecords:    totalRecords,
		YearsAvailable:  years,
		HighSchoolCount: hsCount,
		UniversityCount: univCount,
		LastSyncTime:    d.lastSyncTime,
		IsSyncing:       d.isRunning,
		LastMessage:     d.lastMessage,
	}, nil
}

// GetDataSources 取得所有已登記之官方資料來源清單
func (d *Downloader) GetDataSources() ([]models.DataSource, error) {
	query := `SELECT id, source_name, dataset_name, url, downloaded_at, academic_year, 
	                 http_status, file_hash, data_version, verification_status, record_count, error_message, created_at 
	          FROM data_sources ORDER BY academic_year DESC, downloaded_at DESC`
	rows, err := d.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查詢官方資料來源失敗: %w", err)
	}
	defer rows.Close()

	var list []models.DataSource
	for rows.Next() {
		var ds models.DataSource
		if err := rows.Scan(
			&ds.ID, &ds.SourceName, &ds.DatasetName, &ds.URL, &ds.DownloadedAt, &ds.AcademicYear,
			&ds.HTTPStatus, &ds.FileHash, &ds.DataVersion, &ds.VerificationStatus, &ds.RecordCount, &ds.ErrorMessage, &ds.CreatedAt,
		); err == nil {
			list = append(list, ds)
		}
	}
	return list, nil
}

// GetVerificationLogs 取得最新之資料誠信校驗日誌
func (d *Downloader) GetVerificationLogs(limit int) ([]models.VerificationLog, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT id, dataset_name, academic_year, check_step, status, detail, 
	                 total_rows_checked, valid_rows_count, warning_rows_count, error_rows_count, created_at 
	          FROM verification_logs ORDER BY created_at DESC LIMIT ?`
	rows, err := d.DB.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("查詢檢驗日誌失敗: %w", err)
	}
	defer rows.Close()

	var logs []models.VerificationLog
	for rows.Next() {
		var l models.VerificationLog
		if err := rows.Scan(
			&l.ID, &l.DatasetName, &l.AcademicYear, &l.CheckStep, &l.Status, &l.Detail,
			&l.TotalRowsChecked, &l.ValidRowsCount, &l.WarningRowsCount, &l.ErrorRowsCount, &l.CreatedAt,
		); err == nil {
			logs = append(logs, l)
		}
	}
	return logs, nil
}

// ImportVerifiedCSVFile 透過誠信驗證器匯入本機或已下載之官方 CSV 檔案
func (d *Downloader) ImportVerifiedCSVFile(
	filePath string,
	datasetName string,
	academicYear int,
	sourceURL string,
	sourceName string,
) (*DataVerificationReport, error) {
	// 檢查檔案是否存在
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("讀取官方資料檔案失敗 (%s): %w", filePath, err)
	}

	// 建立模擬或本機下載結果
	downloadResult := &DownloadResult{
		StatusCode:   200,
		ContentType:  "text/csv; charset=utf-8",
		BodyBytes:    content,
		DownloadedAt: time.Now(),
	}
	// 利用驗證器計算真實 SHA-256
	hasher := d.Verifier.HTTPClient
	_ = hasher
	// 計算位元組 SHA-256
	downloadResult.SHA256Hash = fmt.Sprintf("%x", calculateSHA256(content))

	// 呼叫驗證器解析與全欄位校驗
	rows, report, err := d.Verifier.VerifyAndParseCSV(downloadResult, datasetName, academicYear, sourceURL)
	if err != nil {
		// 記錄校驗失敗日誌至資料庫
		d.recordVerificationLog(datasetName, academicYear, "檔案校驗未通過", "FAILED", err.Error(), 0, 0, 0, 1)
		return report, fmt.Errorf("官方資料校驗失敗，中止匯入: %w", err)
	}

	// 開啟資料庫交易批次寫入
	tx, err := d.DB.Begin()
	if err != nil {
		return report, fmt.Errorf("開啟資料庫交易失敗: %w", err)
	}
	defer tx.Rollback()

	// 清除同年度同一資料集舊有已核實紀錄 (支援冪等性更新)
	_, _ = tx.Exec("DELETE FROM admissions WHERE academic_year = ? AND source_dataset = ?", academicYear, datasetName)

	now := time.Now()
	batchSize := 100
	totalLen := len(rows)

	for i := 0; i < totalLen; i += batchSize {
		end := i + batchSize
		if end > totalLen {
			end = totalLen
		}
		chunk := rows[i:end]

		var valueStrings []string
		var valueArgs []interface{}
		for _, r := range chunk {
			valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			valueArgs = append(valueArgs,
				r.AcademicYear, r.HighSchoolCode, r.HighSchoolName, r.HighSchoolCity, r.HighSchoolType,
				r.UniversityCode, r.UniversityName, r.UniversitySys, r.UniversityType,
				r.DisciplineGrp, r.DepartmentName, r.StudentCount,
				sourceURL, datasetName, downloadResult.DownloadedAt, downloadResult.SHA256Hash, report.Status, now,
			)
		}

		batchSQL := fmt.Sprintf(`INSERT INTO admissions (
			academic_year, high_school_code, high_school_name, high_school_city, high_school_type,
			university_code, university_name, university_system, university_type,
			discipline_group, department_name, student_count,
			source_url, source_dataset, source_downloaded_at, source_hash, verification_status, created_at
		) VALUES %s`, strings.Join(valueStrings, ","))

		if _, err := tx.Exec(batchSQL, valueArgs...); err != nil {
			return report, fmt.Errorf("寫入已校驗資料失敗: %w", err)
		}
	}

	// 登記至 data_sources 表
	dsQuery := `INSERT INTO data_sources (
		source_name, dataset_name, url, downloaded_at, academic_year, http_status,
		file_hash, data_version, verification_status, record_count, error_message, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(dsQuery,
		sourceName, datasetName, sourceURL, downloadResult.DownloadedAt, academicYear, 200,
		downloadResult.SHA256Hash, time.Now().Format("2006-01-02"), report.Status, len(rows), "", now,
	)
	if err != nil {
		return report, fmt.Errorf("登記官方資料來源失敗: %w", err)
	}

	// 提交交易
	if err := tx.Commit(); err != nil {
		return report, fmt.Errorf("提交資料庫交易失敗: %w", err)
	}

	// 登記校驗日誌
	for _, l := range report.Logs {
		d.recordVerificationLog(l.DatasetName, l.AcademicYear, l.CheckStep, l.Status, l.Detail, l.TotalRowsChecked, l.ValidRowsCount, l.WarningRowsCount, l.ErrorRowsCount)
	}

	log.Printf("[成功] 官方資料集 '%s' (%d 學年度) 成功匯入 %d 筆紀錄，SHA-256: %s", datasetName, academicYear, len(rows), downloadResult.SHA256Hash)
	return report, nil
}

// DownloadAndVerifyOfficialURL 自官方開放端點下載並進行完整校驗匯入
func (d *Downloader) DownloadAndVerifyOfficialURL(
	url string,
	datasetName string,
	academicYear int,
	sourceName string,
) (*DataVerificationReport, error) {
	// 透過驗證器自網路端點下載
	dlResult, err := d.Verifier.DownloadOfficialData(url)
	if err != nil {
		// 依誠信規範：API 失敗時停止匯入並記錄錯誤，絕不改用模擬或假資料
		d.recordVerificationLog(datasetName, academicYear, "HTTP下載請求", "FAILED", err.Error(), 0, 0, 0, 1)
		return nil, fmt.Errorf("官方端點下載失敗: %w", err)
	}

	// 將原始下載檔案備份至 data 目錄保留稽核依據
	localFileName := fmt.Sprintf("official_%s_%d.csv", sanitizeFilename(datasetName), academicYear)
	localPath := filepath.Join(d.DataDir, localFileName)
	_ = os.WriteFile(localPath, dlResult.BodyBytes, 0644)

	// 執行匯入與校驗流程
	return d.ImportVerifiedCSVFile(localPath, datasetName, academicYear, url, sourceName)
}

// DownloadAndVerifyMOEStudents 自教育部統計處端點下載大專校院科系別學生數官方 CSV 並校驗入庫
func (d *Downloader) DownloadAndVerifyMOEStudents(year int) (*DataVerificationReport, error) {
	// 教育部官方固定開放 CSV 下載網址
	url := fmt.Sprintf("https://stats.moe.gov.tw/files/detail/%d/%d_students.csv", year, year)
	datasetName := fmt.Sprintf("教育部統計處-%d學年度大專校院各校科系別學生數", year)
	sourceName := "教育部統計處"

	log.Printf("[官方下載] 正在自教育部統計處下載 %d 學年度大專校院科系學生數: %s ...", year, url)

	// 步驟 1：下載並計算 SHA-256
	dlResult, err := d.Verifier.DownloadOfficialData(url)
	if err != nil {
		d.recordVerificationLog(datasetName, year, "HTTP下載請求", "FAILED", err.Error(), 0, 0, 0, 1)
		return nil, fmt.Errorf("下載教育部官方資料失敗 (%s): %w", url, err)
	}

	// 步驟 2：備份原始下載檔案至本機 data 目錄以備稽核
	localFileName := fmt.Sprintf("official_moe_%d_students.csv", year)
	localPath := filepath.Join(d.DataDir, localFileName)
	_ = os.WriteFile(localPath, dlResult.BodyBytes, 0644)

	// 步驟 3：透過專業解析器驗證 CSV 並過濾學士班大一生源
	rows, report, err := d.Verifier.VerifyAndParseMOEUniversityStudentsCSV(dlResult, datasetName, year, url)
	if err != nil {
		d.recordVerificationLog(datasetName, year, "教育部CSV校驗解析", "FAILED", err.Error(), 0, 0, 0, 1)
		return report, fmt.Errorf("教育部 CSV 校驗失敗: %w", err)
	}

	// 步驟 4：開啟資料庫交易批次寫入 admissions 表
	tx, err := d.DB.Begin()
	if err != nil {
		return report, fmt.Errorf("開啟資料庫交易失敗: %w", err)
	}
	defer tx.Rollback()

	// 冪等性防護：先刪除舊有同年度該官方資料集
	_, _ = tx.Exec("DELETE FROM admissions WHERE academic_year = ? AND source_url = ?", year, url)

	now := time.Now()
	batchSize := 100
	totalLen := len(rows)

	for i := 0; i < totalLen; i += batchSize {
		end := i + batchSize
		if end > totalLen {
			end = totalLen
		}
		chunk := rows[i:end]

		var valueStrings []string
		var valueArgs []interface{}
		for _, r := range chunk {
			valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			valueArgs = append(valueArgs,
				r.AcademicYear, r.HighSchoolCode, r.HighSchoolName, r.HighSchoolCity, r.HighSchoolType,
				r.UniversityCode, r.UniversityName, r.UniversitySys, r.UniversityType,
				r.DisciplineGrp, r.DepartmentName, r.StudentCount,
				url, datasetName, dlResult.DownloadedAt, dlResult.SHA256Hash, report.Status, now,
			)
		}

		batchSQL := fmt.Sprintf(`INSERT INTO admissions (
			academic_year, high_school_code, high_school_name, high_school_city, high_school_type,
			university_code, university_name, university_system, university_type,
			discipline_group, department_name, student_count,
			source_url, source_dataset, source_downloaded_at, source_hash, verification_status, created_at
		) VALUES %s`, strings.Join(valueStrings, ","))

		if _, err := tx.Exec(batchSQL, valueArgs...); err != nil {
			return report, fmt.Errorf("批次寫入教育部系所資料失敗: %w", err)
		}
	}

	// 登記至 data_sources 表
	dsQuery := `INSERT INTO data_sources (
		source_name, dataset_name, url, downloaded_at, academic_year, http_status,
		file_hash, data_version, verification_status, record_count, error_message, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(dsQuery,
		sourceName, datasetName, url, dlResult.DownloadedAt, year, 200,
		dlResult.SHA256Hash, time.Now().Format("2006-01-02"), report.Status, len(rows), "", now,
	)
	if err != nil {
		return report, fmt.Errorf("登記 data_sources 失敗: %w", err)
	}

	// 提交交易
	if err := tx.Commit(); err != nil {
		return report, fmt.Errorf("提交教育部系所資料庫交易失敗: %w", err)
	}

	// 登記校驗日誌
	for _, l := range report.Logs {
		d.recordVerificationLog(l.DatasetName, l.AcademicYear, l.CheckStep, l.Status, l.Detail, l.TotalRowsChecked, l.ValidRowsCount, l.WarningRowsCount, l.ErrorRowsCount)
	}

	log.Printf("[成功] 教育部 %d 學年度大專校院科系開放資料成功入庫: %d 筆系所，SHA-256: %s", year, len(rows), dlResult.SHA256Hash)
	return report, nil
}

// SyncAllYears 執行官方資料同步作業 (嚴格遵循資料誠信原則，自動自教育部開放端點抓取已核實資料)
func (d *Downloader) SyncAllYears() error {
	d.mu.Lock()
	if d.isRunning {
		d.mu.Unlock()
		return fmt.Errorf("資料同步作業正在背景執行中，請稍候")
	}
	d.isRunning = true
	d.lastMessage = "正在連線教育部開放資料平臺查核官方資料集..."
	d.mu.Unlock()

	go func() {
		defer func() {
			d.mu.Lock()
			d.isRunning = false
			d.lastSyncTime = time.Now()
			d.mu.Unlock()
		}()

		log.Printf("[誠信查核] 啟動教育部官方開放資料同步校驗程序 (110~112學年度)...")
		targetYears := []int{110, 111, 112}
		totalImported := 0

		for _, yr := range targetYears {
			rep, err := d.DownloadAndVerifyMOEStudents(yr)
			if err != nil {
				log.Printf("[警告] 下載匯入教育部 %d 學年度失敗: %v", yr, err)
			} else {
				totalImported += rep.ValidRows
			}
		}

		d.mu.Lock()
		if totalImported > 0 {
			d.lastMessage = fmt.Sprintf("教育部官方大專校院系所母體資料同步完成！共校驗寫入 %d 筆官方核實系所紀錄", totalImported)
		} else {
			d.lastMessage = "連線教育部資料庫未取得新筆數，維持現有狀態。"
		}
		d.mu.Unlock()
		log.Printf("[誠信日誌] %s", d.lastMessage)
	}()

	return nil
}

// ImportOfficialHonorRollCSV 將公立高中教務處官方榜單解析並依最高誠信原則寫入 admissions 資料表
func (d *Downloader) ImportOfficialHonorRollCSV(
	filePath string,
	expectedYear int,
	defaultSourceURL string,
	defaultDatasetName string,
	sourceName string,
) (*DataVerificationReport, error) {
	// 建立專用榜單誠信匯入解析器
	importer := NewHonorRollImporter()

	// 執行檔案解析與名冊校驗
	records, report, err := importer.ParseAndVerifyOfficialHonorRollCSV(filePath, expectedYear, defaultSourceURL, defaultDatasetName)
	if err != nil {
		d.recordVerificationLog(defaultDatasetName, expectedYear, "公立高中官方榜單解析", "FAILED", err.Error(), 0, 0, 0, 1)
		return report, fmt.Errorf("官方高中榜單校驗失敗: %w", err)
	}

	// 開啟資料庫交易批次寫入
	tx, err := d.DB.Begin()
	if err != nil {
		return report, fmt.Errorf("開啟資料庫交易失敗: %w", err)
	}
	defer tx.Rollback()

	// 冪等性防護：若同年度同資料集已有紀錄，先予以清除再更新
	_, _ = tx.Exec("DELETE FROM admissions WHERE academic_year = ? AND source_dataset = ?", expectedYear, defaultDatasetName)

	now := time.Now()
	batchSize := 100
	totalLen := len(records)

	for i := 0; i < totalLen; i += batchSize {
		end := i + batchSize
		if end > totalLen {
			end = totalLen
		}
		chunk := records[i:end]

		var valueStrings []string
		var valueArgs []interface{}
		for _, r := range chunk {
			valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			valueArgs = append(valueArgs,
				r.AcademicYear, r.HighSchoolCode, r.HighSchoolName, r.HighSchoolCity, r.HighSchoolType,
				r.UniversityCode, r.UniversityName, r.UniversitySys, r.UniversityType,
				r.DisciplineGroup, r.DepartmentName, r.StudentCount,
				r.SourceURL, r.SourceDataset, now, r.SourceHash, r.VerificationStatus, now,
			)
		}

		batchSQL := fmt.Sprintf(`INSERT INTO admissions (
			academic_year, high_school_code, high_school_name, high_school_city, high_school_type,
			university_code, university_name, university_system, university_type,
			discipline_group, department_name, student_count,
			source_url, source_dataset, source_downloaded_at, source_hash, verification_status, created_at
		) VALUES %s`, strings.Join(valueStrings, ","))

		if _, err := tx.Exec(batchSQL, valueArgs...); err != nil {
			return report, fmt.Errorf("寫入高中榜單資料失敗: %w", err)
		}
	}

	// 登記至 data_sources 官方來源主表
	dsQuery := `INSERT INTO data_sources (
		source_name, dataset_name, url, downloaded_at, academic_year, http_status,
		file_hash, data_version, verification_status, record_count, error_message, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.Exec(dsQuery,
		sourceName, defaultDatasetName, defaultSourceURL, now, expectedYear, 200,
		report.SHA256Hash, time.Now().Format("2006-01-02"), report.Status, len(records), "", now,
	)
	if err != nil {
		return report, fmt.Errorf("登記官方高中資料來源失敗: %w", err)
	}

	// 提交交易
	if err := tx.Commit(); err != nil {
		return report, fmt.Errorf("提交高中榜單交易失敗: %w", err)
	}

	// 登記檢驗日誌
	for _, l := range report.Logs {
		d.recordVerificationLog(l.DatasetName, l.AcademicYear, l.CheckStep, l.Status, l.Detail, l.TotalRowsChecked, l.ValidRowsCount, l.WarningRowsCount, l.ErrorRowsCount)
	}

	log.Printf("[成功] 公立高中教務處官方榜單 '%s' (%d 學年度) 成功寫入 %d 筆真實升學紀錄，SHA-256: %s", defaultDatasetName, expectedYear, len(records), report.SHA256Hash)
	return report, nil
}

// recordVerificationLog 記錄校驗歷程至 verification_logs 資料表
func (d *Downloader) recordVerificationLog(datasetName string, year int, step, status, detail string, total, valid, warn, errCount int) {
	query := `INSERT INTO verification_logs (
		dataset_name, academic_year, check_step, status, detail,
		total_rows_checked, valid_rows_count, warning_rows_count, error_rows_count, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, _ = d.DB.Exec(query, datasetName, year, step, status, detail, total, valid, warn, errCount, time.Now())
}

// calculateSHA256 使用 crypto/sha256 計算位元組之 SHA-256 數位指紋
func calculateSHA256(data []byte) [32]byte {
	// 直接呼叫標準庫 Sum256 進行計算
	return sha256.Sum256(data)
}

// ProcessYearDirect 供外部直接調用單一學年度同步，若無官方資料則回傳 0 筆與日誌提示 (嚴格禁止合成假資料)
func (d *Downloader) ProcessYearDirect(year int) (int, int, error) {
	// 記錄查核訊息
	log.Printf("[誠信提示] ProcessYearDirect (%d 學年度): 官方未開放微觀升學名冊，依規範不載入任何合成資料", year)
	return 0, 0, nil
}

// sanitizeFilename 清理檔案名稱中的非法字元
func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}
