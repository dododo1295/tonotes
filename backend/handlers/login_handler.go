package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"main/model"
	"main/repository"
	"main/services"
	"main/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/argon2"
)

// verify inputed password hash is same as the hash in  DB
func VerifyPassword(storedPassword, inputPassword string) (bool, error) {
	// check if password stored properly
	parts := strings.Split(storedPassword, "$")
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid format")
	}

	// take out salt and hash
	storedSalt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %v", err)
	}
	storedHash, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %v", err)
	}

	// hash input password with my input salt to check
	checkHash := argon2.IDKey([]byte(inputPassword), storedSalt, 3, 64*1024, 2, 32)
	// check for parity
	if !checkHashes(storedHash, checkHash) {
		return false, nil
	}

	return true, nil
}

// compared inputed hash w/ storedHash
func checkHashes(storedHash, checkHash []byte) bool {
	if len(storedHash) != len(checkHash) {
		return false
	}
	for i := range storedHash {
		if storedHash[i] != checkHash[i] {
			return false
		}
	}
	return true
}

func LoginHandler(c *gin.Context) {
	var user model.User

	// bind to struct
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Request"})
		return
	}

	// get user data from database

	repo := &repository.UsersRepo{
		MongoCollection: utils.MongoClient.Database("tonotes").Collection("users"),
	}

	// look for user
	fetchUser, err := repo.FindUserByUsername(user.Username)
	if err != nil || fetchUser == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid  username"})
		return
	}

	// check password
	checkPassword, err := VerifyPassword(fetchUser.Password, user.Password)
	if err != nil || !checkPassword {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect Password"})
		return
	}

	// generate token
	token, err := services.GenerateJWT(fetchUser.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// return the new token as response

	c.JSON(http.StatusOK, gin.H{"token": token})
}
