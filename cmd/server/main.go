package main

import (
	// 引入上下文 Context 套件
	"context"
	// 引入格式化輸出套件
	"fmt"
	// 引入 HTML 模板套件
	"html/template"
	// 引入標準日誌套件
	"log"
	// 引入 HTTP 協定套件
	"net/http"
	// 引入作業系統與中斷訊號套件
	"os"
	"os/signal"
	// 引入路徑通配比對套件
	"path/filepath"
	// 引入中斷訊號定義
	"syscall"
	// 引入時間控制套件
	"time"

	// 引入 Echo 網頁伺服器框架與中介層套件
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	// 引入本專案各層模組
	"college-admission-stats/internal/collector"
	"college-admission-stats/internal/config"
	"college-admission-stats/internal/database"
	"college-admission-stats/internal/handlers"
	"college-admission-stats/internal/services"
)

func main() {
	// 0. 自動校準當前工作目錄為執行檔所在路徑 (防止雙擊 .bat 或捷徑時工作目錄偏差)
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		_ = os.Chdir(exeDir)
	}

	// 1. 載入系統環境變數與通訊埠號配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("[致命錯誤] 載入系統組態失敗: %v", err)
	}

	// 2. 初始化純 Go SQLite 資料庫連線池並建立資料表結構
	db, err := database.InitDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("[致命錯誤] 資料庫初始化失敗: %v", err)
	}
	defer db.Close()

	// 3. 初始化數據下載與 ETL 清洗管線
	downloader := collector.NewDownloader(db, cfg.DataDir)

	// 4. 檢查目前資料庫筆數；若為空庫則自動執行 110~115 學年度全量預載入庫
	status, err := downloader.GetStatus()
	if err == nil && status.TotalRecords == 0 {
		log.Printf("[資訊] 檢測到資料庫為空，自動觸發 110~115 學年度升學資料初始化...")
		_ = downloader.SyncAllYears()
	}

	// 5. 初始化業務邏輯與報表匯出服務層
	admissionService := services.NewAdmissionService(db)
	exportService := services.NewExportService()

	// 6. 初始化 HTTP Handler 控制層
	admissionHandler := handlers.NewAdmissionHandler(admissionService)
	syncHandler := handlers.NewSyncHandler(downloader)
	exportHandler := handlers.NewExportHandler(admissionService, exportService)

	// 7. 初始化 HTML 模板引擎並註冊自訂輔助函式
	tmplFuncMap := template.FuncMap{
		// add 用於分頁計算 (頁碼 + 1)
		"add": func(a, b int) int { return a + b },
		// sub 用於分頁計算 (頁碼 - 1)
		"sub": func(a, b int) int { return a - b },
		// mul 用於數值乘法
		"mul": func(a, b float64) float64 { return a * b },
		// div 用於數值除法
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		// toFloat 將整數轉換為浮點數
		"toFloat": func(n int) float64 { return float64(n) },
		// percent 計算兩數佔比之百分比字串
		"percent": func(val, total int) string {
			if total == 0 {
				return "0.0%"
			}
			return fmt.Sprintf("%.1f%%", (float64(val)/float64(total))*100.0)
		},
	}

	// 建立支援全頁面與局部 HTMX 片段之自訂模板渲染器
	renderer, err := handlers.NewTemplateRenderer(filepath.Join("web", "templates"), tmplFuncMap)
	if err != nil {
		log.Fatalf("[致命錯誤] 初始化 HTML 模板引擎失敗: %v", err)
	}

	// 8. 建立 Echo 伺服器實例並配置中介層
	e := echo.New()
	e.HideBanner = true
	e.Renderer = renderer

	// 註冊 Panic 恢復中介層，避免單一 Handler 崩潰導致伺服器中止
	e.Use(middleware.Recover())
	// 註冊 HTTP 請求存取日誌中介層
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "[${time_rfc3339}] ${method} ${uri} -> 狀態碼:${status} 耗時:${latency_human}\n",
	}))
	// 註冊 CORS 跨域資源共享中介層
	e.Use(middleware.CORS())

	// 9. 註冊 Web 視圖與 API 路由清單
	// 首頁與高中升學流向視圖
	e.GET("/", admissionHandler.HandleHighSchool)
	e.GET("/highschool", admissionHandler.HandleHighSchool)

	// 醫牙錄取專區視圖 (獨立分析醫學系與牙醫學系)
	e.GET("/medical", admissionHandler.HandleMedical)

	// 學群與系所深度分析視圖 (動態大餅圖與直方圖)
	e.GET("/disciplines", admissionHandler.HandleDisciplines)

	// 大學來源高中分析視圖
	e.GET("/university", admissionHandler.HandleUniversity)

	// 18 大學群升學趨勢視圖
	e.GET("/group", admissionHandler.HandleGroup)

	// 升學明細清單檢索視圖
	e.GET("/explore", admissionHandler.HandleExplore)

	// 數據同步管理視圖與 API
	e.GET("/sync", syncHandler.HandleSyncPage)
	e.POST("/api/sync", syncHandler.TriggerSync)
	e.GET("/api/sync/status", syncHandler.GetStatus)

	// 大學錄取科系連動查詢 API (供前端多選級聯篩選)
	e.GET("/api/departments-by-univs", admissionHandler.HandleGetDepartmentsByUnivs)

	// CSV 報表匯出下載端點
	e.GET("/export/csv", exportHandler.HandleExportCSV)

	// 10. 啟動 HTTP 伺服器 (支援優雅停機 Graceful Shutdown)
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("==================================================")
	log.Printf(" 大學升學統計與篩選系統 (110~115 學年度)")
	log.Printf(" 伺服器已成功啟動: http://localhost:%d", cfg.Port)
	log.Printf(" 資料庫存放路徑: %s", cfg.DBPath)
	log.Printf("==================================================")

	go func() {
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[致命錯誤] 伺服器啟動失敗: %v", err)
		}
	}()

	// 監聽作業系統中斷訊號 (Ctrl+C, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("[資訊] 接收到關閉訊號，正在優雅停止 Web 伺服器...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Printf("[錯誤] 優雅停機過程中發生錯誤: %v", err)
	} else {
		log.Println("[成功] 伺服器已安全關閉。")
	}
}
