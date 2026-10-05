package main

import (
	// 引入 SQL 資料庫抽象介面套件
	"database/sql"
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

	// 引入本專案 collector 套件
	"college-admission-stats/internal/collector"
)

// main 為升學數據全量真實性校準重建工具的進入點
func main() {
	// 記錄執行起始時間
	startTime := time.Now()

	// 定義 SQLite 資料庫檔案路徑
	dbPath := filepath.Join("data", "admission.db")
	// 定義資料快取目錄
	dataDir := filepath.Join("data")

	// 開啟 SQLite 連線
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("[致命錯誤] 無法開啟資料庫 %s: %v", dbPath, err)
	}
	defer db.Close()

	// 啟用 SQLite WAL 高效能日誌模式
	_, _ = db.Exec("PRAGMA journal_mode=WAL;")
	_, _ = db.Exec("PRAGMA synchronous=NORMAL;")

	// 初始化 Downloader 數據管理器
	dl := collector.NewDownloader(db, dataDir)

	fmt.Println("=========================================================================")
	fmt.Println("  [Go CLI 工具] 110~115 學年度全台高中升學大數據 真實體制分流全量重構")
	fmt.Println("=========================================================================")

	// 依序處理 110 ~ 115 學年度
	targetYears := []int{110, 111, 112, 113, 114, 115}
	totalInsertedRecords := 0
	totalStudentsCalculated := 0

	for _, yr := range targetYears {
		fmt.Printf(">>> 正在執行 %d 學年度升學體制分流校準入庫...\n", yr)
		count, students, err := dl.ProcessYearDirect(yr)
		if err != nil {
			log.Fatalf("[錯誤] 處理 %d 學年度升學數據失敗: %v", yr, err)
		}
		totalInsertedRecords += count
		totalStudentsCalculated += students
		fmt.Printf("    - 成功寫入 %d 筆升學明細，校驗升學總人數: %d 人\n", count, students)
	}

	elapsed := time.Since(startTime)
	fmt.Println("=========================================================================")
	fmt.Printf("  ✅ 全學年度校準重構完成！共寫入 %d 筆記錄，升學總人次: %d 人\n", totalInsertedRecords, totalStudentsCalculated)
	fmt.Printf("  ⏱️ 總耗時: %v\n", elapsed)
	fmt.Println("=========================================================================")
}
