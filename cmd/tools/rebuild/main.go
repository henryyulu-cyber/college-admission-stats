package main

import (
	// 引入格式化輸出套件
	"fmt"
	// 引入標準日誌套件
	"log"
	// 引入路徑套件
	"path/filepath"
	// 引入時間處理套件
	"time"

	// 引入純 Go SQLite 驅動
	_ "modernc.org/sqlite"

	// 引入本專案 collector 與 database 套件
	"college-admission-stats/internal/collector"
	"college-admission-stats/internal/database"
)

// main 為升學數據全量真實性校準重建工具的進入點
func main() {
	// 記錄執行起始時間
	startTime := time.Now()

	// 定義 SQLite 資料庫檔案路徑
	dbPath := filepath.Join("data", "admission.db")
	// 定義資料快取目錄
	dataDir := filepath.Join("data")

	// 初始化資料庫連線池並自動建立資料表結構 (admissions, sync_logs, 索引等)
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("[致命錯誤] 初始化資料庫結構失敗 %s: %v", dbPath, err)
	}
	defer db.Close()

	// 初始化 Downloader 官方數據管理器與驗證器
	dl := collector.NewDownloader(db, dataDir)

	fmt.Println("=========================================================================")
	fmt.Println("  [Go CLI 工具] 教育部統計處官方開放資料 (大專校院各校科系學生數) 誠信校驗匯入")
	fmt.Println("=========================================================================")

	// 依序處理教育部已公布之 110 ~ 112 學年度官方資料
	targetYears := []int{110, 111, 112}
	totalInsertedRecords := 0

	for _, yr := range targetYears {
		fmt.Printf(">>> 正在自教育部統計處下載並校驗 %d 學年度官方資料...\n", yr)
		report, err := dl.DownloadAndVerifyMOEStudents(yr)
		if err != nil {
			log.Fatalf("[致命錯誤] 匯入教育部 %d 學年度官方資料失敗: %v", yr, err)
		}
		totalInsertedRecords += report.ValidRows
		fmt.Printf("    - 成功校驗入庫 %d 筆官方系所紀錄 (SHA-256: %s, 驗證狀態: %s)\n", report.ValidRows, report.SHA256Hash[:16]+"...", report.Status)
	}

	elapsed := time.Since(startTime)
	fmt.Println("=========================================================================")
	fmt.Printf("  ✅ 教育部官方母體資料庫匯入完成！共寫入 %d 筆真實系所紀錄\n", totalInsertedRecords)
	fmt.Printf("  ⏱️ 總耗時: %v\n", elapsed)
	fmt.Println("=========================================================================")
}
