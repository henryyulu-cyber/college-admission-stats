# 🚀 全台大學升學統計與篩選系統 - 雲端免費上線部署指南 (Render.com)

本指南將引導您將本系統（Go + Echo + SQLite + HTMX）免費部署至雲端，取得專屬的公開 HTTPS 網址。

---

## 📋 準備工作

確保您已擁有：
1. **[GitHub 帳號](https://github.com/)**（免費註冊）
2. **[Render.com 帳號](https://render.com/)**（直接使用 GitHub 帳號一鍵登入）

---

## 🛠️ 第一步：將程式碼推送到您的 GitHub 倉庫

開啟 PowerShell 終端機，在專案目錄下依序執行以下指令：

```powershell
# 1. 初始化 Git 倉庫 (若尚未初始化)
git init

# 2. 將所有檔案加入暫存區
git add .

# 3. 提交變更
git commit -m "feat: 升學統計系統雲端部署設定與 110-115 學年度升學體制校準"

# 4. 在 GitHub 建立一個名為 college-admission-stats 的公開/私有倉庫後，連結遠端並推送
git branch -M main
git remote add origin https://github.com/您的GitHub帳號/college-admission-stats.git
git push -u origin main
```

> **提示**：由於已配置 `.gitignore`，超過 100MB 的本機 `.db` 不會被推送到 GitHub。雲端伺服器在首次啟動時會自動於 3 秒內快速生成 110~115 學年度 28 萬筆校準升學大數據。

---

## 🌐 第二步：在 Render.com 一鍵自動部署

1. 登入 **[Render.com 儀表板](https://dashboard.render.com/)**。
2. 點擊右上角 **New +** ➔ 選擇 **Web Service**。
3. 選擇 **Build and deploy from a Git repository** ➔ 點擊 **Next**。
4. 找到並授權您的 `college-admission-stats` 倉庫 ➔ 點擊 **Connect**。
5. 設定基本資訊：
   - **Name**：輸入自訂名稱（例如 `taiwan-college-stats`）
   - **Region**：選擇 `Singapore`（新加坡節點，台灣連線最快）
   - **Language**：選擇 `Docker`
   - **Instance Type**：選擇 **Free**（完全免費）
6. 點擊頁面最下方 **Create Web Service**。

---

## 🎉 第三步：部署完成與取得專屬網址

- Render 會自動拉取 GitHub 倉庫並執行 Docker 映像檔建置。
- 建置約需 2~3 分鐘，完成後頁面頂部會出現專屬公開網址：
  👉 **`https://您的自訂名稱.onrender.com/`**
- 此網址具備免費 SSL 憑證 (HTTPS)，可直接分享給所有人使用！
