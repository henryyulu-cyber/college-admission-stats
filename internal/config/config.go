package config

import (
	// 引入標準日誌套件，用於輸出系統啟動資訊
	"log"
	// 引入作業系統介面套件，用於讀取環境變數
	"os"
	// 引入字串轉換套件，用於將文字轉換為整數驗證埠號
	"strconv"

	// 引入 godotenv 套件，自動讀取 .env 檔案
	"github.com/joho/godotenv"
)

// Config 結構體定義系統所需的各項組態參數
type Config struct {
	// Port 為 Web 伺服器監聽的通訊埠號 (例如: 8082)
	Port int
	// DBPath 為 SQLite 資料庫檔案存放的相對或絕對路徑
	DBPath string
	// DataDir 為下載原始檔案與快取的目錄路徑
	DataDir string
}

// LoadConfig 負責載入環境變數並進行合法性校驗，回傳初始化的 Config 物件指標
func LoadConfig() (*Config, error) {
	// 嘗試載入當前目錄下的 .env 檔案；若不存在則忽略錯誤繼續使用系統環境變數
	_ = godotenv.Load()

	// 預設通訊埠號為 8063，符合使用者指定通訊埠規範
	defaultPort := 8063
	// 讀取環境變數 PORT 或 APP_PORT
	portStr := os.Getenv("PORT")
	if portStr == "" {
		// 若 PORT 為空，嘗試讀取 APP_PORT
		portStr = os.Getenv("APP_PORT")
	}

	// 初始化埠號為預設值
	port := defaultPort
	// 如果環境變數中有設定埠號字串，進行解析與驗證
	if portStr != "" {
		// 將字串轉換為整數
		parsedPort, err := strconv.Atoi(portStr)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			// 若非有效整數或超出 1~65535 範圍，輸出警告日誌並回退為預設值
			log.Printf("[警告] 設定的 PORT (%s) 不合法，自動回退使用預設埠號: %d", portStr, defaultPort)
			port = defaultPort
		} else {
			// 若合法則套用解析後的埠號
			port = parsedPort
		}
	}

	// 讀取資料庫路徑，預設為 "data/admission.db"
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/admission.db"
	}

	// 讀取資料目錄路徑，預設為 "data"
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	// 確保資料目錄存在，若不存在則建立該目錄 (權限 0755)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("[錯誤] 建立資料目錄失敗: %v", err)
		return nil, err
	}

	// 回傳設定物件實例
	return &Config{
		Port:    port,
		DBPath:  dbPath,
		DataDir: dataDir,
	}, nil
}
