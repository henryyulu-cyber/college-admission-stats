package handlers

import (
	// 引入 HTTP 狀態碼庫
	"net/http"

	// 引入 Echo Web 框架套件
	"github.com/labstack/echo/v4"

	// 引入本專案 collector 套件
	"college-admission-stats/internal/collector"
	"college-admission-stats/internal/models"
)

// SyncHandler 負責處理升學資料線上同步與狀態查詢
type SyncHandler struct {
	// Downloader 為數據採集管線管理器實例指標
	Downloader *collector.Downloader
}

// NewSyncHandler 建立並初始化 SyncHandler 物件
func NewSyncHandler(downloader *collector.Downloader) *SyncHandler {
	return &SyncHandler{Downloader: downloader}
}

// SyncViewData 封裝同步中心頁面所需之視圖資料
type SyncViewData struct {
	ActiveTab string
	Status    *models.SyncStatus
}

// HandleSyncPage 渲染數據同步與下載管理頁面 (GET /sync)
func (h *SyncHandler) HandleSyncPage(c echo.Context) error {
	status, err := h.Downloader.GetStatus()
	if err != nil {
		return c.String(http.StatusInternalServerError, "取得同步狀態失敗: "+err.Error())
	}

	data := SyncViewData{
		ActiveTab: "sync",
		Status:    status,
	}

	return c.Render(http.StatusOK, "sync.html", data)
}

// TriggerSync 觸發背景非同步執行 110~115 學年度數據採集與清洗 (POST /api/sync)
func (h *SyncHandler) TriggerSync(c echo.Context) error {
	err := h.Downloader.SyncAllYears()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "已成功啟動 110~115 學年度升學資料同步作業！",
	})
}

// GetStatus 回傳當前數據庫筆數與同步進度狀態 (GET /api/sync/status)
func (h *SyncHandler) GetStatus(c echo.Context) error {
	status, err := h.Downloader.GetStatus()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	// 若為 HTMX 請求，回傳狀態卡片 HTML 片段供動態輪詢
	if c.Request().Header.Get("HX-Request") == "true" {
		return c.Render(http.StatusOK, "sync_status_partial.html", status)
	}

	return c.JSON(http.StatusOK, status)
}
