package services

import (
	// 引入上下文 Context 套件，用於控制逾時與取消
	"context"
	// 引入 SQL 抽象資料庫套件
	"database/sql"
	// 引入格式化套件
	"fmt"
	// 引入數學計算套件
	"math"
	// 引入字串處理套件
	"strings"
	// 引入並發同步套件
	"sync"
	// 引入時間套件
	"time"

	// 引入本專案 models 模型定義
	"college-admission-stats/internal/models"
)

// AdmissionService 提供升學數據多維度統計、聚合分析與搜尋服務
type AdmissionService struct {
	// DB 為 SQLite 資料庫連線實例指標
	DB *sql.DB
	// mu 用於保護記憶體快取之並發讀寫安全
	mu sync.RWMutex
	// cachedOptions 快取下拉選單選項，大幅降低 60 萬筆資料庫重複掃描開銷
	cachedOptions *DropdownOptions
	// cachedOptionsTime 快取建立時間
	cachedOptionsTime time.Time
	// cachedMedUnivs 快取醫學與牙醫大學列表
	cachedMedUnivs []string
	// cachedDeptMap 快取各學群下之學系列表
	cachedDeptMap map[string][]string
}

// NewAdmissionService 建立並初始化 AdmissionService 物件
func NewAdmissionService(db *sql.DB) *AdmissionService {
	return &AdmissionService{
		DB:            db,
		cachedDeptMap: make(map[string][]string),
	}
}

// InvalidateCache 清空所有記憶體快取 (通常於大數據重新同步完成後調用)
func (s *AdmissionService) InvalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedOptions = nil
	s.cachedMedUnivs = nil
	s.cachedDeptMap = make(map[string][]string)
}

// DropdownOptions 封裝前端下拉式選單之可選項
type DropdownOptions struct {
	Years       []int    `json:"years"`
	HighSchools []string `json:"high_schools"`
	Cities      []string `json:"cities"`
	Universities []string `json:"universities"`
	Groups      []string `json:"groups"`
}

// GetDropdownOptions 查詢資料庫中所有現存之學年度、高中、大學、縣市與學群選項 (支援高吞吐記憶體快取)
func (s *AdmissionService) GetDropdownOptions(ctx context.Context) (*DropdownOptions, error) {
	// 1. 優先嘗試由記憶體讀取快取 (讀鎖)
	s.mu.RLock()
	if s.cachedOptions != nil && len(s.cachedOptions.HighSchools) > 0 && time.Since(s.cachedOptionsTime) < 30*time.Minute {
		opts := s.cachedOptions
		s.mu.RUnlock()
		return opts, nil
	}
	s.mu.RUnlock()

	// 2. 若快取不存在、內容為空或已逾期，升級為寫鎖並進行資料庫查詢
	s.mu.Lock()
	defer s.mu.Unlock()

	// 雙重檢查鎖定模式 (Double-Checked Locking)
	if s.cachedOptions != nil && len(s.cachedOptions.HighSchools) > 0 && time.Since(s.cachedOptionsTime) < 30*time.Minute {
		return s.cachedOptions, nil
	}

	opts := &DropdownOptions{}

	// 查詢學年度列表
	rows, err := s.DB.QueryContext(ctx, "SELECT DISTINCT academic_year FROM admissions ORDER BY academic_year DESC")
	if err != nil {
		return nil, fmt.Errorf("查詢學年度失敗: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err == nil {
			opts.Years = append(opts.Years, y)
		}
	}

	// 查詢高中學校列表
	hsRows, err := s.DB.QueryContext(ctx, "SELECT DISTINCT high_school_name FROM admissions ORDER BY high_school_name ASC")
	if err != nil {
		return nil, fmt.Errorf("查詢高中失敗: %w", err)
	}
	defer hsRows.Close()
	for hsRows.Next() {
		var name string
		if err := hsRows.Scan(&name); err == nil {
			opts.HighSchools = append(opts.HighSchools, name)
		}
	}

	// 查詢高中所在縣市列表
	cityRows, err := s.DB.QueryContext(ctx, "SELECT DISTINCT high_school_city FROM admissions WHERE high_school_city != '' ORDER BY high_school_city ASC")
	if err != nil {
		return nil, fmt.Errorf("查詢縣市失敗: %w", err)
	}
	defer cityRows.Close()
	for cityRows.Next() {
		var city string
		if err := cityRows.Scan(&city); err == nil {
			opts.Cities = append(opts.Cities, city)
		}
	}

	// 查詢大學校院列表
	univRows, err := s.DB.QueryContext(ctx, "SELECT DISTINCT university_name FROM admissions ORDER BY university_name ASC")
	if err != nil {
		return nil, fmt.Errorf("查詢大學失敗: %w", err)
	}
	defer univRows.Close()
	for univRows.Next() {
		var u string
		if err := univRows.Scan(&u); err == nil {
			opts.Universities = append(opts.Universities, u)
		}
	}

	// 查詢 18 大學群列表
	grpRows, err := s.DB.QueryContext(ctx, "SELECT DISTINCT discipline_group FROM admissions ORDER BY discipline_group ASC")
	if err != nil {
		return nil, fmt.Errorf("查詢學群失敗: %w", err)
	}
	defer grpRows.Close()
	for grpRows.Next() {
		var g string
		if err := grpRows.Scan(&g); err == nil {
			opts.Groups = append(opts.Groups, g)
		}
	}

	// 寫入快取
	s.cachedOptions = opts
	s.cachedOptionsTime = time.Now()

	return opts, nil
}

// GetHighSchoolSummary 統計指定高中在特定學年度之總升學概況 (國立率、頂大率、醫牙中率、私立、科大人數)
func (s *AdmissionService) GetHighSchoolSummary(ctx context.Context, schoolName string, year int) (*models.HighSchoolSummary, error) {
	query := `
	SELECT 
		high_school_name,
		high_school_city,
		high_school_type,
		COALESCE(SUM(student_count), 0) AS total_students,
		COALESCE(SUM(CASE WHEN university_type = '國立' THEN student_count ELSE 0 END), 0) AS national_count,
		COALESCE(SUM(CASE WHEN university_name IN ('國立臺灣大學', '國立政治大學', '國立清華大學', '國立成功大學', '國立陽明交通大學') THEN student_count ELSE 0 END), 0) AS top_univ_count,
		COALESCE(SUM(CASE WHEN department_name IN ('醫學系', '牙醫學系', '中醫學系', '中醫學系甲組', '中醫學系乙組') THEN student_count ELSE 0 END), 0) AS med_dent_chinese_count,
		COALESCE(SUM(CASE WHEN university_type = '私立' THEN student_count ELSE 0 END), 0) AS private_count,
		COALESCE(SUM(CASE WHEN university_system = '技專校院' THEN student_count ELSE 0 END), 0) AS tech_count
	FROM admissions
	WHERE high_school_name = ?
	`
	args := []interface{}{schoolName}

	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}
	query += " GROUP BY high_school_name, high_school_city, high_school_type"

	summary := &models.HighSchoolSummary{HighSchoolName: schoolName}
	row := s.DB.QueryRowContext(ctx, query, args...)
	err := row.Scan(
		&summary.HighSchoolName,
		&summary.City,
		&summary.SchoolType,
		&summary.TotalStudents,
		&summary.NationalUnivStudents,
		&summary.TopUnivStudents,
		&summary.MedDentChineseStudents,
		&summary.PrivateUnivStudents,
		&summary.TechUnivStudents,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return summary, nil
		}
		return nil, fmt.Errorf("查詢高中摘要失敗: %w", err)
	}

	// 計算各項錄取率百分比字串
	if summary.TotalStudents > 0 {
		tot := float64(summary.TotalStudents)
		summary.NationalRate = fmt.Sprintf("%.1f%%", (float64(summary.NationalUnivStudents)/tot)*100.0)
		summary.TopUnivRate = fmt.Sprintf("%.1f%%", (float64(summary.TopUnivStudents)/tot)*100.0)
		summary.MedDentChineseRate = fmt.Sprintf("%.1f%%", (float64(summary.MedDentChineseStudents)/tot)*100.0)
		summary.PrivateRate = fmt.Sprintf("%.1f%%", (float64(summary.PrivateUnivStudents)/tot)*100.0)
		summary.TechRate = fmt.Sprintf("%.1f%%", (float64(summary.TechUnivStudents)/tot)*100.0)
	} else {
		summary.NationalRate = "0.0%"
		summary.TopUnivRate = "0.0%"
		summary.MedDentChineseRate = "0.0%"
		summary.PrivateRate = "0.0%"
		summary.TechRate = "0.0%"
	}

	return summary, nil
}

// GetMultiSchoolComparison 批次查詢多所高中在特定學年度之 6 大核心指標橫向對比數據
func (s *AdmissionService) GetMultiSchoolComparison(ctx context.Context, schoolNames []string, year int) ([]models.MultiSchoolComparisonRow, error) {
	if len(schoolNames) == 0 {
		return nil, nil
	}

	// 建立 IN 佔位符 (?, ?, ?)
	placeholders := make([]string, len(schoolNames))
	args := make([]interface{}, len(schoolNames))
	for i, name := range schoolNames {
		placeholders[i] = "?"
		args[i] = name
	}

	query := fmt.Sprintf(`
	SELECT 
		high_school_name,
		high_school_city,
		high_school_type,
		COALESCE(SUM(student_count), 0) AS total_students,
		COALESCE(SUM(CASE WHEN university_type = '國立' THEN student_count ELSE 0 END), 0) AS national_count,
		COALESCE(SUM(CASE WHEN university_name IN ('國立臺灣大學', '國立政治大學', '國立清華大學', '國立成功大學', '國立陽明交通大學') THEN student_count ELSE 0 END), 0) AS top_univ_count,
		COALESCE(SUM(CASE WHEN department_name IN ('醫學系', '牙醫學系', '中醫學系', '中醫學系甲組', '中醫學系乙組') THEN student_count ELSE 0 END), 0) AS med_dent_chinese_count,
		COALESCE(SUM(CASE WHEN university_type = '私立' THEN student_count ELSE 0 END), 0) AS private_count,
		COALESCE(SUM(CASE WHEN university_system = '技專校院' THEN student_count ELSE 0 END), 0) AS tech_count
	FROM admissions
	WHERE high_school_name IN (%s)
	`, strings.Join(placeholders, ","))

	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}
	query += " GROUP BY high_school_name, high_school_city, high_school_type ORDER BY total_students DESC"

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("批次查詢多校對比失敗: %w", err)
	}
	defer rows.Close()

	var result []models.MultiSchoolComparisonRow
	for rows.Next() {
		var row models.MultiSchoolComparisonRow
		err := rows.Scan(
			&row.HighSchoolName,
			&row.City,
			&row.SchoolType,
			&row.TotalStudents,
			&row.NationalStudents,
			&row.TopUnivStudents,
			&row.MedDentChineseStudents,
			&row.PrivateStudents,
			&row.TechStudents,
		)
		if err != nil {
			return nil, err
		}

		if row.TotalStudents > 0 {
			tot := float64(row.TotalStudents)
			natVal := math.Round((float64(row.NationalStudents)/tot)*1000.0) / 10.0
			topVal := math.Round((float64(row.TopUnivStudents)/tot)*1000.0) / 10.0
			medVal := math.Round((float64(row.MedDentChineseStudents)/tot)*1000.0) / 10.0
			priVal := math.Round((float64(row.PrivateStudents)/tot)*1000.0) / 10.0
			techVal := math.Round((float64(row.TechStudents)/tot)*1000.0) / 10.0

			row.NationalRateVal = natVal
			row.NationalRate = fmt.Sprintf("%.1f%%", natVal)
			row.TopUnivRateVal = topVal
			row.TopUnivRate = fmt.Sprintf("%.1f%%", topVal)
			row.MedDentChineseRateVal = medVal
			row.MedDentChineseRate = fmt.Sprintf("%.1f%%", medVal)
			row.PrivateRate = fmt.Sprintf("%.1f%%", priVal)
			row.TechRate = fmt.Sprintf("%.1f%%", techVal)
		} else {
			row.NationalRate = "0.0%"
			row.TopUnivRate = "0.0%"
			row.MedDentChineseRate = "0.0%"
			row.PrivateRate = "0.0%"
			row.TechRate = "0.0%"
		}

		result = append(result, row)
	}

	return result, nil
}

// GetDepartmentsByUniversities 根據選取之大學名稱列表，動態查詢其所屬學系列表 (支援快取與級聯篩選)
func (s *AdmissionService) GetDepartmentsByUniversities(ctx context.Context, univNames []string) ([]string, error) {
	if len(univNames) == 0 {
		// 若未指定大學，查詢全台所有不重複學系
		rows, err := s.DB.QueryContext(ctx, "SELECT DISTINCT department_name FROM admissions ORDER BY department_name ASC")
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var depts []string
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err == nil {
				depts = append(depts, d)
			}
		}
		return depts, nil
	}

	placeholders := make([]string, len(univNames))
	args := make([]interface{}, len(univNames))
	for i, u := range univNames {
		placeholders[i] = "?"
		args[i] = u
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT department_name 
		FROM admissions 
		WHERE university_name IN (%s)
		ORDER BY department_name ASC
	`, strings.Join(placeholders, ","))

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢指定大學學系失敗: %w", err)
	}
	defer rows.Close()

	var depts []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err == nil {
			depts = append(depts, d)
		}
	}

	return depts, nil
}

// GetHighSchoolDestinations 統計指定高中錄取之各大學人數排行與佔比
func (s *AdmissionService) GetHighSchoolDestinations(ctx context.Context, schoolName string, year int, limit int) ([]models.UniversityDestinationStat, error) {
	if limit <= 0 {
		limit = 15
	}

	query := `
	SELECT 
		university_name,
		university_type,
		university_system,
		SUM(student_count) AS students
	FROM admissions
	WHERE high_school_name = ?
	`
	args := []interface{}{schoolName}

	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}
	query += " GROUP BY university_name, university_type, university_system ORDER BY students DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢錄取大學分佈失敗: %w", err)
	}
	defer rows.Close()

	var list []models.UniversityDestinationStat
	var total int

	for rows.Next() {
		var item models.UniversityDestinationStat
		if err := rows.Scan(&item.UniversityName, &item.UniversityType, &item.UniversitySystem, &item.StudentCount); err != nil {
			return nil, err
		}
		total += item.StudentCount
		list = append(list, item)
	}

	// 計算個別佔比
	for i := range list {
		if total > 0 {
			list[i].Percentage = math.Round((float64(list[i].StudentCount)/float64(total))*1000) / 10
		}
	}

	return list, nil
}

// GetHighSchoolGroups 統計指定高中在 18 大學群的升讀人數與比例
func (s *AdmissionService) GetHighSchoolGroups(ctx context.Context, schoolName string, year int) ([]models.GroupDistributionStat, error) {
	query := `
	SELECT 
		discipline_group,
		SUM(student_count) AS students
	FROM admissions
	WHERE high_school_name = ?
	`
	args := []interface{}{schoolName}

	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}
	query += " GROUP BY discipline_group ORDER BY students DESC"

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢學群分佈失敗: %w", err)
	}
	defer rows.Close()

	var list []models.GroupDistributionStat
	var total int

	for rows.Next() {
		var item models.GroupDistributionStat
		if err := rows.Scan(&item.DisciplineGroup, &item.StudentCount); err != nil {
			return nil, err
		}
		total += item.StudentCount
		list = append(list, item)
	}

	// 計算個別佔比
	for i := range list {
		if total > 0 {
			list[i].Percentage = math.Round((float64(list[i].StudentCount)/float64(total))*1000) / 10
		}
	}

	return list, nil
}

// GetUniversityOrigins 統計特定大學或學系之來源高中排行
func (s *AdmissionService) GetUniversityOrigins(ctx context.Context, univName, deptName string, year int, limit int) ([]models.UniversityOriginStat, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
	SELECT 
		high_school_name,
		high_school_city,
		high_school_type,
		SUM(student_count) AS students
	FROM admissions
	WHERE university_name = ?
	`
	args := []interface{}{univName}

	if deptName != "" {
		query += " AND department_name LIKE ?"
		args = append(args, "%"+deptName+"%")
	}
	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}

	query += " GROUP BY high_school_name, high_school_city, high_school_type ORDER BY students DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢大學來源高中失敗: %w", err)
	}
	defer rows.Close()

	var list []models.UniversityOriginStat
	var total int

	for rows.Next() {
		var item models.UniversityOriginStat
		if err := rows.Scan(&item.HighSchoolName, &item.City, &item.SchoolType, &item.StudentCount); err != nil {
			return nil, err
		}
		total += item.StudentCount
		list = append(list, item)
	}

	// 計算佔比
	for i := range list {
		if total > 0 {
			list[i].Percentage = math.Round((float64(list[i].StudentCount)/float64(total))*1000) / 10
		}
	}

	return list, nil
}

// GetGroupTrends 查詢 18 大學群在 110 ~ 115 學年度的跨年升學趨勢
func (s *AdmissionService) GetGroupTrends(ctx context.Context, groupName string) ([]models.GroupTrendStat, error) {
	query := `
	SELECT 
		academic_year,
		discipline_group,
		SUM(student_count) AS students
	FROM admissions
	WHERE 1=1
	`
	var args []interface{}
	if groupName != "" && groupName != "全部學群" {
		query += " AND discipline_group = ?"
		args = append(args, groupName)
	}

	query += " GROUP BY academic_year, discipline_group ORDER BY academic_year ASC, students DESC"

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢學群趨勢失敗: %w", err)
	}
	defer rows.Close()

	var list []models.GroupTrendStat
	for rows.Next() {
		var item models.GroupTrendStat
		if err := rows.Scan(&item.AcademicYear, &item.DisciplineGroup, &item.StudentCount); err != nil {
			return nil, err
		}
		list = append(list, item)
	}

	return list, nil
}

// GetTopDepartments 查詢熱門科系升學排行榜
func (s *AdmissionService) GetTopDepartments(ctx context.Context, year int, group string, limit int) ([]models.TopDepartmentStat, error) {
	if limit <= 0 {
		limit = 15
	}

	query := `
	SELECT 
		university_name,
		department_name,
		discipline_group,
		SUM(student_count) AS students
	FROM admissions
	WHERE 1=1
	`
	var args []interface{}
	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}
	if group != "" && group != "全部學群" {
		query += " AND discipline_group = ?"
		args = append(args, group)
	}

	query += " GROUP BY university_name, department_name, discipline_group ORDER BY students DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢熱門科系失敗: %w", err)
	}
	defer rows.Close()

	var list []models.TopDepartmentStat
	for rows.Next() {
		var item models.TopDepartmentStat
		if err := rows.Scan(&item.UniversityName, &item.DepartmentName, &item.DisciplineGroup, &item.StudentCount); err != nil {
			return nil, err
		}
		list = append(list, item)
	}

	return list, nil
}

// QueryResult 封裝分頁篩選結果
type QueryResult struct {
	Records    []models.AdmissionRecord `json:"records"`
	TotalCount int64                    `json:"total_count"`
	TotalPages int                      `json:"total_pages"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
}

// QueryAdmissions 提供多條件動態組合 SQL 查詢升學明細清單
func (s *AdmissionService) QueryAdmissions(ctx context.Context, filter models.FilterParams) (*QueryResult, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	// 構建 WHERE 子句與參數
	var whereClauses []string
	var args []interface{}

	if filter.AcademicYear > 0 {
		whereClauses = append(whereClauses, "academic_year = ?")
		args = append(args, filter.AcademicYear)
	}
	if filter.HighSchool != "" {
		whereClauses = append(whereClauses, "high_school_name LIKE ?")
		args = append(args, "%"+filter.HighSchool+"%")
	}
	if filter.HighSchoolCity != "" {
		whereClauses = append(whereClauses, "high_school_city = ?")
		args = append(args, filter.HighSchoolCity)
	}
	if filter.University != "" {
		whereClauses = append(whereClauses, "university_name LIKE ?")
		args = append(args, "%"+filter.University+"%")
	}
	if filter.UniversitySystem != "" {
		whereClauses = append(whereClauses, "university_system = ?")
		args = append(args, filter.UniversitySystem)
	}
	if filter.UniversityType != "" {
		whereClauses = append(whereClauses, "university_type = ?")
		args = append(args, filter.UniversityType)
	}
	if filter.DisciplineGroup != "" && filter.DisciplineGroup != "全部學群" {
		whereClauses = append(whereClauses, "discipline_group = ?")
		args = append(args, filter.DisciplineGroup)
	}
	if filter.Department != "" {
		whereClauses = append(whereClauses, "department_name LIKE ?")
		args = append(args, "%"+filter.Department+"%")
	}
	if filter.Keyword != "" {
		kw := "%" + filter.Keyword + "%"
		whereClauses = append(whereClauses, "(high_school_name LIKE ? OR university_name LIKE ? OR department_name LIKE ? OR high_school_city LIKE ?)")
		args = append(args, kw, kw, kw, kw)
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// 1. 計算總筆數
	countSQL := "SELECT COUNT(*) FROM admissions" + whereSQL
	var totalCount int64
	err := s.DB.QueryRowContext(ctx, countSQL, args...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("統計查詢筆數失敗: %w", err)
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(filter.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	// 2. 查詢分頁資料
	offset := (filter.Page - 1) * filter.PageSize
	querySQL := fmt.Sprintf(`
		SELECT 
			id, academic_year, high_school_code, high_school_name, high_school_city, high_school_type,
			university_code, university_name, university_system, university_type,
			discipline_group, department_name, student_count, created_at
		FROM admissions
		%s
		ORDER BY academic_year DESC, student_count DESC, high_school_name ASC
		LIMIT ? OFFSET ?
	`, whereSQL)

	queryArgs := append(args, filter.PageSize, offset)
	rows, err := s.DB.QueryContext(ctx, querySQL, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("查詢分頁資料失敗: %w", err)
	}
	defer rows.Close()

	var records []models.AdmissionRecord
	for rows.Next() {
		var r models.AdmissionRecord
		err := rows.Scan(
			&r.ID, &r.AcademicYear, &r.HighSchoolCode, &r.HighSchoolName, &r.HighSchoolCity, &r.HighSchoolType,
			&r.UniversityCode, &r.UniversityName, &r.UniversitySystem, &r.UniversityType,
			&r.DisciplineGroup, &r.DepartmentName, &r.StudentCount, &r.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}

	return &QueryResult{
		Records:    records,
		TotalCount: totalCount,
		TotalPages: totalPages,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
	}, nil
}

// GetMedicalSummary 查詢全台「醫學系」、「牙醫學系」與「中醫學系」之核心 KPI 統計
func (s *AdmissionService) GetMedicalSummary(ctx context.Context, year int, deptFilter string) (*models.MedicalStatsSummary, error) {
	// 建立基礎查詢條件，篩選醫學系、牙醫學系與中醫學系 (包含甲組、乙組)
	deptCondition := "department_name IN ('醫學系', '牙醫學系', '中醫學系', '中醫學系甲組', '中醫學系乙組')"
	if deptFilter == "med" {
		deptCondition = "department_name = '醫學系'"
	} else if deptFilter == "dent" {
		deptCondition = "department_name = '牙醫學系'"
	} else if deptFilter == "chinese_med" {
		deptCondition = "department_name IN ('中醫學系', '中醫學系甲組', '中醫學系乙組')"
	}

	query := fmt.Sprintf(`
		SELECT 
			COALESCE(SUM(student_count), 0) AS total_all,
			COALESCE(SUM(CASE WHEN department_name = '醫學系' THEN student_count ELSE 0 END), 0) AS med_total,
			COALESCE(SUM(CASE WHEN department_name = '牙醫學系' THEN student_count ELSE 0 END), 0) AS dent_total,
			COALESCE(SUM(CASE WHEN department_name IN ('中醫學系', '中醫學系甲組', '中醫學系乙組') THEN student_count ELSE 0 END), 0) AS chinese_med_total,
			COALESCE(SUM(CASE WHEN university_type = '國立' THEN student_count ELSE 0 END), 0) AS national_total,
			COALESCE(SUM(CASE WHEN university_type = '私立' THEN student_count ELSE 0 END), 0) AS private_total
		FROM admissions
		WHERE %s
	`, deptCondition)

	var args []interface{}
	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}

	summary := &models.MedicalStatsSummary{}
	row := s.DB.QueryRowContext(ctx, query, args...)
	err := row.Scan(
		&summary.TotalMedDentStudents,
		&summary.MedStudents,
		&summary.DentStudents,
		&summary.ChineseMedStudents,
		&summary.NationalMedStudents,
		&summary.PrivateMedStudents,
	)
	if err != nil {
		return nil, fmt.Errorf("查詢醫牙中概況失敗: %w", err)
	}

	// 計算國立醫牙中錄取率
	if summary.TotalMedDentStudents > 0 {
		rate := (float64(summary.NationalMedStudents) / float64(summary.TotalMedDentStudents)) * 100.0
		summary.NationalRate = fmt.Sprintf("%.1f%%", rate)
	} else {
		summary.NationalRate = "0.0%"
	}

	return summary, nil
}

// GetMedicalHighSchoolRanking 統計全台高中錄取醫學系、牙醫學系與中醫學系排行榜 Top N
func (s *AdmissionService) GetMedicalHighSchoolRanking(ctx context.Context, year int, deptFilter string, limit int) ([]models.MedicalHighSchoolRank, error) {
	if limit <= 0 {
		limit = 20
	}

	deptCondition := "department_name IN ('醫學系', '牙醫學系', '中醫學系', '中醫學系甲組', '中醫學系乙組')"
	if deptFilter == "med" {
		deptCondition = "department_name = '醫學系'"
	} else if deptFilter == "dent" {
		deptCondition = "department_name = '牙醫學系'"
	} else if deptFilter == "chinese_med" {
		deptCondition = "department_name IN ('中醫學系', '中醫學系甲組', '中醫學系乙組')"
	}

	query := fmt.Sprintf(`
		SELECT 
			high_school_name,
			high_school_city,
			high_school_type,
			COALESCE(SUM(CASE WHEN department_name = '醫學系' THEN student_count ELSE 0 END), 0) AS med_cnt,
			COALESCE(SUM(CASE WHEN department_name = '牙醫學系' THEN student_count ELSE 0 END), 0) AS dent_cnt,
			COALESCE(SUM(CASE WHEN department_name IN ('中醫學系', '中醫學系甲組', '中醫學系乙組') THEN student_count ELSE 0 END), 0) AS chinese_med_cnt,
			COALESCE(SUM(student_count), 0) AS total_cnt
		FROM admissions
		WHERE %s
	`, deptCondition)

	var args []interface{}
	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}
	query += " GROUP BY high_school_name, high_school_city, high_school_type ORDER BY total_cnt DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢高中醫牙中排行失敗: %w", err)
	}
	defer rows.Close()

	var list []models.MedicalHighSchoolRank
	rank := 1
	for rows.Next() {
		var item models.MedicalHighSchoolRank
		item.Rank = rank
		err := rows.Scan(
			&item.HighSchoolName,
			&item.City,
			&item.SchoolType,
			&item.MedCount,
			&item.DentCount,
			&item.ChineseMedCount,
			&item.TotalCount,
		)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
		rank++
	}

	return list, nil
}

// GetMedicalUnivDistribution 統計各大學醫學系、牙醫學系與中醫學系錄取人數佔比 (用於大餅圖)
func (s *AdmissionService) GetMedicalUnivDistribution(ctx context.Context, year int, deptFilter string) ([]models.MedicalUnivStat, error) {
	deptCondition := "department_name IN ('醫學系', '牙醫學系', '中醫學系', '中醫學系甲組', '中醫學系乙組')"
	if deptFilter == "med" {
		deptCondition = "department_name = '醫學系'"
	} else if deptFilter == "dent" {
		deptCondition = "department_name = '牙醫學系'"
	} else if deptFilter == "chinese_med" {
		deptCondition = "department_name IN ('中醫學系', '中醫學系甲組', '中醫學系乙組')"
	}

	query := fmt.Sprintf(`
		SELECT 
			university_name,
			department_name,
			university_type,
			SUM(student_count) AS students
		FROM admissions
		WHERE %s
	`, deptCondition)

	var args []interface{}
	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}
	query += " GROUP BY university_name, department_name, university_type ORDER BY students DESC"

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢大學醫牙分佈失敗: %w", err)
	}
	defer rows.Close()

	var list []models.MedicalUnivStat
	var totalStudents int
	for rows.Next() {
		var item models.MedicalUnivStat
		err := rows.Scan(&item.UniversityName, &item.DepartmentName, &item.UniversityType, &item.StudentCount)
		if err != nil {
			return nil, err
		}
		totalStudents += item.StudentCount
		list = append(list, item)
	}

	// 計算佔比百分比
	if totalStudents > 0 {
		for i := range list {
			list[i].Percentage = math.Round((float64(list[i].StudentCount)/float64(totalStudents))*1000.0) / 10.0
		}
	}

	return list, nil
}

// GetMedicalUnivSources 統計指定大學醫牙學系的主要生源高中分佈 (直方圖與清單)
func (s *AdmissionService) GetMedicalUnivSources(ctx context.Context, univName, deptName string, year int, limit int) ([]models.DisciplineHighSchoolStat, error) {
	if limit <= 0 {
		limit = 15
	}

	query := `
		SELECT 
			high_school_name,
			high_school_city,
			high_school_type,
			SUM(student_count) AS students
		FROM admissions
		WHERE university_name = ? AND (department_name = ? OR ? = '')
	`
	args := []interface{}{univName, deptName, deptName}
	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}
	query += " GROUP BY high_school_name, high_school_city, high_school_type ORDER BY students DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢大學醫牙生源失敗: %w", err)
	}
	defer rows.Close()

	var list []models.DisciplineHighSchoolStat
	var total int
	for rows.Next() {
		var item models.DisciplineHighSchoolStat
		err := rows.Scan(&item.HighSchoolName, &item.City, &item.SchoolType, &item.StudentCount)
		if err != nil {
			return nil, err
		}
		total += item.StudentCount
		list = append(list, item)
	}

	if total > 0 {
		for i := range list {
			list[i].Percentage = math.Round((float64(list[i].StudentCount)/float64(total))*1000.0) / 10.0
		}
	}

	return list, nil
}

// GetMedicalUnivOptions 取得設有醫學系或牙醫學系的大專院校名單 (支援快取)
func (s *AdmissionService) GetMedicalUnivOptions(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	if len(s.cachedMedUnivs) > 0 {
		univs := s.cachedMedUnivs
		s.mu.RUnlock()
		return univs, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.cachedMedUnivs) > 0 {
		return s.cachedMedUnivs, nil
	}

	query := `
		SELECT DISTINCT university_name 
		FROM admissions 
		WHERE department_name IN ('醫學系', '牙醫學系')
		ORDER BY university_name ASC
	`
	rows, err := s.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var univs []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err == nil {
			univs = append(univs, u)
		}
	}
	s.cachedMedUnivs = univs
	return univs, nil
}

// GetDisciplineGroupSummary 統計特定學群的核心 KPI
func (s *AdmissionService) GetDisciplineGroupSummary(ctx context.Context, groupName string, year int) (*models.DisciplineGroupSummary, error) {
	query := `
		SELECT 
			COALESCE(SUM(student_count), 0) AS total_all,
			COUNT(DISTINCT department_name) AS dept_count,
			COALESCE(SUM(CASE WHEN university_type = '國立' THEN student_count ELSE 0 END), 0) AS national_total,
			COALESCE(SUM(CASE WHEN university_type = '私立' THEN student_count ELSE 0 END), 0) AS private_total,
			COALESCE(SUM(CASE WHEN high_school_type = '公立' THEN student_count ELSE 0 END), 0) AS public_hs_total
		FROM admissions
		WHERE discipline_group = ?
	`
	args := []interface{}{groupName}
	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}

	summary := &models.DisciplineGroupSummary{DisciplineGroup: groupName}
	var publicHsTotal int
	row := s.DB.QueryRowContext(ctx, query, args...)
	err := row.Scan(
		&summary.TotalStudents,
		&summary.DepartmentCount,
		&summary.NationalStudents,
		&summary.PrivateStudents,
		&publicHsTotal,
	)
	if err != nil {
		return nil, fmt.Errorf("統計學群摘要失敗: %w", err)
	}

	if summary.TotalStudents > 0 {
		natRate := (float64(summary.NationalStudents) / float64(summary.TotalStudents)) * 100.0
		summary.NationalRate = fmt.Sprintf("%.1f%%", natRate)
		pubRate := (float64(publicHsTotal) / float64(summary.TotalStudents)) * 100.0
		summary.PublicHighSchoolRate = fmt.Sprintf("%.1f%%", pubRate)
	} else {
		summary.NationalRate = "0.0%"
		summary.PublicHighSchoolRate = "0.0%"
	}

	return summary, nil
}

// GetDisciplineDepartmentShares 統計學群內各學系人數佔比 (用於動態大餅圖 Top 10 + 其它)
func (s *AdmissionService) GetDisciplineDepartmentShares(ctx context.Context, groupName string, year int, limit int) ([]models.DepartmentShareStat, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT 
			department_name,
			SUM(student_count) AS students
		FROM admissions
		WHERE discipline_group = ?
	`
	args := []interface{}{groupName}
	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}
	query += " GROUP BY department_name ORDER BY students DESC"

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢學系佔比失敗: %w", err)
	}
	defer rows.Close()

	var allList []models.DepartmentShareStat
	var totalGroupStudents int
	for rows.Next() {
		var item models.DepartmentShareStat
		if err := rows.Scan(&item.DepartmentName, &item.StudentCount); err != nil {
			return nil, err
		}
		totalGroupStudents += item.StudentCount
		allList = append(allList, item)
	}

	if totalGroupStudents == 0 {
		return nil, nil
	}

	var finalList []models.DepartmentShareStat
	otherCount := 0

	for i, item := range allList {
		if i < limit {
			item.Percentage = math.Round((float64(item.StudentCount)/float64(totalGroupStudents))*1000.0) / 10.0
			finalList = append(finalList, item)
		} else {
			otherCount += item.StudentCount
		}
	}

	if otherCount > 0 {
		otherPct := math.Round((float64(otherCount)/float64(totalGroupStudents))*1000.0) / 10.0
		finalList = append(finalList, models.DepartmentShareStat{
			DepartmentName: "其他學系",
			StudentCount:   otherCount,
			Percentage:     otherPct,
		})
	}

	return finalList, nil
}

// GetDisciplineTopHighSchools 統計該學群（或下鑽特定學系）錄取人數最多的 Top 高中排行榜 (直方圖)
func (s *AdmissionService) GetDisciplineTopHighSchools(ctx context.Context, groupName, deptName string, year int, limit int) ([]models.DisciplineHighSchoolStat, error) {
	if limit <= 0 {
		limit = 15
	}

	query := `
		SELECT 
			high_school_name,
			high_school_city,
			high_school_type,
			SUM(student_count) AS students
		FROM admissions
		WHERE (discipline_group = ? OR ? = '') AND (department_name = ? OR ? = '')
	`
	args := []interface{}{groupName, groupName, deptName, deptName}
	if year > 0 {
		query += " AND academic_year = ?"
		args = append(args, year)
	}
	query += " GROUP BY high_school_name, high_school_city, high_school_type ORDER BY students DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查詢學群 Top 高中失敗: %w", err)
	}
	defer rows.Close()

	var list []models.DisciplineHighSchoolStat
	var total int
	for rows.Next() {
		var item models.DisciplineHighSchoolStat
		if err := rows.Scan(&item.HighSchoolName, &item.City, &item.SchoolType, &item.StudentCount); err != nil {
			return nil, err
		}
		total += item.StudentCount
		list = append(list, item)
	}

	if total > 0 {
		for i := range list {
			list[i].Percentage = math.Round((float64(list[i].StudentCount)/float64(total))*1000.0) / 10.0
		}
	}

	return list, nil
}

// GetDisciplineDepartments 取得指定學群底下的所有不重複學系列表 (支援快取)
func (s *AdmissionService) GetDisciplineDepartments(ctx context.Context, groupName string) ([]string, error) {
	s.mu.RLock()
	if depts, ok := s.cachedDeptMap[groupName]; ok {
		s.mu.RUnlock()
		return depts, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if depts, ok := s.cachedDeptMap[groupName]; ok {
		return depts, nil
	}

	query := `
		SELECT DISTINCT department_name 
		FROM admissions 
		WHERE discipline_group = ? 
		ORDER BY department_name ASC
	`
	rows, err := s.DB.QueryContext(ctx, query, groupName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var depts []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err == nil {
			depts = append(depts, d)
		}
	}
	s.cachedDeptMap[groupName] = depts
	return depts, nil
}

// GetFilteredAdmissions 依據多維度篩選條件 (高中清單、大學清單、科系列表、學年度) 查詢升學明細清單與條件符合統計
func (s *AdmissionService) GetFilteredAdmissions(ctx context.Context, schools []string, univs []string, depts []string, year int, limit int) (*models.FilteredAdmissionsResult, error) {
	// 若未指定限制筆數，預設上限為 1000 筆
	if limit <= 0 {
		limit = 1000
	}

	// 初始化結果結構體
	res := &models.FilteredAdmissionsResult{
		ActiveConditions: make([]string, 0),
		Records:          make([]models.AdmissionRecord, 0),
	}

	// 1. 計算啟用之篩選條件維度並記錄條件摘要
	if len(schools) > 0 {
		res.ActiveConditions = append(res.ActiveConditions, fmt.Sprintf("高中: %d 所", len(schools)))
	}
	if len(univs) > 0 {
		res.ActiveConditions = append(res.ActiveConditions, fmt.Sprintf("大學: %d 所", len(univs)))
	}
	if len(depts) > 0 {
		res.ActiveConditions = append(res.ActiveConditions, fmt.Sprintf("科系: %d 系", len(depts)))
	}
	if year > 0 {
		res.ActiveConditions = append(res.ActiveConditions, fmt.Sprintf("學年: %d", year))
	} else {
		res.ActiveConditions = append(res.ActiveConditions, "學年: 全部累計")
	}
	res.ActiveConditionCount = len(res.ActiveConditions)

	// 2. 動態構建 SQL WHERE 條件與參數列表
	var whereClauses []string
	var args []interface{}

	// 高中篩選條件
	if len(schools) > 0 {
		placeholders := make([]string, len(schools))
		for i, sch := range schools {
			placeholders[i] = "?"
			args = append(args, sch)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("high_school_name IN (%s)", strings.Join(placeholders, ",")))
	}

	// 錄取大學篩選條件
	if len(univs) > 0 {
		placeholders := make([]string, len(univs))
		for i, u := range univs {
			placeholders[i] = "?"
			args = append(args, u)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("university_name IN (%s)", strings.Join(placeholders, ",")))
	}

	// 錄取科系篩選條件
	if len(depts) > 0 {
		placeholders := make([]string, len(depts))
		for i, d := range depts {
			placeholders[i] = "?"
			args = append(args, d)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("department_name IN (%s)", strings.Join(placeholders, ",")))
	}

	// 學年度篩選條件
	if year > 0 {
		whereClauses = append(whereClauses, "academic_year = ?")
		args = append(args, year)
	}

	// 組合 WHERE 子句
	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// 3. 聚合統計符合該篩選組合的摘要指標
	summarySQL := fmt.Sprintf(`
		SELECT 
			COUNT(*) AS matched_records,
			COALESCE(SUM(student_count), 0) AS total_students,
			COALESCE(SUM(CASE WHEN university_type = '國立' THEN student_count ELSE 0 END), 0) AS national_count,
			COALESCE(SUM(CASE WHEN university_type = '私立' THEN student_count ELSE 0 END), 0) AS private_count,
			COALESCE(SUM(CASE WHEN university_system = '技專校院' THEN student_count ELSE 0 END), 0) AS tech_count,
			COALESCE(SUM(CASE WHEN university_name IN ('國立臺灣大學', '國立政治大學', '國立清華大學', '國立成功大學', '國立陽明交通大學') THEN student_count ELSE 0 END), 0) AS top_count,
			COALESCE(SUM(CASE WHEN department_name IN ('醫學系', '牙醫學系', '中醫學系', '中醫學系甲組', '中醫學系乙組') THEN student_count ELSE 0 END), 0) AS med_count
		FROM admissions
		%s
	`, whereSQL)

	var matchedRecords int
	var totalStudents int
	var nationalCount int
	var privateCount int
	var techCount int
	var topCount int
	var medCount int

	err := s.DB.QueryRowContext(ctx, summarySQL, args...).Scan(
		&matchedRecords,
		&totalStudents,
		&nationalCount,
		&privateCount,
		&techCount,
		&topCount,
		&medCount,
	)
	if err != nil {
		return nil, fmt.Errorf("統計多維度篩選摘要失敗: %w", err)
	}

	res.MatchedRecordsCount = matchedRecords
	res.MatchedTotalStudents = totalStudents
	res.NationalStudents = nationalCount
	res.PrivateStudents = privateCount
	res.TechStudents = techCount
	res.TopUnivStudents = topCount
	res.MedDentChineseStudents = medCount

	// 計算各項錄取比率
	if totalStudents > 0 {
		tot := float64(totalStudents)
		res.NationalRate = fmt.Sprintf("%.1f%%", (float64(nationalCount)/tot)*100.0)
		res.PrivateRate = fmt.Sprintf("%.1f%%", (float64(privateCount)/tot)*100.0)
		res.TechRate = fmt.Sprintf("%.1f%%", (float64(techCount)/tot)*100.0)
		res.TopUnivRate = fmt.Sprintf("%.1f%%", (float64(topCount)/tot)*100.0)
		res.MedDentChineseRate = fmt.Sprintf("%.1f%%", (float64(medCount)/tot)*100.0)
	} else {
		res.NationalRate = "0.0%"
		res.PrivateRate = "0.0%"
		res.TechRate = "0.0%"
		res.TopUnivRate = "0.0%"
		res.MedDentChineseRate = "0.0%"
	}

	// 4. 查詢詳細升學明細清單
	detailSQL := fmt.Sprintf(`
		SELECT 
			id, academic_year, high_school_code, high_school_name, high_school_city, high_school_type,
			university_code, university_name, university_system, university_type,
			discipline_group, department_name, student_count, created_at
		FROM admissions
		%s
		ORDER BY academic_year DESC, student_count DESC, high_school_name ASC, university_name ASC
		LIMIT ?
	`, whereSQL)

	detailArgs := append(args, limit)
	rows, err := s.DB.QueryContext(ctx, detailSQL, detailArgs...)
	if err != nil {
		return nil, fmt.Errorf("查詢多維度升學明細清單失敗: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r models.AdmissionRecord
		err := rows.Scan(
			&r.ID, &r.AcademicYear, &r.HighSchoolCode, &r.HighSchoolName, &r.HighSchoolCity, &r.HighSchoolType,
			&r.UniversityCode, &r.UniversityName, &r.UniversitySystem, &r.UniversityType,
			&r.DisciplineGroup, &r.DepartmentName, &r.StudentCount, &r.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		res.Records = append(res.Records, r)
	}

	return res, nil
}


