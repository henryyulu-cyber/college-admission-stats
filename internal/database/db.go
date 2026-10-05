package database

import (
	// 引入 SQL 標準抽象介面庫
	"database/sql"
	// 引入格式化輸出與錯誤處理庫
	"fmt"
	// 引入標準日誌記錄庫
	"log"
	// 引入時間處理庫
	"time"

	// 引入純 Go 實作的 SQLite 驅動，不需 GCC 編譯器或外部 C 依賴庫
	_ "modernc.org/sqlite"
)

// InitDB 負責建立 SQLite 連線池、配置 WAL 效能模式與建立資料表結構及複合索引
func InitDB(dbPath string) (*sql.DB, error) {
	// 開啟 SQLite 資料庫連線，資料庫路徑來自 config 設定
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		// 若開啟連線發生錯誤，記錄日誌並回傳詳細錯誤
		log.Printf("[錯誤] 無法開啟 SQLite 資料庫 (%s): %v", dbPath, err)
		return nil, fmt.Errorf("開啟資料庫失敗: %w", err)
	}

	// 設定最大開啟連線數為 25，避免並發查詢時資源耗盡
	db.SetMaxOpenConns(25)
	// 設定最大閒置連線數為 10，減少連線重複建立的開銷
	db.SetMaxIdleConns(10)
	// 設定連線最長存活時間為 1 小時
	db.SetConnMaxLifetime(1 * time.Hour)

	// 測試資料庫連線是否通暢 (Ping)
	if err := db.Ping(); err != nil {
		log.Printf("[錯誤] 資料庫 Ping 測試失敗: %v", err)
		return nil, fmt.Errorf("資料庫連線失敗: %w", err)
	}

	// 啟用 WAL (Write-Ahead Logging) 模式，大幅提升並發讀寫效能
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("[警告] 設定 WAL 模式失敗: %v", err)
	}
	// 設定 busy_timeout 為 5000 毫秒，防止多執行緒寫入時發生 database is locked 錯誤
	if _, err := db.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		log.Printf("[警告] 設定 busy_timeout 失敗: %v", err)
	}
	// 設定 synchronous 模式為 NORMAL，在確保資料安全的前提下獲得極佳磁碟寫入速度
	if _, err := db.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		log.Printf("[警告] 設定 synchronous 失敗: %v", err)
	}

	// 執行資料表結構建立
	if err := createTables(db); err != nil {
		return nil, err
	}

	log.Printf("[成功] SQLite 資料庫初始化完成，路徑: %s", dbPath)
	return db, nil
}

// createTables 建立 admissions (升學明細表) 與 sync_logs (同步日誌表) 以及複合索引
func createTables(db *sql.DB) error {
	// 定義資料庫 Schema 的 DDL 語法
	schema := `
	-- 升學明細主資料表：儲存高中錄取各大學系所人數
	CREATE TABLE IF NOT EXISTS admissions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,                      -- 自動遞增主鍵
		academic_year INTEGER NOT NULL,                            -- 學年度 (如 110, 111, 112, 113, 114, 115)
		high_school_code TEXT NOT NULL DEFAULT '',                 -- 高中學校代碼
		high_school_name TEXT NOT NULL,                            -- 高中學校全稱
		high_school_city TEXT NOT NULL DEFAULT '',                 -- 高中所在縣市
		high_school_type TEXT NOT NULL DEFAULT '公立',              -- 高中公私立 (公立/私立)
		university_code TEXT NOT NULL DEFAULT '',                  -- 大學校院代碼
		university_name TEXT NOT NULL,                             -- 大學校院全稱
		university_system TEXT NOT NULL DEFAULT '一般大學',          -- 大學體系 (一般大學/技專校院)
		university_type TEXT NOT NULL DEFAULT '國立',               -- 大學公私立 (國立/私立)
		discipline_group TEXT NOT NULL DEFAULT '未分類',            -- 教育部 18 大學群名稱
		department_name TEXT NOT NULL,                             -- 學系/學程名稱
		student_count INTEGER NOT NULL DEFAULT 1,                  -- 錄取/入學學生人數
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP              -- 記錄建立時間
	);

	-- 數據同步日誌表：記錄各學年度資料擷取與更新歷程
	CREATE TABLE IF NOT EXISTS sync_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,                      -- 自動遞增主鍵
		academic_year INTEGER NOT NULL,                            -- 同步學年度
		source_name TEXT NOT NULL,                                 -- 資料來源名稱
		record_count INTEGER NOT NULL DEFAULT 0,                   -- 成功寫入筆數
		status TEXT NOT NULL,                                      -- 同步狀態 ("SUCCESS", "FAILED", "RUNNING")
		message TEXT NOT NULL DEFAULT '',                          -- 執行結果或錯誤訊息
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP              -- 執行時間戳記
	);

	-- 建立複合索引：優化高中與年度聯合查詢效能
	CREATE INDEX IF NOT EXISTS idx_adm_year_hs ON admissions(academic_year, high_school_name);

	-- 建立複合索引：優化大學與年度聯合查詢效能
	CREATE INDEX IF NOT EXISTS idx_adm_year_univ ON admissions(academic_year, university_name);

	-- 建立複合索引：優化 18 大學群與年度交叉統計效能
	CREATE INDEX IF NOT EXISTS idx_adm_year_group ON admissions(academic_year, discipline_group);

	-- 建立複合索引：優化高中與學系查詢效能
	CREATE INDEX IF NOT EXISTS idx_adm_hs_dept ON admissions(high_school_name, department_name);

	-- 建立索引：優化大學與學系搜尋效能
	CREATE INDEX IF NOT EXISTS idx_adm_univ_dept ON admissions(university_name, department_name);

	-- 建立索引：優化高中縣市篩選效能
	CREATE INDEX IF NOT EXISTS idx_adm_hs_city ON admissions(high_school_city);

	-- 建立複合索引：優化醫牙科系與大學性質交叉統計效能
	CREATE INDEX IF NOT EXISTS idx_adm_dept_type ON admissions(department_name, university_type);

	-- 建立複合索引：優化特定科系與大學查詢效能
	CREATE INDEX IF NOT EXISTS idx_adm_dept_univ ON admissions(department_name, university_name);

	-- 建立複合索引：優化學群、學系與學年度三維分析效能
	CREATE INDEX IF NOT EXISTS idx_adm_group_dept_year ON admissions(discipline_group, department_name, academic_year);

	-- 建立複合索引：優化學群生源高中排行統計效能
	CREATE INDEX IF NOT EXISTS idx_adm_group_hs ON admissions(discipline_group, high_school_name);
	`

	// 執行 DDL 語法建立資料表與索引
	if _, err := db.Exec(schema); err != nil {
		log.Printf("[錯誤] 建立資料表或索引失敗: %v", err)
		return fmt.Errorf("建立資料表失敗: %w", err)
	}

	return nil
}
