package formaterror

import (
	"strings"

	"github.com/gin-gonic/gin"
)

var errorMessages = make(map[string]string)

var err error

func FormatError(errString string) map[string]string {
	if strings.Contains(errString, "username") {
		errorMessages["Taken_username"] = "Username Already Taken"
	}

	if strings.Contains(errString, "email") {
		errorMessages["Taken_email"] = "Email Already Taken"
	}

	if strings.Contains(errString, "title") {
		errorMessages["Taken_title"] = "Title Already Taken"
	}

	if strings.Contains(errString, "hashedPassword") {
		errorMessages["Incorrect_password"] = "Incorrect Password"
	}

	if strings.Contains(errString, "record not found") {
		errorMessages["No_record"] = "No Record Found"
	}

	if strings.Contains(errString, "double like") {
		errorMessages["Double_like"] = "You cannot like this post twice"
	}

	if strings.Contains(errString, "required email") {
		errorMessages["Required_email"] = "required email"
	}

	if strings.Contains(errString, "invalid email") {
		errorMessages["Invalid_email"] = "invalid email"
	}

	if strings.Contains(errString, "required password") {
		errorMessages["Required_password"] = "required password"
	}

	if len(errorMessages) > 0 {
		return errorMessages
	}

	if len(errorMessages) == 0 {
		errorMessages["Incorrect_details"] = "Incorrect Details"
	}

	return nil
}

func HandleError(c *gin.Context, statusCode int, errList map[string]string) {
	c.JSON(statusCode, gin.H{
		"status": statusCode,
		"error":  errList,
	})
}
