package handlers

import (
	// 引入格式化與錯誤套件
	"fmt"
	// 引入 HTML 模板標準庫
	"html/template"
	// 引入 I/O 串流庫
	"io"
	// 引入標準日誌套件
	"log"
	// 引入路徑處理套件
	"path/filepath"

	// 引入 Echo Web 框架套件
	"github.com/labstack/echo/v4"
)

// TemplateRenderer 實作 Echo 的 echo.Renderer 介面，支援獨立頁面與局部 HTMX 片段
type TemplateRenderer struct {
	// templates 保存每個模板名稱對應的專屬 template.Template 實例
	templates map[string]*template.Template
}

// NewTemplateRenderer 初始化所有全頁模板與 HTMX 局部片段
func NewTemplateRenderer(tmplDir string, funcMap template.FuncMap) (*TemplateRenderer, error) {
	r := &TemplateRenderer{
		templates: make(map[string]*template.Template),
	}

	layoutPath := filepath.Join(tmplDir, "layout.html")

	// 1. 定義 HTMX 獨立局部更新片段清單
	partials := []string{
		"highschool_partial.html",
		"university_partial.html",
		"group_partial.html",
		"explore_table_partial.html",
		"sync_status_partial.html",
		"medical_partial.html",
		"disciplines_partial.html",
	}

	for _, partial := range partials {
		partialPath := filepath.Join(tmplDir, partial)
		t, err := template.New(partial).Funcs(funcMap).ParseFiles(partialPath)
		if err != nil {
			return nil, fmt.Errorf("解析局部模板 %s 失敗: %w", partial, err)
		}
		r.templates[partial] = t
	}

	// 2. 定義全頁面模板清單 (需組裝 layout.html 以及所有 partials)
	fullPages := []string{
		"highschool.html",
		"university.html",
		"group.html",
		"explore.html",
		"sync.html",
		"medical.html",
		"disciplines.html",
	}

	for _, page := range fullPages {
		pagePath := filepath.Join(tmplDir, page)
		// 組合 layout.html、當前 page 與所有 partials
		filesToParse := []string{layoutPath, pagePath}
		for _, p := range partials {
			filesToParse = append(filesToParse, filepath.Join(tmplDir, p))
		}
		t, err := template.New("layout.html").Funcs(funcMap).ParseFiles(filesToParse...)
		if err != nil {
			return nil, fmt.Errorf("解析全頁模板 %s 失敗: %w", page, err)
		}
		r.templates[page] = t
	}

	return r, nil
}

// Render 實作 echo.Renderer 的 Render 方法，將指定模板名稱與資料渲染至回應串流
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	tmpl, ok := t.templates[name]
	if !ok {
		log.Printf("[模板錯誤] 找不到已註冊之 HTML 模板: %s", name)
		return fmt.Errorf("找不到已註冊之 HTML 模板: %s", name)
	}

	var err error
	// 若為全頁面模板，執行 layout.html 入口渲染
	if _, isFull := map[string]bool{
		"highschool.html":  true,
		"university.html":  true,
		"group.html":       true,
		"explore.html":     true,
		"sync.html":        true,
		"medical.html":     true,
		"disciplines.html": true,
	}[name]; isFull {
		err = tmpl.ExecuteTemplate(w, "layout.html", data)
	} else {
		// 若為局部片段，直接執行該片段渲染
		err = tmpl.ExecuteTemplate(w, name, data)
	}

	if err != nil {
		log.Printf("[模板渲染錯誤] 模板: %s, 錯誤訊息: %v", name, err)
		return err
	}

	return nil
}
