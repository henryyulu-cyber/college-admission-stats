package handlers

import (
	// 引入格式化套件
	"fmt"
	// 引入 HTTP 協定套件
	"net/http"
	// 引入時間套件
	"time"

	// 引入 Echo Web 框架套件
	"github.com/labstack/echo/v4"

	// 引入本專案 models 與 services 套件
	"college-admission-stats/internal/models"
	"college-admission-stats/internal/services"
)

// ExportHandler 負責處理升學數據之 Excel / CSV 匯出下載請求
type ExportHandler struct {
	// AdmissionService 為升學資料查詢服務
	AdmissionService *services.AdmissionService
	// ExportService 為 CSV / Excel 編碼與檔案生成服務
	ExportService *services.ExportService
}

// NewExportHandler 建立並初始化 ExportHandler 物件
func NewExportHandler(admService *services.AdmissionService, expService *services.ExportService) *ExportHandler {
	return &ExportHandler{
		AdmissionService: admService,
		ExportService:    expService,
	}
}

// HandleExportCSV 處理 CSV 報表下載 (GET /export/csv)
func (h *ExportHandler) HandleExportCSV(c echo.Context) error {
	ctx := c.Request().Context()

	// 綁定當前篩選條件
	var filter models.FilterParams
	if err := c.Bind(&filter); err != nil {
		return c.String(http.StatusBadRequest, "解析篩選參數失敗: "+err.Error())
	}

	// 匯出時設定較大的筆數上限 (例如 50000 筆)
	filter.Page = 1
	filter.PageSize = 50000

	// 執行資料查詢
	result, err := h.AdmissionService.QueryAdmissions(ctx, filter)
	if err != nil {
		return c.String(http.StatusInternalServerError, "查詢匯出資料失敗: "+err.Error())
	}

	// 設定下載檔名與 MIME Header
	fileName := fmt.Sprintf("大學升學統計報表_%s.csv", time.Now().Format("20060102_150405"))
	c.Response().Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Response().WriteHeader(http.StatusOK)

	// 串流寫入 CSV (包含 UTF-8 BOM)
	return h.ExportService.ExportAdmissionsCSV(c.Response().Writer, result.Records)
}
