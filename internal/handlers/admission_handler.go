package handlers

import (
	// 引入 HTTP 協定狀態碼庫
	"net/http"
	// 引入字串轉換庫
	"strconv"
	// 引入字串操作庫
	"strings"

	// 引入 Echo Web 框架套件
	"github.com/labstack/echo/v4"

	// 引入本專案 models 與 services 套件
	"college-admission-stats/internal/models"
	"college-admission-stats/internal/services"
)

// AdmissionHandler 負責處理高中升學流向、大學錄取來源與學群統計之 HTTP 請求
type AdmissionHandler struct {
	// Service 為升學業務邏輯服務實例指標
	Service *services.AdmissionService
}

// NewAdmissionHandler 建立並初始化 AdmissionHandler 物件
func NewAdmissionHandler(service *services.AdmissionService) *AdmissionHandler {
	return &AdmissionHandler{Service: service}
}

// HighSchoolViewData 封裝高中升學流向頁面所需的所有視圖資料
type HighSchoolViewData struct {
	// ActiveTab 當前頁籤
	ActiveTab string
	// IsComparisonMode 是否處於多校橫向比較模式 (選取 >= 2 所高中且未限定大學/科系)
	IsComparisonMode bool
	// HasSpecificFilters 是否啟用了特定大學或科系篩選
	HasSpecificFilters bool
	// SelectedSchools 選取之高中清單 (支援多選)
	SelectedSchools []string
	// SelectedSchoolsStr 選取之高中以逗號分隔字串
	SelectedSchoolsStr string
	// SelectedUnivs 選取之錄取大學清單 (支援多選)
	SelectedUnivs []string
	// SelectedUnivsStr 選取之大學以逗號分隔字串
	SelectedUnivsStr string
	// SelectedDepts 選取之錄取科系清單 (支援多選)
	SelectedDepts []string
	// SelectedDeptsStr 選取之科系以逗號分隔字串
	SelectedDeptsStr string
	// SelectedYear 當前選取的學年度 (0 代表全部學年)
	SelectedYear int
	// Options 下拉選單所有基礎選項
	Options *services.DropdownOptions
	// AvailableDepartments 根據當前選取大學所過濾出的可用科系列表
	AvailableDepartments []string
	// Summary 單校模式時之 6 大指標概況
	Summary *models.HighSchoolSummary
	// Destinations 單校模式時之錄取大學分佈
	Destinations []models.UniversityDestinationStat
	// Groups 單校模式時之 18 大學群分佈
	Groups []models.GroupDistributionStat
	// ComparisonRows 多校模式時之各校 6 大指標對比數據行
	ComparisonRows []models.MultiSchoolComparisonRow
	// FilteredResult 多維度條件篩選之符合條件統計與詳細明細清單
	FilteredResult *models.FilteredAdmissionsResult
}

// helperSplitAndClean 將字串切分成非空字串切片並去除前後空格與重複
func helperSplitAndClean(rawParams []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, raw := range rawParams {
		parts := strings.Split(raw, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" && !seen[trimmed] {
				seen[trimmed] = true
				result = append(result, trimmed)
			}
		}
	}
	return result
}

// HandleHighSchool 處理高中升學流向視圖 (GET / 或 GET /highschool)
func (h *AdmissionHandler) HandleHighSchool(c echo.Context) error {
	ctx := c.Request().Context()

	// 1. 讀取全域下拉選單基礎選項 (由內存快取提供)
	options, err := h.Service.GetDropdownOptions(ctx)
	if err != nil {
		return c.String(http.StatusInternalServerError, "載入下拉選單失敗: "+err.Error())
	}

	// 2. 解析選取之高中學校 (支援多選陣列與逗號分隔字串)
	rawSchools := c.QueryParams()["schools"]
	if len(rawSchools) == 0 {
		rawSchools = c.QueryParams()["school"]
	}
	selectedSchools := helperSplitAndClean(rawSchools)
	// 若使用者未指定任何高中，預設使用建國中學
	if len(selectedSchools) == 0 {
		if len(options.HighSchools) > 0 {
			selectedSchools = []string{"臺北市立建國高級中學"}
		}
	}

	// 3. 解析選取之錄取大學 (支援多選)
	rawUnivs := c.QueryParams()["univs"]
	if len(rawUnivs) == 0 {
		rawUnivs = c.QueryParams()["univ"]
	}
	selectedUnivs := helperSplitAndClean(rawUnivs)

	// 4. 解析選取之錄取科系 (支援多選)
	rawDepts := c.QueryParams()["depts"]
	if len(rawDepts) == 0 {
		rawDepts = c.QueryParams()["dept"]
	}
	selectedDepts := helperSplitAndClean(rawDepts)

	// 5. 解析學年度
	yearStr := c.QueryParam("year")
	year := 0
	if yearStr != "" {
		year, _ = strconv.Atoi(yearStr)
	} else if len(options.Years) > 0 {
		year = options.Years[0] // 預設使用最新學年度
	}

	// 6. 依據已選取的錄取大學，動態取得關聯科系列表
	availableDepts, err := h.Service.GetDepartmentsByUniversities(ctx, selectedUnivs)
	if err != nil {
		return c.String(http.StatusInternalServerError, "載入關聯科系列表失敗: "+err.Error())
	}

	// 7. 判斷是否觸發「多校橫向比較模式」或「特定條件篩選模式」
	hasSpecificFilters := len(selectedUnivs) > 0 || len(selectedDepts) > 0
	isComparisonMode := len(selectedSchools) >= 2 && !hasSpecificFilters

	var summary *models.HighSchoolSummary
	var destinations []models.UniversityDestinationStat
	var groups []models.GroupDistributionStat
	var comparisonRows []models.MultiSchoolComparisonRow

	if isComparisonMode {
		// 多校模式：批次查詢各校 6 大指標
		comparisonRows, err = h.Service.GetMultiSchoolComparison(ctx, selectedSchools, year)
		if err != nil {
			return c.String(http.StatusInternalServerError, "統計多校比較數據失敗: "+err.Error())
		}
	} else if len(selectedSchools) > 0 && !hasSpecificFilters {
		// 單校純覽模式 (未指定大學/科系)：展示 6 大 KPI、大學排行與學群分佈
		primarySchool := selectedSchools[0]

		// 取得該高中升學概況 6 大 KPI
		summary, err = h.Service.GetHighSchoolSummary(ctx, primarySchool, year)
		if err != nil {
			return c.String(http.StatusInternalServerError, "統計高中概況失敗: "+err.Error())
		}

		// 取得錄取大學分佈
		destinations, err = h.Service.GetHighSchoolDestinations(ctx, primarySchool, year, 15)
		if err != nil {
			return c.String(http.StatusInternalServerError, "統計錄取大學失敗: "+err.Error())
		}

		// 取得 18 大學群分佈
		groups, err = h.Service.GetHighSchoolGroups(ctx, primarySchool, year)
		if err != nil {
			return c.String(http.StatusInternalServerError, "統計學群分佈失敗: "+err.Error())
		}
	}

	// 8. 查詢多維度條件篩選之符合條件統計與詳細明細清單 (上限 1000 筆)
	filteredResult, err := h.Service.GetFilteredAdmissions(ctx, selectedSchools, selectedUnivs, selectedDepts, year, 1000)
	if err != nil {
		return c.String(http.StatusInternalServerError, "查詢多維度篩選明細清單失敗: "+err.Error())
	}

	// 增加指標物件空值防禦保護，避免模板渲染時發生空指標存取
	if summary == nil {
		summary = &models.HighSchoolSummary{}
	}
	if filteredResult == nil {
		filteredResult = &models.FilteredAdmissionsResult{
			Records: []models.AdmissionRecord{},
		}
	}

	data := HighSchoolViewData{
		ActiveTab:            "highschool",
		IsComparisonMode:     isComparisonMode,
		HasSpecificFilters:   hasSpecificFilters,
		SelectedSchools:      selectedSchools,
		SelectedSchoolsStr:   strings.Join(selectedSchools, ","),
		SelectedUnivs:        selectedUnivs,
		SelectedUnivsStr:     strings.Join(selectedUnivs, ","),
		SelectedDepts:        selectedDepts,
		SelectedDeptsStr:     strings.Join(selectedDepts, ","),
		SelectedYear:         year,
		Options:              options,
		AvailableDepartments: availableDepts,
		Summary:              summary,
		Destinations:         destinations,
		Groups:               groups,
		ComparisonRows:       comparisonRows,
		FilteredResult:       filteredResult,
	}

	// 檢查是否為 HTMX 局部請求
	if c.Request().Header.Get("HX-Request") == "true" {
		return c.Render(http.StatusOK, "highschool_partial.html", data)
	}


	// 回傳完整頁面
	return c.Render(http.StatusOK, "highschool.html", data)
}

// HandleGetDepartmentsByUnivs 提供 API 端點，以 JSON 格式動態回傳指定大學所屬的全部科系列表
// GET /api/departments-by-univs?univs=國立臺灣大學,國立清華大學
func (h *AdmissionHandler) HandleGetDepartmentsByUnivs(c echo.Context) error {
	ctx := c.Request().Context()

	rawUnivs := c.QueryParams()["univs"]
	if len(rawUnivs) == 0 {
		rawUnivs = c.QueryParams()["univ"]
	}
	selectedUnivs := helperSplitAndClean(rawUnivs)

	depts, err := h.Service.GetDepartmentsByUniversities(ctx, selectedUnivs)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, depts)
}

// UniversityViewData 封裝大學來源高中分析頁面所需之視圖資料
type UniversityViewData struct {
	ActiveTab      string
	SelectedUniv   string
	SelectedDept   string
	SelectedYear   int
	Options        *services.DropdownOptions
	Origins        []models.UniversityOriginStat
}

// HandleUniversity 處理大學來源高中視圖 (GET /university)
func (h *AdmissionHandler) HandleUniversity(c echo.Context) error {
	ctx := c.Request().Context()

	options, err := h.Service.GetDropdownOptions(ctx)
	if err != nil {
		return c.String(http.StatusInternalServerError, "載入下拉選單失敗: "+err.Error())
	}

	univ := c.QueryParam("university")
	if univ == "" && len(options.Universities) > 0 {
		univ = "國立臺灣大學"
	}

	dept := c.QueryParam("department")
	yearStr := c.QueryParam("year")
	year := 0
	if yearStr != "" {
		year, _ = strconv.Atoi(yearStr)
	} else if len(options.Years) > 0 {
		year = options.Years[0]
	}

	// 取得來源高中排行
	origins, err := h.Service.GetUniversityOrigins(ctx, univ, dept, year, 25)
	if err != nil {
		return c.String(http.StatusInternalServerError, "統計大學來源高中失敗: "+err.Error())
	}

	data := UniversityViewData{
		ActiveTab:    "university",
		SelectedUniv: univ,
		SelectedDept: dept,
		SelectedYear: year,
		Options:      options,
		Origins:      origins,
	}

	if c.Request().Header.Get("HX-Request") == "true" {
		return c.Render(http.StatusOK, "university_partial.html", data)
	}

	return c.Render(http.StatusOK, "university.html", data)
}

// GroupViewData 封裝 18 大學群趨勢分析頁面所需之視圖資料
type GroupViewData struct {
	ActiveTab     string
	SelectedGroup string
	SelectedYear  int
	Options       *services.DropdownOptions
	Trends        []models.GroupTrendStat
	TopDepts      []models.TopDepartmentStat
}

// HandleGroup 處理學群與學系趨勢視圖 (GET /group)
func (h *AdmissionHandler) HandleGroup(c echo.Context) error {
	ctx := c.Request().Context()

	options, err := h.Service.GetDropdownOptions(ctx)
	if err != nil {
		return c.String(http.StatusInternalServerError, "載入下拉選單失敗: "+err.Error())
	}

	group := c.QueryParam("group")
	if group == "" {
		group = "資訊學群"
	}

	yearStr := c.QueryParam("year")
	year := 0
	if yearStr != "" {
		year, _ = strconv.Atoi(yearStr)
	} else if len(options.Years) > 0 {
		year = options.Years[0]
	}

	// 取得學群跨年趨勢
	trends, err := h.Service.GetGroupTrends(ctx, group)
	if err != nil {
		return c.String(http.StatusInternalServerError, "查詢學群趨勢失敗: "+err.Error())
	}

	// 取得該學群熱門科系排行
	topDepts, err := h.Service.GetTopDepartments(ctx, year, group, 15)
	if err != nil {
		return c.String(http.StatusInternalServerError, "查詢熱門科系失敗: "+err.Error())
	}

	data := GroupViewData{
		ActiveTab:     "group",
		SelectedGroup: group,
		SelectedYear:  year,
		Options:       options,
		Trends:        trends,
		TopDepts:      topDepts,
	}

	if c.Request().Header.Get("HX-Request") == "true" {
		return c.Render(http.StatusOK, "group_partial.html", data)
	}

	return c.Render(http.StatusOK, "group.html", data)
}

// ExploreViewData 封裝升學明細清單檢索頁面資料
type ExploreViewData struct {
	ActiveTab string
	Filter    models.FilterParams
	Options   *services.DropdownOptions
	Result    *services.QueryResult
}

// HandleExplore 處理升學明細清單多條件檢索 (GET /explore)
func (h *AdmissionHandler) HandleExplore(c echo.Context) error {
	ctx := c.Request().Context()

	options, err := h.Service.GetDropdownOptions(ctx)
	if err != nil {
		return c.String(http.StatusInternalServerError, "載入下拉選單失敗: "+err.Error())
	}

	// 綁定查詢參數
	var filter models.FilterParams
	if err := c.Bind(&filter); err != nil {
		return c.String(http.StatusBadRequest, "解析查詢參數失敗: "+err.Error())
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 25
	}

	// 執行資料庫複合查詢
	result, err := h.Service.QueryAdmissions(ctx, filter)
	if err != nil {
		return c.String(http.StatusInternalServerError, "檢索明細資料失敗: "+err.Error())
	}

	data := ExploreViewData{
		ActiveTab: "explore",
		Filter:    filter,
		Options:   options,
		Result:    result,
	}

	if c.Request().Header.Get("HX-Request") == "true" {
		return c.Render(http.StatusOK, "explore_table_partial.html", data)
	}

	return c.Render(http.StatusOK, "explore.html", data)
}

// MedicalViewData 封裝醫牙專區頁面所需的所有視圖資料
type MedicalViewData struct {
	ActiveTab      string
	SelectedMode   string // "hs_rank" (全台高中醫牙排行) 或 "univ_source" (大學醫牙生源分佈)
	SelectedYear   int
	SelectedDept   string // "all", "med", "dent"
	SelectedUniv   string
	Options        *services.DropdownOptions
	MedicalUnivs   []string
	Summary        *models.MedicalStatsSummary
	HighSchoolRank []models.MedicalHighSchoolRank
	UnivDist       []models.MedicalUnivStat
	UnivSources    []models.DisciplineHighSchoolStat
}

// HandleMedical 處理「🏥 醫牙錄取專區」之 HTTP 請求 (GET /medical)
func (h *AdmissionHandler) HandleMedical(c echo.Context) error {
	ctx := c.Request().Context()

	// 1. 取得下拉選單選項
	options, err := h.Service.GetDropdownOptions(ctx)
	if err != nil {
		return c.String(http.StatusInternalServerError, "載入下拉選單失敗: "+err.Error())
	}

	// 2. 取得醫牙相關大學選項
	medUnivs, err := h.Service.GetMedicalUnivOptions(ctx)
	if err != nil {
		return c.String(http.StatusInternalServerError, "載入醫學院校失敗: "+err.Error())
	}

	// 3. 解析查詢參數
	mode := c.QueryParam("mode")
	if mode == "" {
		mode = "hs_rank" // 預設為全台高中醫牙排行模式
	}

	dept := c.QueryParam("dept")
	if dept == "" {
		dept = "all" // 預設為全部醫牙
	}

	yearStr := c.QueryParam("year")
	year := 0
	if yearStr != "" {
		year, _ = strconv.Atoi(yearStr)
	}

	selectedUniv := c.QueryParam("univ")
	if selectedUniv == "" && len(medUnivs) > 0 {
		selectedUniv = "國立臺灣大學"
	}

	// 4. 查詢醫牙核心 KPI
	summary, err := h.Service.GetMedicalSummary(ctx, year, dept)
	if err != nil {
		return c.String(http.StatusInternalServerError, "統計醫牙指標失敗: "+err.Error())
	}

	// 5. 根據模式載入對應維度數據
	var hsRank []models.MedicalHighSchoolRank
	var univDist []models.MedicalUnivStat
	var univSources []models.DisciplineHighSchoolStat

	if mode == "hs_rank" {
		// 查詢全台高中醫牙排行 Top 20
		hsRank, err = h.Service.GetMedicalHighSchoolRanking(ctx, year, dept, 20)
		if err != nil {
			return c.String(http.StatusInternalServerError, "統計高中排行失敗: "+err.Error())
		}
		// 查詢各大學醫牙錄取分佈 (供大餅圖)
		univDist, err = h.Service.GetMedicalUnivDistribution(ctx, year, dept)
		if err != nil {
			return c.String(http.StatusInternalServerError, "統計大學分佈失敗: "+err.Error())
		}
	} else {
		// 查詢指定大學醫牙中生源高中 Top 15
		targetDeptName := ""
		if dept == "med" {
			targetDeptName = "醫學系"
		} else if dept == "dent" {
			targetDeptName = "牙醫學系"
		} else if dept == "chinese_med" {
			targetDeptName = "中醫學系"
		}
		univSources, err = h.Service.GetMedicalUnivSources(ctx, selectedUniv, targetDeptName, year, 15)
		if err != nil {
			return c.String(http.StatusInternalServerError, "統計醫牙中生源失敗: "+err.Error())
		}
	}

	data := MedicalViewData{
		ActiveTab:      "medical",
		SelectedMode:   mode,
		SelectedYear:   year,
		SelectedDept:   dept,
		SelectedUniv:   selectedUniv,
		Options:        options,
		MedicalUnivs:   medUnivs,
		Summary:        summary,
		HighSchoolRank: hsRank,
		UnivDist:       univDist,
		UnivSources:    univSources,
	}

	if c.Request().Header.Get("HX-Request") == "true" {
		return c.Render(http.StatusOK, "medical_partial.html", data)
	}

	return c.Render(http.StatusOK, "medical.html", data)
}

// DisciplinesViewData 封裝學群系所深度分析頁面資料
type DisciplinesViewData struct {
	ActiveTab      string
	SelectedGroup  string
	SelectedDept   string
	SelectedYear   int
	Options        *services.DropdownOptions
	Departments    []string
	Summary        *models.DisciplineGroupSummary
	DeptShares     []models.DepartmentShareStat
	TopHighSchools []models.DisciplineHighSchoolStat
}

// HandleDisciplines 處理「📊 學群系所深度分析」之 HTTP 請求 (GET /disciplines)
func (h *AdmissionHandler) HandleDisciplines(c echo.Context) error {
	ctx := c.Request().Context()

	// 1. 取得下拉選單選項
	options, err := h.Service.GetDropdownOptions(ctx)
	if err != nil {
		return c.String(http.StatusInternalServerError, "載入下拉選單失敗: "+err.Error())
	}

	// 2. 解析學群參數
	group := c.QueryParam("group")
	if group == "" && len(options.Groups) > 0 {
		group = "資訊學群" // 預設展示資訊學群
	}

	dept := c.QueryParam("dept")

	yearStr := c.QueryParam("year")
	year := 0
	if yearStr != "" {
		year, _ = strconv.Atoi(yearStr)
	}

	// 3. 取得該學群下之學系列表
	depts, err := h.Service.GetDisciplineDepartments(ctx, group)
	if err != nil {
		return c.String(http.StatusInternalServerError, "載入學系列表失敗: "+err.Error())
	}

	// 4. 取得學群總體 KPI
	summary, err := h.Service.GetDisciplineGroupSummary(ctx, group, year)
	if err != nil {
		return c.String(http.StatusInternalServerError, "統計學群指標失敗: "+err.Error())
	}

	// 5. 取得該學群內各系所佔比 (大餅圖)
	deptShares, err := h.Service.GetDisciplineDepartmentShares(ctx, group, year, 10)
	if err != nil {
		return c.String(http.StatusInternalServerError, "統計系所佔比失敗: "+err.Error())
	}

	// 6. 取得該學群/系所生源 Top 15 高中 (直方圖)
	topHS, err := h.Service.GetDisciplineTopHighSchools(ctx, group, dept, year, 15)
	if err != nil {
		return c.String(http.StatusInternalServerError, "統計生源高中失敗: "+err.Error())
	}

	data := DisciplinesViewData{
		ActiveTab:      "disciplines",
		SelectedGroup:  group,
		SelectedDept:   dept,
		SelectedYear:   year,
		Options:        options,
		Departments:    depts,
		Summary:        summary,
		DeptShares:     deptShares,
		TopHighSchools: topHS,
	}

	if c.Request().Header.Get("HX-Request") == "true" {
		return c.Render(http.StatusOK, "disciplines_partial.html", data)
	}

	return c.Render(http.StatusOK, "disciplines.html", data)
}

