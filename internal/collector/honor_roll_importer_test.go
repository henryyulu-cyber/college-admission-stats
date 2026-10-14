package collector

import (
	// 引入作業系統與暫存目錄套件
	"os"
	"path/filepath"
	// 引入單元測試套件
	"testing"

	// 引入純 Go SQLite 驅動
	_ "modernc.org/sqlite"

	// 引入資料庫模型
	"college-admission-stats/internal/models"
)

// TestHonorRollImporter_ValidData 測試合法之公立高中教務處官方榜單解析與校驗
func TestHonorRollImporter_ValidData(t *testing.T) {
	// 建立暫存測試檔案
	tmpDir, err := os.MkdirTemp("", "honor_test_*")
	if err != nil {
		t.Fatalf("建立暫存目錄失敗: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	validCSV := `學年度,高中學校全稱,大學校院全稱,學系/學程名稱,錄取學生人數,官方公告網址,官方資料集名稱
112,國立臺南第一高級中學,國立臺灣大學,醫學系,2,https://www.tnfsh.tn.edu.tw/ann/index.php,國立臺南第一高級中學112學年度大學多元入學錄取榜單
112,臺北市立建國高級中學,國立臺灣大學,醫學系,22,https://www.ck.tp.edu.tw/ann/index.php,臺北市立建國高級中學112學年度高三升大學錄取統計
112,臺北市立第一女子高級中學,國立臺灣大學,電機工程學系,18,https://www.fg.tp.edu.tw/ann/index.php,臺北市立第一女子高級中學112學年度升學統計名單`

	csvPath := filepath.Join(tmpDir, "test_honor_rolls.csv")
	if err := os.WriteFile(csvPath, []byte(validCSV), 0644); err != nil {
		t.Fatalf("寫入測試 CSV 失敗: %v", err)
	}

	importer := NewHonorRollImporter()
	records, report, err := importer.ParseAndVerifyOfficialHonorRollCSV(
		csvPath, 112,
		"https://official.gov.tw/default",
		"公立高中教務處官方測試榜單",
	)

	// 斷言 1：校驗必須成功
	if err != nil {
		t.Fatalf("預期校驗成功但發生錯誤: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("預期解析 3 筆，實際得到: %d 筆", len(records))
	}

	// 斷言 2：驗證狀態必須為 verified_official
	if report.Status != models.VerificationStatusVerifiedOfficial {
		t.Fatalf("預期狀態為 verified_official，實際得到: %s", report.Status)
	}

	// 斷言 3：臺南一中臺大醫學系人數為 2
	tnfshMed := records[0]
	if tnfshMed.HighSchoolName != "國立臺南第一高級中學" || tnfshMed.UniversityName != "國立臺灣大學" || tnfshMed.DepartmentName != "醫學系" || tnfshMed.StudentCount != 2 {
		t.Fatalf("臺南一中資料欄位不正確: %+v", tnfshMed)
	}

	// 斷言 4：建國中學臺大醫學系人數為 22
	ckMed := records[1]
	if ckMed.HighSchoolName != "臺北市立建國高級中學" || ckMed.StudentCount != 22 {
		t.Fatalf("建中資料欄位不正確: %+v", ckMed)
	}

	// 斷言 5：SHA-256 必須存在
	if len(report.SHA256Hash) != 64 {
		t.Fatalf("SHA-256 雜湊格式不正確: %s", report.SHA256Hash)
	}
}

// TestHonorRollImporter_NegativeCountRejection 測試負數人數必須直接被拒絕
func TestHonorRollImporter_NegativeCountRejection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "honor_neg_*")
	if err != nil {
		t.Fatalf("建立暫存目錄失敗: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	negCSV := `學年度,高中學校全稱,大學校院全稱,學系/學程名稱,錄取學生人數
112,國立臺南第一高級中學,國立臺灣大學,醫學系,-5`

	csvPath := filepath.Join(tmpDir, "test_neg.csv")
	_ = os.WriteFile(csvPath, []byte(negCSV), 0644)

	importer := NewHonorRollImporter()
	_, report, err := importer.ParseAndVerifyOfficialHonorRollCSV(csvPath, 112, "http://test.url", "負數測試")

	if err == nil {
		t.Fatalf("錄取人數為負數時預期應拋出錯誤，但返回 nil")
	}
	if report.ErrorCount == 0 {
		t.Fatalf("預期 ErrorCount 大於 0")
	}
}

// TestHonorRollImporter_MissingFieldRejection 測試缺失必要欄位時必須拒絕匯入
func TestHonorRollImporter_MissingFieldRejection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "honor_miss_*")
	if err != nil {
		t.Fatalf("建立暫存目錄失敗: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	missCSV := `學年度,學系/學程名稱,錄取學生人數
112,醫學系,10`

	csvPath := filepath.Join(tmpDir, "test_miss.csv")
	_ = os.WriteFile(csvPath, []byte(missCSV), 0644)

	importer := NewHonorRollImporter()
	_, _, err = importer.ParseAndVerifyOfficialHonorRollCSV(csvPath, 112, "http://test.url", "欄位缺失測試")

	if err == nil {
		t.Fatalf("缺少高中全稱與大學名稱時預期應報錯，但返回 nil")
	}
}

// TestHonorRollImporter_ImportToDB 測試完整寫入資料庫流程與溯源欄位
func TestHonorRollImporter_ImportToDB(t *testing.T) {
	db, tmpDir, cleanup := setupTestDB(t)
	defer cleanup()

	validCSV := `學年度,高中學校全稱,大學校院全稱,學系/學程名稱,錄取學生人數,官方公告網址,官方資料集名稱
112,國立臺南第一高級中學,國立臺灣大學,醫學系,2,https://www.tnfsh.tn.edu.tw/ann/index.php,國立臺南第一高級中學112學年度大學多元入學錄取榜單`

	csvPath := filepath.Join(tmpDir, "test_db_honor.csv")
	_ = os.WriteFile(csvPath, []byte(validCSV), 0644)

	dl := NewDownloader(db, tmpDir)
	report, err := dl.ImportOfficialHonorRollCSV(
		csvPath, 112,
		"https://www.tnfsh.tn.edu.tw/ann/index.php",
		"國立臺南第一高級中學112學年度大學多元入學錄取榜單",
		"國立臺南第一高級中學教務處",
	)

	if err != nil {
		t.Fatalf("匯入資料庫失敗: %v", err)
	}
	if report.ValidRows != 1 {
		t.Fatalf("預期寫入 1 筆，實際: %d", report.ValidRows)
	}

	// 檢查資料庫內容
	var count int
	var srcURL, srcDataset, verStatus string
	err = db.QueryRow(`SELECT student_count, source_url, source_dataset, verification_status 
		FROM admissions WHERE high_school_name = '國立臺南第一高級中學' AND department_name = '醫學系'`).Scan(&count, &srcURL, &srcDataset, &verStatus)

	if err != nil {
		t.Fatalf("查詢資料庫失敗: %v", err)
	}
	if count != 2 {
		t.Fatalf("預期人數為 2，實際: %d", count)
	}
	if srcURL != "https://www.tnfsh.tn.edu.tw/ann/index.php" {
		t.Fatalf("來源 URL 不符: %s", srcURL)
	}
	if verStatus != models.VerificationStatusVerifiedOfficial {
		t.Fatalf("驗證狀態不符: %s", verStatus)
	}
}
