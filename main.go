package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// struct
type TodoItem struct {
	Id          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	// image
	IntExample    int        `json:"int_example"`
	DoubleExample float64    `json:"double_example"`
	CreatedAt     *time.Time `json:"created_at"`
	UpdatedAt     *time.Time `json:"updated_at"`
}

// function

func main() {
	// load .env file
	enverr := godotenv.Load()
	if enverr != nil {
		log.Fatal("error loading environment variables")
	}

	now := time.Now().UTC()

	item := TodoItem{
		Id:          1,
		Title:       "title1",
		Description: "description1",
		Status:      "Doing",
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}

	dsn := os.Getenv("DB_CONN_STR")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Fatal(err) // log and exit program
	}

	fmt.Println(db)

	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": item,
		})
	})

	r.Run(":3000") // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
