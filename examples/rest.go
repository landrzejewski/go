package examples

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

var database *sql.DB

func RestApi() {
	db, err := sql.Open("postgres", "postgres://admin:admin@localhost/users?sslmode=disable")
	if err != nil {
		// Bez przerwania działania kolejna linia dereferencjonowałaby nil *sql.DB.
		log.Fatalf("Nie można otworzyć połączenia z bazą: %v", err)
	}

	// DDL/DML wykonujemy przez Exec, nie Query. Query zwraca *sql.Rows, które
	// trzeba zamknąć - porzucone trzymało połączenie wypożyczone z puli aż do
	// finalizera GC.
	if _, err := db.Exec("create table if not exists users (id serial primary key, name varchar(100), email varchar(50))"); err != nil {
		log.Fatalf("Nie można utworzyć tabeli: %v", err)
	}
	database = db

	router := gin.Default()
	router.GET("/users", getUsers)
	router.GET("/users/:id", getUserById)
	router.POST("/users", createUser)
	router.PUT("/users/:id", updateUser)
	router.DELETE("/users/:id", deleteUser)
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Serwer zakończył działanie: %v", err)
	}
}

func getUsers(c *gin.Context) {
	// Kolumny wypisane jawnie: "select *" wiąże Scan z fizyczną kolejnością
	// kolumn, więc dodanie kolumny po cichu psuje odczyt.
	rows, err := database.Query("select id, name, email from users")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Bez Close połączenie wraca do puli dopiero po wyczerpaniu wyniku.
	defer rows.Close()

	var users = make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		users = append(users, user)
	}
	// rows.Err() odróżnia normalny koniec wyniku od błędu transportu w trakcie.
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

func getUserById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user id"})
		return
	}

	// QueryRow zamiast Query: poprzednia wersja wołała rows.Next() raz i wracała
	// bez Close, więc każde GET /users/:id wyciekało połączenie. Brak wiersza
	// rozpoznajemy po sql.ErrNoRows, a nie po heurystyce "user.ID == 0"
	// (która błędnie zgłaszałaby 404 dla legalnego rekordu o id 0).
	var user User
	err = database.QueryRow("select id, name, email from users where id = $1", id).
		Scan(&user.ID, &user.Name, &user.Email)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

func createUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := database.QueryRow("insert into users(name, email) values($1, $2) returning id", user.Name, user.Email).Scan(&user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

func updateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user id"})
		return
	}

	var updatedUser User
	if err := c.ShouldBindJSON(&updatedUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := database.Exec("update users set name = $1, email = $2 where id = $3", updatedUser.Name, updatedUser.Email, id)
	if err != nil {
		// gin.H{"error": err} serializowało interfejs error, który nie ma pól
		// eksportowanych - klient dostawał {"error":{}}. Potrzebny err.Error().
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Bez sprawdzenia RowsAffected aktualizacja nieistniejącego id kończyła się
	// statusem 200, więc 404 nigdy nie powstawało.
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	updatedUser.ID = id
	c.JSON(http.StatusOK, updatedUser)
}

func deleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user id"})
		return
	}
	result, err := database.Exec("delete from users where id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	// Bez tego handler kończył się bez zapisania odpowiedzi, więc gin zwracał
	// 200 OK z pustym ciałem zamiast 204 No Content.
	c.Status(http.StatusNoContent)
}
