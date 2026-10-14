package collector

import (
	// 引入 SHA-256 雜湊套件
	"crypto/sha256"
	// 引入 SQL 資料庫套件
	"database/sql"
	// 引入十六進位轉換套件
	"encoding/hex"
	// 引入 HTTP 測試伺服器套件
	"net/http"
	"net/http/httptest"
	// 引入作業系統暫存目錄套件
	"os"
	"path/filepath"
	// 引入單元測試套件
	"testing"
	// 引入時間套件
	"time"

	// 引入純 Go SQLite 驅動
	_ "modernc.org/sqlite"

	// 引入本專案資料模型與資料庫套件
	"college-admission-stats/internal/database"
	"college-admission-stats/internal/models"
)

// setupTestDB 建立記憶體或暫存 SQLite 資料庫供單元測試使用
func setupTestDB(t *testing.T) (*sql.DB, string, func()) {
	// 建立暫存目錄
	tmpDir, err := os.MkdirTemp("", "adm_test_*")
	if err != nil {
		t.Fatalf("建立暫存目錄失敗: %v", err)
	}
	dbPath := filepath.Join(tmpDir, "test_admission.db")

	// 初始化資料庫連線與 Schema
	db, err := database.InitDB(dbPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("初始化測試資料庫失敗: %v", err)
	}

	// 回傳資料庫連線、目錄與清理函式
	cleanup := func() {
		db.Close()
		_ = os.RemoveAll(tmpDir)
	}
	return db, tmpDir, cleanup
}

// TestAPI_Failure_DoesNotWriteToDB 測試當官方 API 或檔案下載失敗時，程式停止匯入且絕不寫入資料庫
func TestAPI_Failure_DoesNotWriteToDB(t *testing.T) {
	db, tmpDir, cleanup := setupTestDB(t)
	defer cleanup()

	// 建立模擬官方伺服器，回傳 HTTP 500 伺服器錯誤
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("500 Internal Server Error: 官方端點維護中"))
	}))
	defer server.Close()

	// 建立 Downloader
	dl := NewDownloader(db, tmpDir)

	// 呼叫下載與驗證函式
	report, err := dl.DownloadAndVerifyOfficialURL(server.URL, "教育部官方測試資料集", 112, "教育部開放平臺")

	// 斷言 1：下載必須失敗並回傳錯誤，嚴禁 fallback 轉用假資料
	if err == nil {
		t.Fatalf("預期下載失敗但未回傳錯誤 (report: %+v)", report)
	}

	// 斷言 2：資料庫 admissions 表筆數必須維持為 0
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM admissions").Scan(&count)
	if err != nil {
		t.Fatalf("查詢資料庫失敗: %v", err)
	}
	if count != 0 {
		t.Fatalf("預期資料庫筆數為 0，但發現寫入 %d 筆資料，違反資料誠信規範！", count)
	}

	// 斷言 3：verification_logs 必須確實記錄失敗原因
	var logCount int
	err = db.QueryRow("SELECT COUNT(*) FROM verification_logs WHERE status = 'FAILED'").Scan(&logCount)
	if err != nil || logCount == 0 {
		t.Fatalf("預期 verification_logs 記錄失敗日誌，但未發現 (logCount=%d, err=%v)", logCount, err)
	}
}

// TestMissingRequiredFields_RejectsImport 測試當官方資料欄位缺失時，拒絕匯入並回傳錯誤
func TestMissingRequiredFields_RejectsImport(t *testing.T) {
	// 缺少學校名稱欄位之異常 CSV
	badCSV := "學年度,科系名稱,人數\n112,資訊工程學系,50\n"

	verifier := NewDataVerifier()
	dlResult := &DownloadResult{
		StatusCode:   200,
		ContentType:  "text/csv",
		BodyBytes:    []byte(badCSV),
		DownloadedAt: time.Now(),
		SHA256Hash:   "dummy_hash",
	}

	_, report, err := verifier.VerifyAndParseCSV(dlResult, "欄位缺失測試", 112, "http://test.gov.tw/missing")

	// 斷言：必須拒絕匯入
	if err == nil {
		t.Fatalf("缺少必要學校欄位時預期應回傳錯誤，但回傳 nil (report: %+v)", report)
	}
	if report.Status == models.VerificationStatusVerifiedOfficial {
		t.Fatalf("欄位缺失時驗證狀態不應為 verified_official (實際狀態: %s)", report.Status)
	}
}

// TestNegativeStudentCount_RejectsImport 測試人數出現負數時，必須認定為致命錯誤並拒絕匯入
func TestNegativeStudentCount_RejectsImport(t *testing.T) {
	// 人數為 -10 之違反物理意義 CSV
	negativeCSV := "學年度,大學校院全稱,學系/學程名稱,錄取學生人數\n112,國立臺灣大學,資訊工程學系,-10\n"

	verifier := NewDataVerifier()
	dlResult := &DownloadResult{
		StatusCode:   200,
		ContentType:  "text/csv",
		BodyBytes:    []byte(negativeCSV),
		DownloadedAt: time.Now(),
		SHA256Hash:   "dummy_hash",
	}

	_, report, err := verifier.VerifyAndParseCSV(dlResult, "負數檢測測試", 112, "http://test.gov.tw/negative")

	// 斷言：必須拒絕匯入
	if err == nil {
		t.Fatalf("人數為負數時預期應回傳錯誤，但回傳 nil")
	}
	if report.ErrorCount == 0 {
		t.Fatalf("預期 ErrorCount > 0，但為 0")
	}
}

// TestSyntheticData_ForbiddenInFormalDatabase 測試 synthetic 測試假資料被嚴格拒絕載入
func TestSyntheticData_ForbiddenInFormalDatabase(t *testing.T) {
	verifier := NewDataVerifier()

	// 模擬帶有 synthetic 標籤之 JSON 資料
	syntheticJSON := []byte(`[{"high_school_name":"測試高中","student_count":10,"verification_status":"synthetic"}]`)

	err := verifier.VerifyJSONAndCheckNoSynthetic(syntheticJSON)
	if err == nil {
		t.Fatalf("含有 synthetic 標記的資料預期應被嚴格拒絕，但卻通過了檢驗！")
	}
}

// TestTraceableSourceInformation 測試正常匯入流程中，每筆正式資料都有可追溯來源與相符的 SHA-256
func TestTraceableSourceInformation(t *testing.T) {
	db, tmpDir, cleanup := setupTestDB(t)
	defer cleanup()

	// 合法合規的官方 CSV 格式內容
	validCSV := "學年度,高中學校全稱,大學校院全稱,學系/學程名稱,錄取學生人數\n112,臺北市立建國高級中學,國立臺灣大學,資訊工程學系,35\n112,臺北市立建國高級中學,國立臺灣大學,電機工程學系,42\n"

	// 計算預期之 SHA-256 雜湊
	expectedHasher := sha256.New()
	expectedHasher.Write([]byte(validCSV))
	expectedHash := hex.EncodeToString(expectedHasher.Sum(nil))

	// 建立模擬官方伺服器提供正確下載
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validCSV))
	}))
	defer server.Close()

	dl := NewDownloader(db, tmpDir)

	// 執行官方端點下載、校驗與入庫
	report, err := dl.DownloadAndVerifyOfficialURL(server.URL, "教育部大專校院系所學生數", 112, "教育部統計處")
	if err != nil {
		t.Fatalf("合規資料下載與驗證失敗: %v", err)
	}

	// 斷言 1：報告中的 SHA-256 必須與檔案內容精確吻合
	if report.SHA256Hash != expectedHash {
		t.Fatalf("SHA-256 雜湊不吻合！預期: %s, 實際: %s", expectedHash, report.SHA256Hash)
	}
	if report.ValidRows != 2 {
		t.Fatalf("預期有效筆數 2，實際: %d", report.ValidRows)
	}

	// 斷言 2：資料庫中每筆 admissions 紀錄都有可追溯來源
	rows, err := db.Query("SELECT source_url, source_dataset, source_hash, verification_status FROM admissions WHERE academic_year = 112")
	if err != nil {
		t.Fatalf("查詢已匯入資料失敗: %v", err)
	}
	defer rows.Close()

	checkedCount := 0
	for rows.Next() {
		var srcURL, srcDataset, srcHash, verStatus string
		if err := rows.Scan(&srcURL, &srcDataset, &srcHash, &verStatus); err != nil {
			t.Fatalf("掃描欄位失敗: %v", err)
		}
		if srcURL != server.URL {
			t.Fatalf("溯源 URL 不符: %s", srcURL)
		}
		if srcDataset != "教育部大專校院系所學生數" {
			t.Fatalf("溯源資料集名稱不符: %s", srcDataset)
		}
		if srcHash != expectedHash {
			t.Fatalf("溯源 SHA-256 雜湊不符: %s", srcHash)
		}
		if verStatus != models.VerificationStatusVerifiedOfficial {
			t.Fatalf("驗證狀態不符: 預期 verified_official, 實際 %s", verStatus)
		}
		checkedCount++
	}

	if checkedCount != 2 {
		t.Fatalf("查核筆數不符，預期 2 筆，實際查核 %d 筆", checkedCount)
	}

	// 斷言 3：data_sources 表已登記該資料集
	var dsCount int
	err = db.QueryRow("SELECT COUNT(*) FROM data_sources WHERE file_hash = ?", expectedHash).Scan(&dsCount)
	if err != nil || dsCount != 1 {
		t.Fatalf("data_sources 表未正確登記官方資料來源 (dsCount=%d)", dsCount)
	}
}

// TestQueryExcludesSynthetic 測試查詢狀態與統計時，嚴格排除 synthetic 資料
func TestQueryExcludesSynthetic(t *testing.T) {
	db, tmpDir, cleanup := setupTestDB(t)
	defer cleanup()

	// 寫入一筆官方已核實資料
	_, err := db.Exec(`INSERT INTO admissions (
		academic_year, high_school_name, university_name, department_name, student_count, verification_status
	) VALUES (112, '臺北市立建國高級中學', '國立臺灣大學', '資訊工程學系', 10, 'verified_official')`)
	if err != nil {
		t.Fatalf("寫入正式資料失敗: %v", err)
	}

	// 寫入一筆測試用 synthetic 資料
	_, err = db.Exec(`INSERT INTO admissions (
		academic_year, high_school_name, university_name, department_name, student_count, verification_status
	) VALUES (112, '假測試高中', '假測試大學', '假系所', 999, 'synthetic')`)
	if err != nil {
		t.Fatalf("寫入合成資料失敗: %v", err)
	}

	dl := NewDownloader(db, tmpDir)
	status, err := dl.GetStatus()
	if err != nil {
		t.Fatalf("取得狀態失敗: %v", err)
	}

	// 斷言：總筆數只能是 1 (排除 synthetic 筆數)
	if status.TotalRecords != 1 {
		t.Fatalf("預期正式記錄筆數為 1 (排除 synthetic)，但實際計算為: %d", status.TotalRecords)
	}
	if status.HighSchoolCount != 1 {
		t.Fatalf("預期高中數為 1，但實際為: %d", status.HighSchoolCount)
	}
}
