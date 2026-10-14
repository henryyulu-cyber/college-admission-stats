package main

import (
	// 引入格式化輸出套件
	"fmt"
	// 引入標準日誌套件
	"log"
	// 引入作業系統與路徑套件
	"os"
	"path/filepath"
	// 引入時間處理套件
	"time"

	// 引入純 Go SQLite 驅動
	_ "modernc.org/sqlite"

	// 引入本專案 collector 與 database 套件
	"college-admission-stats/internal/collector"
	"college-admission-stats/internal/database"
)

// main 為 Docker 建置與雲端發布前全量官方資料庫重建與誠信烘焙進入點
func main() {
	// 記錄起始時間
	startTime := time.Now()

	// 定義資料庫路徑與資料目錄
	dbPath := filepath.Join("data", "admission.db")
	dataDir := filepath.Join("data")

	// 初始化資料庫結構與連線池
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("[致命錯誤] 初始化資料庫結構失敗 %s: %v", dbPath, err)
	}
	defer db.Close()

	dl := collector.NewDownloader(db, dataDir)

	fmt.Println("=========================================================================")
	fmt.Println("  [Docker 烘焙] 教育部統計處母體與公立高中官方榜單全量誠信入庫程序")
	fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// 階段一：匯入教育部統計處 110 ~ 112 學年度大專校院各校科系別學生數母體
	// -------------------------------------------------------------------------
	targetYears := []int{110, 111, 112}
	totalMOERecords := 0

	for _, yr := range targetYears {
		localCSV := filepath.Join(dataDir, fmt.Sprintf("official_moe_%d_students.csv", yr))
		datasetName := fmt.Sprintf("教育部統計處-%d學年度大專校院各校科系別學生數", yr)
		sourceURL := fmt.Sprintf("https://stats.moe.gov.tw/files/detail/%d/%d_students.csv", yr, yr)
		var report *collector.DataVerificationReport
		var pErr error

		// 若本機已有官方 CSV 則直接驗證匯入，避免 Docker 建置時受網路波動影響
		if _, statErr := os.Stat(localCSV); statErr == nil {
			fmt.Printf(">>> 正在使用隨附之官方原始 CSV 校驗匯入教育部 %d 學年度資料...\n", yr)
			report, pErr = dl.ImportVerifiedCSVFile(localCSV, datasetName, yr, sourceURL, "教育部統計處")
		} else {
			fmt.Printf(">>> 正在自教育部統計處官方端點下載並校驗 %d 學年度資料...\n", yr)
			report, pErr = dl.DownloadAndVerifyMOEStudents(yr)
		}

		if pErr != nil {
			log.Fatalf("[致命錯誤] 匯入教育部 %d 學年度官方資料失敗: %v", yr, pErr)
		}
		totalMOERecords += report.ValidRows
		fmt.Printf("    ✔ 成功校驗入庫 %d 筆官方系所母體紀錄 (SHA-256: %s, 狀態: %s)\n", report.ValidRows, report.SHA256Hash[:16]+"...", report.Status)
	}

	// -------------------------------------------------------------------------
	// 階段二：匯入 110 ~ 113 學年度公立高中教務處官方公告錄取榜單真實數據
	// -------------------------------------------------------------------------
	honorYears := []int{110, 111, 112, 113}
	totalHonorRecords := 0

	for _, yr := range honorYears {
		csvPath := filepath.Join(dataDir, fmt.Sprintf("official_honor_rolls_%d.csv", yr))
		if _, statErr := os.Stat(csvPath); statErr != nil {
			fmt.Printf("    ⚠️ 未找到 %d 學年度公立高中榜單 CSV (%s)，跳過\n", yr, csvPath)
			continue
		}

		datasetName := fmt.Sprintf("公立高中教務處官方公告錄取榜單-%d學年度", yr)
		fmt.Printf(">>> 正在校驗並匯入 %d 學年度公立高中官方放榜金榜真實數據...\n", yr)
		hReport, hErr := dl.ImportOfficialHonorRollCSV(
			csvPath,
			yr,
			"https://www.tnfsh.tn.edu.tw/ann/index.php",
			datasetName,
			"公立高中教務處聯合發布",
		)
		if hErr != nil {
			log.Fatalf("[致命錯誤] 匯入 %d 學年度公立高中官方榜單失敗: %v", yr, hErr)
		}
		totalHonorRecords += hReport.ValidRows
		fmt.Printf("    ✔ 成功校驗入庫 %d 筆官方高中升學明細 (SHA-256: %s, 狀態: %s)\n", hReport.ValidRows, hReport.SHA256Hash[:16]+"...", hReport.Status)
	}

	// -------------------------------------------------------------------------
	// 階段三：登記 114 與 115 未來學年度誠信防線 (尚未放榜，維持 0 筆假資料)
	// -------------------------------------------------------------------------
	futureYears := []int{114, 115}
	for _, fYr := range futureYears {
		_, _ = db.Exec(`DELETE FROM data_sources WHERE academic_year = ? AND file_hash = 'UNRELEASED'`, fYr)
		_, _ = db.Exec(`INSERT INTO data_sources (
			source_name, dataset_name, url, downloaded_at, academic_year, http_status,
			file_hash, data_version, verification_status, record_count, error_message, created_at
		) VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, 200, 'UNRELEASED', 'AWAITING_OFFICIAL', 'unverified', 0, '該年度尚未舉行大學升學分發，官方尚無放榜資料', CURRENT_TIMESTAMP)`,
			"教育部與各校教務處", fmt.Sprintf("%d學年度大學多元入學放榜資料", fYr), "https://www.cac.edu.tw/", fYr,
		)
		_, _ = db.Exec(`INSERT INTO verification_logs (
			dataset_name, academic_year, check_step, status, detail, created_at
		) VALUES (?, ?, '未來年度誠信查核', 'WARNING', '該學年度尚未放榜，依規範維持 0 筆純淨狀態，禁止載入推估假資料', CURRENT_TIMESTAMP)`,
			fmt.Sprintf("%d學年度大學升學放榜資料", fYr), fYr,
		)
		fmt.Printf("    🛡️ [%d 學年度] 狀態: 尚未放榜 (AWAITING_OFFICIAL) | 誠信隔離: 0 筆合成假資料\n", fYr)
	}

	elapsed := time.Since(startTime)
	fmt.Println("=========================================================================")
	fmt.Printf("  ✅ 官方大專母體與高中榜單全量烘焙完成！總計入庫 %d 筆真實數據\n", totalMOERecords+totalHonorRecords)
	fmt.Printf("  ⏱️ 總耗時: %v\n", elapsed)
	fmt.Println("=========================================================================")
}
