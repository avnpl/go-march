package main

import (
	"fmt"
	"github.com/avnpl/go-march/models"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"log"
	"net/http"
)

var PORT = ":8000"
var DB *sqlx.DB

func ConnectDB() error {
	dbURL := ""

	var err error
	DB, err = sqlx.Open("pgx", dbURL)

	if err != nil {
		return fmt.Errorf("/Failed to open database: %w", err)
	}
	if err := DB.Ping(); err != nil {
		return fmt.Errorf("/Failed to ping database: %w", err)
	}

	fmt.Println("Successfully connected to database!")
	return nil
}

func createProduct(db *sqlx.DB, product *models.Post) error {
	sql := `
		INSERT INTO post (post_id, title, body, author_id)
		VALUES (:postID, :title, :body, :authorID)
		RETURNING id;
	`
	stmt, err := db.PrepareNamed(sql) // Prepare named statement
	if err != nil {
		return fmt.Errorf("prepare named statement error: %w", err)
	}
	defer stmt.Close()

	var newID string
	args := map[string]interface{}{
		"post_id":   product.PostID,
		"title":     product.Title,
		"body":      product.Body,
		"author_id": product.AuthorID,
	}
	err = stmt.Get(&newID, args) // Execute named statement and get result
	if err != nil {
		return fmt.Errorf("named statement execute error: %w", err)
	}
	product.PostID = newID // Update product ID with the newly generated ID
	fmt.Printf("Product created with ID: %d\n", newID)
	return nil
}

func main() {
	if err := ConnectDB(); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer DB.Close()
	fmt.Println("Starting server...")
	mux := http.NewServeMux()

	mux.HandleFunc("GET /comment/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		fmt.Fprintf(w, "Return comment with ID : %s", id)
	})

	if err := http.ListenAndServe(PORT, mux); err != nil {
		fmt.Println(err.Error())
	}
}
