package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

type LogData struct {
	ID           string `json:"id"`
	LogTimestamp string `json:"log_timestamp"`
	Category     string `json:"category"`
	ItemName     string `json:"item_name"`
	Value        string `json:"value"`
	CreatedAt    string `json:"created_at"`
}

var db *sql.DB

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

func getData(w http.ResponseWriter, r *http.Request) {
	query := `SELECT id, log_timestamp, category, item_name, value, created_at FROM logs ORDER BY log_timestamp;`

	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var dataList []LogData
	for rows.Next() {
		var d LogData
		// カラムの数と順番に合わせてスキャンする
		if err := rows.Scan(&d.ID, &d.LogTimestamp, &d.Category, &d.ItemName, &d.Value, &d.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		dataList = append(dataList, d)
		rows.Err()
	}

	// JSONに変換してレスポンスとして返す
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dataList)
}

func main() {
	connectDB()

	r := mux.NewRouter()
	r.HandleFunc("/get", getData).Methods("GET")

	log.Println("Server starting on :2000...")
	// 第2引数に r を渡す！
	log.Fatal(http.ListenAndServe(":2000", r))
}
