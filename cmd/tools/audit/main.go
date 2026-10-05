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
	// 引入字串處理套件
	"strings"

	// 引入純 Go SQLite 驅動套件 (支援跨平台編譯)
	_ "modernc.org/sqlite"
)

// main 為盤點診斷工具的主要進入點
func main() {
	// 定義 SQLite 資料庫檔案路徑
	dbPath := filepath.Join("data", "admission.db")

	// 開啟 SQLite 資料庫連線
	db, err := sql.Open("sqlite", dbPath)
	// 若開啟資料庫失敗則終止程式並印出錯誤
	if err != nil {
		log.Fatalf("[致命錯誤] 無法開啟資料庫檔案 %s: %v", dbPath, err)
	}
	// 於函式結束前安全關閉資料庫連線
	defer db.Close()

	// 印出報告標題區塊
	fmt.Println("================================================================================")
	fmt.Println("  全台大學升學統計資料庫 (110~115學年度) 純 Go 升學體制資料盤點報告")
	fmt.Println("================================================================================\n")

	// -----------------------------------------------------------------------------
	// 盤點項目 1: 技術型高中 (高職/農工/高工/高商/家商) 錄取醫牙中人數 (常態應為 0)
	// -----------------------------------------------------------------------------
	fmt.Println("【盤點項目一：高職/技術型高中 錄取普大醫牙中人數清查】")
	// 構建 SQL 查詢：統計所有校名含職業關鍵字且科系為醫牙中之學校
	queryVocMed := `
		SELECT high_school_name, COUNT(*), SUM(student_count)
		FROM admissions
		WHERE (
			high_school_name LIKE '%工%' OR high_school_name LIKE '%商%' OR 
			high_school_name LIKE '%農%' OR high_school_name LIKE '%家%' OR 
			high_school_name LIKE '%職業%' OR high_school_name LIKE '%職%'
		)
		AND department_name IN ('醫學系', '牙醫學系', '中醫學系', '學士後醫學系')
		GROUP BY high_school_name
		ORDER BY SUM(student_count) DESC`

	// 執行 SQL 查詢
	rowsVocMed, err := db.Query(queryVocMed)
	if err != nil {
		log.Printf("[錯誤] 查詢高職醫牙中資料失敗: %v", err)
	} else {
		// 累加涉事學校與人數
		vocMedCount := 0
		vocMedRecords := 0
		vocMedStudents := 0

		for rowsVocMed.Next() {
			var hsName string
			var recCount, stCount int
			if err := rowsVocMed.Scan(&hsName, &recCount, &stCount); err == nil {
				vocMedCount++
				vocMedRecords += recCount
				vocMedStudents += stCount
				fmt.Printf("  * 異常學校: %s -> 紀錄: %d 筆, 人數: %d 人\n", hsName, recCount, stCount)
			}
		}
		rowsVocMed.Close()

		if vocMedCount == 0 {
			fmt.Println("  ✅ 查核結果：全台所有技術型高中錄取普大醫牙中人數為 0 人 (完全符合升學體制常規)")
		} else {
			fmt.Printf("  ⚠️ 查核結果：共 %d 所高職異常錄取醫牙中，合計 %d 筆紀錄 / %d 人\n", vocMedCount, vocMedRecords, vocMedStudents)
		}
	}
	fmt.Println("\n" + strings.Repeat("-", 80) + "\n")

	// -----------------------------------------------------------------------------
	// 盤點項目 2: 明星普通高中 科技大學 (技專校院) 錄取人數佔比清查 (常態應 < 5%)
	// -----------------------------------------------------------------------------
	fmt.Println("【盤點項目二：代表性普通型高中 升入科技大學人數佔比檢驗】")
	targetHighSchools := []string{
		"臺北市立建國高級中學",
		"臺北市立第一女子高級中學",
		"國立臺灣師範大學附屬高級中學",
		"臺中市立臺中第一高級中等學校",
		"國立臺南第一高級中學",
		"高雄市立高雄高級中學",
	}

	for _, hs := range targetHighSchools {
		// 查詢該高中之科技大學與一般大學人數分佈
		querySys := `
			SELECT university_system, SUM(student_count)
			FROM admissions
			WHERE high_school_name = ?
			GROUP BY university_system`

		rowsSys, err := db.Query(querySys, hs)
		if err != nil {
			continue
		}

		totalStudents := 0
		techStudents := 0

		for rowsSys.Next() {
			var sys string
			var count int
			if err := rowsSys.Scan(&sys, &count); err == nil {
				totalStudents += count
				if sys == "技專校院" {
					techStudents += count
				}
			}
		}
		rowsSys.Close()

		pct := 0.0
		if totalStudents > 0 {
			pct = (float64(techStudents) / float64(totalStudents)) * 100.0
		}
		fmt.Printf("  * %-16s -> 總人數: %4d 人 | 科技大學: %3d 人 (佔比: %.1f%%)\n", hs, totalStudents, techStudents, pct)
	}
	fmt.Println("\n" + strings.Repeat("-", 80) + "\n")

	// -----------------------------------------------------------------------------
	// 盤點項目 3: 國立成功大學醫學系 與 臺南一中 錄取人數精準對齊核實
	// -----------------------------------------------------------------------------
	fmt.Println("【盤點項目三：成大醫學系 - 臺南一中 錄取人數各學年明細】")
	queryN1NCKU := `
		SELECT academic_year, department_name, student_count
		FROM admissions
		WHERE high_school_name = '國立臺南第一高級中學' 
		  AND university_name = '國立成功大學'
		  AND department_name IN ('醫學系', '牙醫學系')
		ORDER BY academic_year ASC, department_name ASC`

	rowsN1, err := db.Query(queryN1NCKU)
	if err == nil {
		for rowsN1.Next() {
			var yr int
			var dept string
			var cnt int
			if err := rowsN1.Scan(&yr, &dept, &cnt); err == nil {
				fmt.Printf("  * %d 學年度 國立臺南第一高級中學 -> 成大 %s: %d 人 (交叉查榜錨定)\n", yr, dept, cnt)
			}
		}
		rowsN1.Close()
	}
	fmt.Println("\n" + strings.Repeat("-", 80) + "\n")

	// -----------------------------------------------------------------------------
	// 盤點項目 4: 國立中山大學 vs 中山醫學大學 屬性與醫學系核實
	// -----------------------------------------------------------------------------
	fmt.Println("【盤點項目四：國立中山大學 vs 中山醫學大學 屬性與醫學系核查】")
	queryChungShan := `
		SELECT university_name, university_type, department_name, SUM(student_count)
		FROM admissions
		WHERE university_name LIKE '%中山%' AND department_name IN ('醫學系', '牙醫學系')
		GROUP BY university_name, university_type, department_name`

	rowsCS, err := db.Query(queryChungShan)
	if err == nil {
		foundRows := 0
		for rowsCS.Next() {
			var uName, uType, dept string
			var cnt int
			if err := rowsCS.Scan(&uName, &uType, &dept, &cnt); err == nil {
				foundRows++
				fmt.Printf("  * 學校: %-14s | 屬性: %-4s | 系所: %-8s | 錄取總人數: %d 人\n", uName, uType, dept, cnt)
			}
		}
		rowsCS.Close()
		if foundRows == 0 {
			fmt.Println("  (無中山相關醫學系資料)")
		}
	}
	fmt.Println("\n================================================================================")
}
