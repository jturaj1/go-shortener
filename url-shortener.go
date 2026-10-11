package main

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"strings"
	"regexp"
)

const (
	base62Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz1234567890"
	DOMAIN string = "http://localhost:8080/"
	SecretKey = "your-secret-key" // Change to env var in a real setting
)

var db *sql.DB

type CreateURLRequest struct {
	URL string `json:"url"`
}

type User struct {
	ID	int	`json:"id"`
	Email string `json:"email"`
	Password string	`json:"password"`
	CreatedAt string `json:"created_at,omitempty"`
}

type LoginRequest struct {
	Email string `json:"email"`
	Password string `json:"password"`
}

type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func intToBase62(n int64) string {
	if n == 0 {
		return string(base62Chars[0])
	}

	var result strings.Builder
	for n > 0 {
		remainder := n % 62
		result.WriteByte(base62Chars[remainder])
		n = n / 62
	}

	return result.String()
}

func base62ToInt(s string) int64 {
	var result int64
	for _, char := range s {
		result = result * 62
		index := strings.IndexRune(base62Chars, char)
		if index == -1 {
			return 0
		}
		result += int64(index)
	}
	return result
}

func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func init_db() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./urls.db")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
				CREATE TABLE IF NOT EXISTS urls (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					long_url TEXT NOT NULL,
					click_count INTEGER DEFAULT 0,
					last_accessed TIMESTAMP,
					expires_at TIMESTAMP,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					user_id	INTEGER,
					FOREIGN KEY (user_id) REFERENCES users(id)
				)
			`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS users (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				email TEXT UNIQUE NOT NULL,
				password TEXT NOT NULL,
				created_at	TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization Header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(SecretKey), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error":"Invalid token"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(*Claims); ok {
			c.Set("userID", claims.UserID)
		}

		c.Next()
	}

}

func user_register(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if !isValidEmail(user.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error":"Invalid email found"})
		return
	}

	if len(user.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error":"Password too short"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error":"Failed to hash password", "details":err.Error()})
		return
	}

	result, err := db.Exec("INSERT INTO users (email, password) VALUES (?, ?)", user.Email, string(hashedPassword))
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error":"User already exists", "details": err.Error()})
		return
	}

	userID, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user id", "details":err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully", "user_id": userID})
}

func isValidEmail(email string) bool {
	const emailRegex = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	match, _ := regexp.MatchString(emailRegex, email)
	return match
}

func user_login(c *gin.Context) {
	var request LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var user User
	err := db.QueryRow("SELECT id, email, password FROM users WHERE email = ?", request.Email).Scan(&user.ID, &user.Email, &user.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error":"Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error":"Invalid credentials"})
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID: int64(user.ID),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(SecretKey))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func submitUrl(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var request CreateURLRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	urlStr := request.URL
	if urlStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL paramater required"})
		return
	}

	if !strings.Contains(urlStr, "://") {
		urlStr = "https://" + urlStr
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL format"})
		return
	}

	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "https"
	}

	if parsedURL.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL requires a host"})
		return
	}

	validURL := parsedURL.String()

	var existingID int64
	err = db.QueryRow("SELECT id FROM urls WHERE long_url == ?", validURL).Scan(&existingID)
	if err == nil {
		shortCode := intToBase62(existingID)
		shortUrl := DOMAIN + shortCode
		c.JSON(http.StatusOK, gin.H{"received_url": shortUrl})
		return
	} else if err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Something went wrong"})
		return
	}

	result, err := db.Exec("INSERT INTO urls (long_url, expires_at, user_id) VALUES (?, datetime('now', '+30 days'), ?)", validURL, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store URL"})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve Id"})
		return
	}

	shortCode := intToBase62(id)
	shortUrl := DOMAIN + shortCode

	c.JSON(http.StatusOK, gin.H{"received_url": shortUrl})
}

func redirectUrl(c *gin.Context) {
	shortCode := c.Param("id")

	id := base62ToInt(shortCode)

	var longUrl string
	var expiresAt string
	err := db.QueryRow("SELECT long_url, expires_at FROM urls WHERE id = ?", id).Scan(&longUrl, &expiresAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Url not found"})
		return
	}

	expiresTime, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse timestamp"})
		return
	}

	if time.Now().After(expiresTime) {
		c.JSON(http.StatusGone, gin.H{"error": "URL has expired"})
		return
	}

	_, err = db.Exec("UPDATE urls SET click_count = click_count + 1, last_accessed = datetime('now') WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update click count and last accessed"})
		return
	}

	c.Redirect(http.StatusMovedPermanently, longUrl)
}

func getUrls(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	rows, err := db.Query("SELECT id, long_url, click_count, last_accessed, expires_at FROM urls WHERE user_id = ?", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch URLs"})
		return
	}
	defer rows.Close()

	var urls []gin.H
	for rows.Next() {
		var id int64
		var longUrl string
		var clickCount int
		var lastAccessed sql.NullString
		var expiresAt string
		err := rows.Scan(&id, &longUrl, &clickCount, &lastAccessed, &expiresAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read rows"})
			return
		}
		shortCode := intToBase62(id)
		urls = append(urls, gin.H{
			"id":            id,
			"long_url":      longUrl,
			"short_url":     DOMAIN + shortCode,
			"short_code":    shortCode,
			"click_count":   clickCount,
			"last_accessed": lastAccessed.String,
			"expires_at":    expiresAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"urls": urls})
}

func deleteUrl(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error":"Unauthorized"})
		return
	}
	shortCode := c.Param("id")

	id := base62ToInt(shortCode)

	_, err := db.Exec("DELETE FROM urls WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "URL deleted successfully"})
}

func getUrlStats(c *gin.Context) {
	shortCode := c.Param("id")

	id := base62ToInt(shortCode)

	var longUrl string
	var clickCount int
	var lastAccessed sql.NullString
	var expiresAt string
	err := db.QueryRow("SELECT long_url, click_count, last_accessed, expires_at FROM urls WHERE id = ?", id).Scan(&longUrl, &clickCount, &lastAccessed, &expiresAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
		return
	}

	urlStats := gin.H{
		"url":           longUrl,
		"click_count":   clickCount,
		"last_accessed": lastAccessed.String,
		"expires_at":    expiresAt,
	}

	c.JSON(http.StatusOK, urlStats)
}

func main() {
	var err error
	db, err = init_db()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "DELETE", "PUT", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.POST("/register", user_register)
	router.POST("/login", user_login)

	authGroup := router.Group("/")
	authGroup.Use(AuthMiddleware())
	{
	authGroup.POST("/submit", submitUrl)
	authGroup.GET("/urls", getUrls)
	authGroup.DELETE("/delete/:id", deleteUrl)
	authGroup.GET("/stats/:id", getUrlStats)
	// authGroup.GET("/me", user_validate)
	}

	router.GET("/:id", redirectUrl)
	router.Static("/static", "./client")
	router.Run("localhost:8080")
}
