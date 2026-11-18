package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// データベース接続とテーブル自動作成の初期化
func connectDB() {
	// 例: user:password@tcp(127.0.0.1:3306)/dbname?parseTime=true
	dsn := os.Getenv("DB_DSN")

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to open db: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to connect db: %v", err)
	}
}

func initDB() {
	// テーブルが存在しない場合は自動で作成（オートマイグレーション）
	query := `
	CREATE TABLE IF NOT EXISTS logs (
		id INT AUTO_INCREMENT PRIMARY KEY,
		log_timestamp DATETIME NOT NULL,
		category VARCHAR(50) NOT NULL,
		item_name VARCHAR(50) NOT NULL,
		value INT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.Exec(query)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	log.Println("Database connected and table verified successfully.")
}

// ログ受領・展開・登録のエンドポイント
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. リクエストボディ（ZIPファイル）をメモリ上に全読み込み
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	// 2. メモリ上でZIPとして読み込み
	zipReader, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
	if err != nil {
		http.Error(w, "Invalid zip file", http.StatusBadRequest)
		return
	}

	// 3. ZIP内のファイルをループ処理
	for _, file := range zipReader.File {
		f, err := file.Open()
		if err != nil {
			continue
		}

		// CSVをパースしてデータベースにインサート
		err = parseAndInsertCSV(f)
		f.Close()
		if err != nil {
			log.Printf("Error processing file %s: %v", file.Name, err)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Successfully uploaded, extracted, and registered logs!"))
}

// CSVファイルをパースしてMySQLへINSERTする関数
func parseAndInsertCSV(r io.Reader) error {
	reader := csv.NewReader(r)
	// 必要に応じてフィールド数のバリデーション等を設定可能

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// 想定フォーマット: [日時(2026/11/05 11:00:00)], [カテゴリ], [アイテム名], [数値]
		if len(record) < 4 {
			continue
		}

		rawTime := record[0]
		category := record[1]
		itemName := record[2]
		valStr := record[3]

		// 日付のパース (レイアウトは実際のCSVに合わせて調整してください)
		parsedTime, err := time.Parse("2006/01/02 15:04:05", rawTime)
		if err != nil {
			// フォーマット違いの場合はYYYY-MM-DD HH:MM:SSなども試す
			parsedTime, err = time.Parse("2006-01-02 15:04:05", rawTime)
			if err != nil {
				parsedTime = time.Now() // フォールバック
			}
		}

		// 数値への変換
		var val int
		fmt.Sscanf(valStr, "%d", &val)

		// DBへインサート
		stmt := "INSERT INTO logs (log_timestamp, category, item_name, value) VALUES (?, ?, ?, ?)"
		_, err = db.Exec(stmt, parsedTime, category, itemName, val)
		if err != nil {
			log.Printf("Insert failed: %v", err)
		}
	}
	return nil
}

func main() {
	// DB初期化
	connectDB()
	initDB()
	defer db.Close()

	// ルーティング設定
	http.HandleFunc("/upload", uploadHandler)

	// 3000番ポートでリッスン
	log.Println("Go server started on :3000")
	if err := http.ListenAndServe(":3000", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
