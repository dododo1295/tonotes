package usecase

import (
	"log"
	"net/http"

	"main/model"
	"main/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
)

// NOTE: This repo STILL does not have the ability to find username via email

type UserService struct {
	MongoCollection *mongo.Collection
}

type Response struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// creating user and inserting uuid
func (svc *UserService) CreateUser(c *gin.Context) {
	res := &Response{}

	var user model.User

	if err := c.ShouldBindJSON(&user); err == nil {
		// Response is Bad Request
		c.JSON(400, Response{Error: "Error decoding: " + err.Error()})
		log.Println("Error decoding: ", err)
		return
	}

	// assign user ID
	user.UserID = uuid.NewString()

	// inserting the ID with the new User
	repo := repository.UsersRepo{MongoCollection: svc.MongoCollection}

	insertID, err := repo.AddUser(&user)
	if err != nil {
		c.JSON(400, Response{Error: "Error adding user ID: " + err.Error()})
		log.Println("error adding user: ", err)

		return
	}

	res.Data = user.UserID
	c.JSON(200, res)

	log.Println("user inserted with id: ", insertID)
}

// Retreive User
func (svc *UserService) GetUserID(c *gin.Context) {
	res := &Response{}

	// Get the users ID
	userID := c.Param("user_id")

	log.Println("user ID: ", userID)

	repo := repository.UsersRepo{MongoCollection: svc.MongoCollection}

	user, err := repo.FindUser(userID)
	if err != nil {
		c.JSON(400, Response{Error: "Could not find user: " + err.Error()})
		log.Println("Could not find user: ", err)

		return
	}
	res.Data = user
	c.JSON(http.StatusOK, res)
}

func (svc *UserService) UpdateUserPassword(c *gin.Context) {
	res := &Response{}

	// getting the user ID
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, Response{Error: "userID is required"})
		return
	}

	var bodyRequest struct {
		NewHashedPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&bodyRequest); err != nil {
		c.JSON(http.StatusBadRequest, Response{Error: "Error binding request: " + err.Error()})
	}
	// making sure password isn't empty in error
	if bodyRequest.NewHashedPassword == "" {
		c.JSON(http.StatusBadRequest, Response{Error: "Error retrieving hashed password"})
		return
	}

	repo := repository.UsersRepo{MongoCollection: svc.MongoCollection}
	modifiedCount, err := repo.UpdateUserPassword(userID, bodyRequest.NewHashedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Error: "Error updating password: " + err.Error()})
		return
	}

	// off chance the userID isn't working
	if modifiedCount == 0 {
		c.JSON(http.StatusNotFound, Response{Error: "User ID could not be found"})
		return
	}

	res.Data = "password successfully changed"
	c.JSON(http.StatusOK, res)
}

func (svc *UserService) UpdateUserByID(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	res := &Response{}

	userID := c.Param("user_id")
	log.Println("user ID: ", userID)

	if userID == "" {
		c.JSON(http.StatusBadRequest, Response{Error: "invalid user id"})
		log.Println("invalid user ID")
		return
	}

	var user model.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, Response{Error: "could not bind: " + err.Error()})
		log.Println("error decoding")
		return
	}

	user.UserID = userID

	repo := repository.UsersRepo{MongoCollection: svc.MongoCollection}
	count, err := repo.UpdateUserByID(userID, &user)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Error: "could not update user name"})
		log.Println("error updating usernmae")
		return
	}

	res.Data = count
	c.JSON(http.StatusOK, res)
}

func (svc *UserService) DeleteUser(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	res := &Response{}

	userID := c.Param("user_id")

	log.Println("user id: ", userID)

	if userID == "" {
		c.JSON(http.StatusBadRequest, Response{Error: "invalid user id"})
		log.Println("invalid user ID")
		return
	}

	var user model.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, Response{Error: "could not bind: " + err.Error()})
		log.Println("error decoding")
		return
	}

	repo := repository.UsersRepo{MongoCollection: svc.MongoCollection}

	count, err := repo.DeleteUserByID(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not delete user: ", "details": err.Error()})
	}

	res.Data = count
	c.JSON(http.StatusOK, res)
}
