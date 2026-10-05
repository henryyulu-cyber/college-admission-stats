# ==============================================================================
# 階段一：編譯階段 (Builder Stage) - 使用輕量 Alpine 版 Golang 映像檔
# ==============================================================================
FROM golang:1.22-alpine AS builder

# 設定容器內部工作目錄
WORKDIR /app

# 複製 Go 依賴描述檔案
COPY go.mod go.sum ./

# 下載並快取專案所需的外部依賴套件
RUN go mod download

# 複製專案全部原始碼檔案
COPY . .

# 執行 Go 編譯：
# - CGO_ENABLED=0：停用 CGO，生成純靜態二進位執行檔
# - GOOS=linux：指定目標作業系統為 Linux
# - -ldflags="-w -s"：去除除錯符號與資訊以最大幅度壓縮執行檔大小
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server/main.go

# 在建置期的高速多核心環境下，直接預先烘焙生成 110~115 學年度全量升學大數據庫
RUN go run ./cmd/tools/rebuild/main.go

# ==============================================================================
# 階段二：正式運行階段 (Production Stage) - 使用超輕量 Alpine Linux
# ==============================================================================
FROM alpine:latest

# 設定工作目錄
WORKDIR /app

# 安裝基礎 CA 安全憑證與 Asia/Taipei 時區資訊
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Taipei

# 從編譯階段複製編譯完成的 server 二進位執行檔
COPY --from=builder /app/server .

# 從編譯階段複製已完整預先烘焙好 110~115 學年度的 SQLite 升學資料庫與快取
COPY --from=builder /app/data/ ./data/

# 複製 HTML/HTMX 網頁模板與靜態資源
COPY web/ ./web/

# 設定預設服務埠號 (Render 或雲端平台會自動以環境變數 PORT 覆蓋此值)
ENV PORT=8063
ENV GIN_MODE=release

# 宣告對外開放之服務埠號
EXPOSE 8063

# 容器啟動指令：執行 Go 伺服器
CMD ["./server"]
