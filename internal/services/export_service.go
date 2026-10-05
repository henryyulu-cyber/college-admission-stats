package services

import (
	// 引入 CSV 編碼套件
	"encoding/csv"
	// 引入格式化套件
	"fmt"
	// 引入 I/O 串流套件
	"io"
	// 引入本專案 models 模型定義
	"college-admission-stats/internal/models"
)

// ExportService 提供報表資料匯出與 UTF-8 BOM 編碼支援
type ExportService struct{}

// NewExportService 建立 ExportService 實例指標
func NewExportService() *ExportService {
	return &ExportService{}
}

// ExportAdmissionsCSV 將升學明細清單寫入為 CSV 串流，並自動加入 UTF-8 BOM 防亂碼
func (s *ExportService) ExportAdmissionsCSV(w io.Writer, records []models.AdmissionRecord) error {
	// 1. 寫入 UTF-8 BOM (\xEF\xBB\xBF)，確保 Excel 在 Windows 下開啟不會出現繁體中文字元亂碼
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return fmt.Errorf("寫入 UTF-8 BOM 失敗: %w", err)
	}

	// 2. 建立 CSV Writer
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// 3. 寫入 CSV 標題列
	headers := []string{
		"序號",
		"學年度",
		"高中學校代碼",
		"高中學校名稱",
		"高中所在縣市",
		"高中公私立",
		"大學校院代碼",
		"大學校院名稱",
		"大學體系",
		"大學公私立",
		"教育部18大學群",
		"錄取學系名稱",
		"錄取/升學人數",
	}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("寫入標題列失敗: %w", err)
	}

	// 4. 逐行寫入資料列
	for idx, r := range records {
		row := []string{
			fmt.Sprintf("%d", idx+1),
			fmt.Sprintf("%d學年度", r.AcademicYear),
			r.HighSchoolCode,
			r.HighSchoolName,
			r.HighSchoolCity,
			r.HighSchoolType,
			r.UniversityCode,
			r.UniversityName,
			r.UniversitySystem,
			r.UniversityType,
			r.DisciplineGroup,
			r.DepartmentName,
			fmt.Sprintf("%d", r.StudentCount),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("寫入資料列失敗: %w", err)
		}
	}

	return nil
}
