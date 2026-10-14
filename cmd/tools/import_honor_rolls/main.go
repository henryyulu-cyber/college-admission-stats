package main

import (
	// 引入格式化輸出套件
	"fmt"
	// 引入標準日誌套件
	"log"
	// 引入路徑套件
	"path/filepath"

	// 引入純 Go SQLite 驅動
	_ "modernc.org/sqlite"

	// 引入本專案 collector 與 database 模組
	"college-admission-stats/internal/collector"
	"college-admission-stats/internal/database"
)

func main() {
	dbPath := filepath.Join("data", "admission.db")
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("[致命錯誤] 初始化資料庫失敗: %v", err)
	}
	defer db.Close()

	csvPath := filepath.Join("data", "official_honor_rolls_112.csv")
	downloader := collector.NewDownloader(db, "data")

	log.Printf("[啟動] 開始匯入並校驗公立高中教務處官方公告 112 學年度升學金榜資料: %s", csvPath)
	report, err := downloader.ImportOfficialHonorRollCSV(
		csvPath,
		112,
		"https://www.tnfsh.tn.edu.tw/ann/index.php",
		"公立高中教務處官方公告錄取榜單",
		"公立高中教務處聯合發布",
	)
	if err != nil {
		log.Fatalf("[失敗] 匯入公立高中官方榜單失敗: %v", err)
	}

	fmt.Println("================================================================================")
	fmt.Println("  公立高中教務處官方榜單誠信校驗與匯入完成報告")
	fmt.Println("================================================================================")
	fmt.Printf("  ✔ 資料集名稱: %s\n", report.DatasetName)
	fmt.Printf("  ✔ 學年度: %d 學年度\n", report.AcademicYear)
	fmt.Printf("  ✔ 驗證狀態: %s\n", report.Status)
	fmt.Printf("  ✔ SHA-256 數位指紋: %s\n", report.SHA256Hash)
	fmt.Printf("  ✔ 總檢驗列數: %d | 成功入庫筆數: %d | 警告: %d | 錯誤: %d\n", report.TotalRows, report.ValidRows, report.WarningCount, report.ErrorCount)
	fmt.Println("================================================================================")
}
