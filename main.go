package main

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

type App struct {
	DB *sql.DB
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	var db = connectToMysql()
	app := &App{DB: db}
	http.HandleFunc("/", app.handleForm)
	http.HandleFunc("/shorten", app.handleShorten)
	http.HandleFunc("/s/", app.handleRedirect)

	fmt.Println("URL Shortener is running on :3030")
	http.ListenAndServe(":3030", nil)

}

func (app *App) handleForm(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		http.Redirect(w, r, "/shorten", http.StatusSeeOther)
		return
	}

	// Serve the HTML form
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `
		<!DOCTYPE html>
		<html>
		<head>
			<title>URL Shortener</title>
		</head>
		<body>
			<h2>URL Shortener</h2>
			<form method="post" action="/shorten">
				<input type="url" name="url" placeholder="Enter a URL" required>
				<input type="submit" value="Shorten">
			</form>
		</body>
		</html>
	`)
}

func (app *App) handleShorten(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	originalURL := r.FormValue("url")
	if originalURL == "" {
		http.Error(w, "URL parameter is missing", http.StatusBadRequest)
		return
	}

	// Generate a unique shortened key for the original URL
	var shortKey = generateShortKey()
	for {

		query := "SELECT count(1) as count FROM links WHERE id = ?"
		var row = app.DB.QueryRow(query, shortKey)
		var count int
		var _ = row.Scan(&count)
		if count == 0 {
			break
		}
		shortKey = generateShortKey()
	}
	website := os.Getenv("WEBSITE_URL")
	// Construct the full shortened URL
	shortenedURL := fmt.Sprintf("%s/s/%s", website, shortKey)
	// Insert in MySQL with a prepared statement
	stmt, err := app.DB.Prepare("INSERT INTO links(id, url) VALUES(?, ?)")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	// Insert values
	_, err = stmt.Exec(shortKey, originalURL)
	if err != nil {
		log.Fatal(err)
	}

	if err != nil {
		panic(err.Error())
	}

	// Serve the result page
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `
		<!DOCTYPE html>
		<html>
		<head>
			<title>URL Shortener</title>
		</head>
		<body>
			<h2>URL Shortener</h2>
			<p>Original URL: `, originalURL, `</p>
			<p>Shortened URL: <a href="`, shortenedURL, `">`, shortenedURL, `</a></p>
		</body>
		</html>
	`)
}

func (app *App) handleRedirect(w http.ResponseWriter, r *http.Request) {
	shortKey := strings.TrimPrefix(r.URL.Path, "/s/")
	if shortKey == "" {
		http.Error(w, "Shortened key is missing", http.StatusBadRequest)
		return
	}
	var (
		id    string
		url   string
		count int
	)
	query := "SELECT id, url, count FROM links WHERE id = ?"
	var row = app.DB.QueryRow(query, shortKey)
	var err = row.Scan(&id, &url, &count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Shortened key not found", http.StatusNotFound)
			return
		} else {
			log.Fatal(err)
		}
	}

	stmt, err := app.DB.Prepare("UPDATE links SET count = count + 1 WHERE id = ?")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	// Execute the statement with the desired condition value
	_, err = stmt.Exec(shortKey)
	if err != nil {
		log.Fatal(err)
	}

	// Redirect the user to the original URL
	http.Redirect(w, r, url, http.StatusFound)
}

func generateShortKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const keyLength = 6

	rand.Seed(time.Now().UnixNano())
	shortKey := make([]byte, keyLength)
	for i := range shortKey {
		shortKey[i] = charset[rand.Intn(len(charset))]
	}
	return string(shortKey)
}

func connectToMysql() *sql.DB {
	// Open the connection
	user := os.Getenv("MYSQL_USER")
	password := os.Getenv("MYSQL_PASSWORD")
	host := os.Getenv("MYSQL_HOST")
	database := os.Getenv("MYSQL_DATABASE")
	var db, err = sql.Open("mysql", user+":"+password+"@tcp("+host+")/"+database)
	if err != nil {
		panic(err.Error())
	}
	return db
}
