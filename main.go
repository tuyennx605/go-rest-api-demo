package main

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ItemStatus int

const (
	ItemStatusDOing ItemStatus = iota // 0
	ItemStatusDone
	ItemStatusDeleted
)

var allItemStatus = [3]string{"Doing", "Done", "Deleted"}

func (item *ItemStatus) StatusString() string {
	return allItemStatus[*item]
}

func parseStr2ItemStatus(s string) (ItemStatus, error) {
	for i := range allItemStatus {
		if allItemStatus[i] == s {
			return ItemStatus(i), nil
		}
	}
	return ItemStatus(0), errors.New("invalid status string")
}

// lay tu db len map vao struct
// override function scan data  tu db
func (item *ItemStatus) Scan(value interface{}) error { // impliment khi 2 du lieu duoi db va struct khac nhau (duoi db len)
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprintf("fail to scan data from sql: %s", value))
	}

	v, err := parseStr2ItemStatus(string(bytes))
	if err != nil {
		return errors.New(fmt.Sprintf("fail to scan data from sql: %s", value))
	}

	*item = v

	return nil
}

// override function map du lieu tu struct sang 1 dang khac
// thay doi gia tri len json
func (item *ItemStatus) MarshalJSON() ([]byte, error) {
	if item == nil {
		return nil, nil
	}
	return []byte(fmt.Sprintf("\"%s\"", item.StatusString())), nil
}

/////////// chieu day du lieu len sever

// override function Value de chuyen du lieu tu struct xuong db
// int -> string de luu vao db
func (item *ItemStatus) Value() (driver.Value, error) {
	if item == nil {
		return nil, nil
	}
	return item.StatusString(), nil
}

// override function UnmarshalJSON map du lieu tu json -> struct sang 1 dang khac
// thay doi gia tri tu json len struct (Doing -> 0)
func (item *ItemStatus) UnmarshalJSON(data []byte) error {
	str := strings.ReplaceAll(string(data), "\"", "") // xoa dau "

	itemValue, err := parseStr2ItemStatus(str)
	if err != nil {
		return err
	}

	*item = itemValue // doi data
	return nil
}

// struct
type TodoItem struct {
	Id          int         `json:"id" gorm:"column:id;"`
	Title       string      `json:"title" gorm:"column:title;"`
	Description string      `json:"description" gorm:"column:description;"`
	Status      *ItemStatus `json:"status" gorm:"column:status;"`
	// image
	IntExample    int        `json:"int_example" gorm:"column:int_example;"`
	DoubleExample float64    `json:"double_example" gorm:"column:double_example;"`
	CreatedAt     *time.Time `json:"created_at" gorm:"column:created_at;"`
	UpdatedAt     *time.Time `json:"updated_at" gorm:"column:updated_at;"`
}

func (TodoItem) TableName() string { return "todo_items" }

type TodoItemCreate struct {
	Id          int    `json:"-" gorm:"column:id;"`
	Title       string `json:"title" gorm:"column:title;"`
	Description string `json:"description" gorm:"column:description;"`
}

func (TodoItemCreate) TableName() string { return TodoItem{}.TableName() }

type TodoItemUpdate struct {
	// Id          int    `json:"-" gorm:"column:id;"`
	Title       *string     `json:"title" gorm:"column:title;"`
	Description *string     `json:"description" gorm:"column:description;"`
	Status      *ItemStatus `json:"status" gorm:"column:status;"`
}

func (TodoItemUpdate) TableName() string { return TodoItem{}.TableName() }

type Paging struct {
	Page  int   `json:"page" form:"page"`
	Limit int   `json:"limit" form:"limit"`
	Total int64 `json:"total" form:"-"`
}

func (p *Paging) Process() {
	fmt.Println(p.Limit, p.Page)
	if p.Page <= 0 {
		p.Page = 1
	}

	if p.Limit <= 0 || p.Limit >= 100 {
		p.Limit = 10
	}
}

// function

func main() {
	// load .env file
	enverr := godotenv.Load()
	if enverr != nil {
		log.Fatal("error loading environment variables")
	}

	// connect db
	dsn := os.Getenv("DB_CONN_STR")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err) // log and exit program
	}

	if err := runService(db); err != nil {
		log.Fatal(err) // log and exit program
	}
}

func runService(db *gorm.DB) error {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	// CRUD

	v1 := r.Group("/v1")

	{
		items := v1.Group("/items")

		{
			// POST /v1/items (create anew item)
			items.POST("", CreateItem(db))
			// GET /v1/items (list items) /v1/items?page=1...
			items.GET("", ListItem(db))
			// GET /v1/items/:id (get item detail by id)
			items.GET("/:id", GetItem(db))
			// (PUT || PATCH) /v1/items/:id (update by id)
			items.PATCH("/:id", UpdateItem(db))
			// DELETE /v1/items/:id (delete by id)
			items.DELETE("/:id", DeleteItem(db))
		}
	}

	return r.Run(":3000") // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func CreateItem(db *gorm.DB) func(*gin.Context) {
	return func(c *gin.Context) {
		var data TodoItemCreate

		if err := c.ShouldBind(&data); err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		if err := db.Create(&data).Error; err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": data.Id})
	}
}

func ListItem(db *gorm.DB) func(*gin.Context) {
	return func(c *gin.Context) {
		var paging Paging

		if err := c.ShouldBind(&paging); err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		paging.Process()

		var result []TodoItem

		// where not is deleted
		db = db.Where("status <> ?", "Deleted")

		// count
		if err := db.Table(TodoItem{}.TableName()).Count(&paging.Total).Error; err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		if err := db.Order("id desc").Limit(paging.Limit).Offset((paging.Page - 1) * paging.Limit).Find(&result).Error; err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": result, "paging": paging})
	}
}

func GetItem(db *gorm.DB) func(*gin.Context) {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))

		if err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		var data TodoItem

		if err := db.Where("id = ?", id).First(&data).Error; err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": data})
	}
}

func UpdateItem(db *gorm.DB) func(*gin.Context) {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))

		if err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		var data TodoItemUpdate

		if err := c.ShouldBind(&data); err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		if err := db.Where("id = ?", id).Updates(&data).Error; err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": 1})
	}
}

func DeleteItem(db *gorm.DB) func(*gin.Context) {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))

		if err != nil {
			c.JSON(401, map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		// if err := db.Table(TodoItem{}.TableName()).Where("id = ?", id).Delete(nil).Error; err != nil {
		// 	c.JSON(401, map[string]interface{}{
		// 		"error": err.Error(),
		// 	})
		// 	return
		// }

		if err := db.Table(TodoItem{}.TableName()).Where("id = ?", id).Updates(map[string]interface{}{
			"status": "Deleted",
		}).Error; err != nil {
			c.JSON(401, map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": 1})
	}
}

// func main() {
// 	// load .env file
// 	enverr := godotenv.Load()
// 	if enverr != nil {
// 		log.Fatal("error loading environment variables")
// 	}

// 	now := time.Now().UTC()

// 	item := TodoItem{
// 		Id:          1,
// 		Title:       "title1",
// 		Description: "description1",
// 		Status:      "Doing",
// 		CreatedAt:   &now,
// 		UpdatedAt:   &now,
// 	}

// 	dsn := os.Getenv("DB_CONN_STR")
// 	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})

// 	if err != nil {
// 		log.Fatal(err) // log and exit program
// 	}

// 	fmt.Println(db)

// 	r := gin.Default()

// 	r.GET("/ping", func(c *gin.Context) {
// 		c.JSON(http.StatusOK, gin.H{
// 			"message": item,
// 		})
// 	})

// 	r.Run(":3000") // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
// }
