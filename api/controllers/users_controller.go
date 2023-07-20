package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Mdromi/go-forum-backend-gh/api/auth"
	"github.com/Mdromi/go-forum-backend-gh/api/models"
	"github.com/Mdromi/go-forum-backend-gh/api/security"
	"github.com/Mdromi/go-forum-backend-gh/api/utils/fileformat"
	"github.com/Mdromi/go-forum-backend-gh/api/utils/formaterror"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func (server *Server) CreateUser(c *gin.Context) {
	// clear previous error if any
	errList = map[string]string{}

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		errList["Invalid_body"] = "Unable to get request"
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": http.StatusUnprocessableEntity,
			"error":  errList,
		})
		return
	}

	user := models.User{}

	err = json.Unmarshal(body, &user)
	if err != nil {
		errList["Unmarshal_error"] = "Cannot unmarshal body"
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": http.StatusUnprocessableEntity,
			"error":  errList,
		})
		return
	}

	user.Prepare()
	errorMessages := user.Validate("")
	if len(errorMessages) > 0 {
		errList = errorMessages
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": http.StatusUnprocessableEntity,
			"error":  errList,
		})
		return
	}

	userCreated, err := user.SaveUser(server.DB)
	if err != nil {
		formattedError := formaterror.FormatError(err.Error())
		errList = formattedError
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": http.StatusUnprocessableEntity,
			"error":  errList,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":   http.StatusCreated,
		"response": userCreated,
	})
}

func (server *Server) GetUsers(c *gin.Context) {
	// clear previous error if any
	errList = map[string]string{}

	user := models.User{}

	users, err := user.FindAllUsers(server.DB)
	if err != nil {
		errList["No_user"] = "No User Found"
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": http.StatusInternalServerError,
			"error":  errList,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":   http.StatusOK,
		"response": users,
	})
}

func (server *Server) GetUser(c *gin.Context) {
	userID := c.Param("id")

	uid, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		errList["Invalid_request"] = "Invalid Request"
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusBadRequest,
			"error":  errList,
		})
		return
	}
	user := models.User{}

	userGotten, err := user.FindUserByID(server.DB, uint32(uid))
	if err != nil {
		errList["Invalid_request"] = "Invalid Request"
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusBadRequest,
			"error":  errList,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":   http.StatusOK,
		"response": userGotten,
	})
}

// IF YOU ARE USING AMAZON S3
func SaveProfileImage(s *session.Session, file *multipart.FileHeader) (string, error) {
	size := file.Size
	buffer := make([]byte, size)
	f, err := file.Open()
	if err != nil {
		fmt.Println("This is the error: ")
		fmt.Println(err)
	}
	defer f.Close()
	filePath := "/profile-photos/" + fileformat.UniqueFormat(file.Filename)
	f.Read(buffer)
	fileBytes := bytes.NewReader(buffer)
	fileType := http.DetectContentType(buffer)

	_, err = s3.New(s).PutObject(&s3.PutObjectInput{
		ACL:           aws.String("public-read"),
		Body:          fileBytes,
		Bucket:        aws.String("chodapibucket"),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(fileType),
		Key:           aws.String(filePath),
	})
	if err != nil {
		return "", err
	}
	return filePath, err
}

func (server *Server) UpdateAvatar(c *gin.Context) {
	// clear previous error if any
	errList = map[string]string{}

	var err error
	err = godotenv.Load()
	if err != nil {
		log.Fatalf("Error gotting env, %v", err)
	}

	userID := c.Param("id")
	// check if the user id is valid
	uid, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		errList["Invalid_request"] = "Invalid Request"
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusBadRequest,
			"error":  errList,
		})
		return
	}

	// Get user id from the token for valid tokens
	tokenID, err := auth.ExtractTokenID(c.Request)
	if err != nil {
		errList["Unauthorized"] = "Unauthorized"
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusBadRequest,
			"error":  errList,
		})
		return
	}

	// if the id is not the authenticated user id
	if tokenID != 0 && tokenID != uint32(uid) {
		errList["Unauthorized"] = "Unauthorized"
		c.JSON(http.StatusUnauthorized, gin.H{
			"status": http.StatusUnauthorized,
			"error":  errList,
		})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		errList["Invalid_file"] = "Invalid File"
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": http.StatusUnprocessableEntity,
			"error":  errList,
		})
		return
	}

	f, err := file.Open()
	if err != nil {
		errList["Invalid_file"] = "Invalid File"
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": http.StatusUnprocessableEntity,
			"error":  errList,
		})
		return
	}
	defer f.Close()

	size := file.Size
	// The image should not be more than 500KB
	if size > int64(512000) {
		errList["To_large"] = "Sorry, Please upload an Image of 500KB or less"
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": http.StatusUnprocessableEntity,
			"error":  errList,
		})
		return
	}
	buffer := make([]byte, size)
	f.Read(buffer)
	fileBytes := bytes.NewReader(buffer)
	fileType := http.DetectContentType(buffer)
	// if the image is valid
	if !strings.HasPrefix(fileType, "image") {
		errList["Not_Image"] = "Please Upload a valid image"
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": http.StatusUnprocessableEntity,
			"error":  errList,
		})
		return
	}
	filePath := fileformat.UniqueFormat(file.Filename)
	path := "/profile-photos/" + filePath
	params := &s3.PutObjectInput{
		Bucket:        aws.String("chodapi"),
		Key:           aws.String(path),
		Body:          fileBytes,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(fileType),
		ACL:           aws.String("public-read"),
	}
	s3Config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(
			os.Getenv("DO_SPACES_KEY"), os.Getenv("DO_SPACES_SECRET"), os.Getenv("DO_SPACES_TOKEN")),
		Endpoint: aws.String(os.Getenv("DO_SPACES_ENDPOINT")),
		Region:   aws.String(os.Getenv("DO_SPACES_REGION")),
	}
	newSession := session.New(s3Config)
	s3Client := s3.New(newSession)

	_, err = s3Client.PutObject(params)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	//IF YOU PREFER TO USE AMAZON S3
	// s, err := session.NewSession(&aws.Config{Too_large
	// 	Region: aws.String("us-east-1"),
	// 	Credentials: credentials.NewStaticCredentials(
	// 		os.Getenv("AWS_KEY"),
	// 		os.Getenv("AWS_SECRET"),
	// 		os.Getenv("AWS_TOKEN"),
	// 		),
	// })
	// if err != nil {
	// 	fmt.Printf("Could not upload file first error: %s\n", err)
	// }
	// fileName, err := SaveProfileImage(s, file)
	// if err != nil {
	// 	fmt.Printf("Could not upload file %s\n", err)
	// } else {
	// 	fmt.Printf("Image uploaded: %s\n", fileName)
	// }

	// save The iamge path to the database
	user := models.User{}
	user.AvatarPath = filePath
	user.Prepare()
	updatedUser, err := user.UpdateAUserAvatar(server.DB, uint32(uid))

	if err != nil {
		errList["Cannot_Save"] = "Cannot Save Image, Pls try again later"
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": http.StatusInternalServerError,
			"error":  errList,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":   http.StatusOK,
		"response": updatedUser,
	})
}

// TASK: repeted error code handle
func (server *Server) UpdateUser(c *gin.Context) {
	// clear previous error if any
	errList = map[string]string{}

	userID := c.Param("id")
	// check the user id is  valid
	uid, err := strconv.ParseUint(userID, 10, 20)
	if err != nil {
		errList["Invalid_request"] = "Invalid Request"
		formaterror.HandleError(c, http.StatusBadRequest, errList)
		return
	}

	// Get user id from token for valid tokens
	tokenID, err := auth.ExtractTokenID(c.Request)
	if err != nil {
		errList["Unauthorized"] = "Unauthorized"
		formaterror.HandleError(c, http.StatusUnauthorized, errList)
		return
	}

	// if the id is not the authentiacation user id
	if tokenID != 0 && tokenID != uint32(uid) {
		errList["Unauthorized"] = "Unauthorized"
		formaterror.HandleError(c, http.StatusUnauthorized, errList)
		return
	}

	// start processing the request
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		errList["Invalid_body"] = "Unable to get request"
		formaterror.HandleError(c, http.StatusUnprocessableEntity, errList)
		return
	}

	requestBody := map[string]string{}
	err = json.Unmarshal(body, &requestBody)
	if err != nil {
		errList["Unmarshal_error"] = "Cannot unmarshal body"
		formaterror.HandleError(c, http.StatusUnprocessableEntity, errList)
		return
	}

	// check for previous details
	formerUser := models.User{}
	err = server.DB.Debug().Model(models.User{}).Where("id = ?", uid).Take(&formerUser).Error
	if err != nil {
		errList["User_invalid"] = "The user is does not exist"
		formaterror.HandleError(c, http.StatusUnprocessableEntity, errList)
		return
	}

	newUser := models.User{}

	// when current password has content.
	if requestBody["current_password"] == "" && requestBody["new_password"] != "" {
		errList["Empty_current"] = "Please Provide current password"
		formaterror.HandleError(c, http.StatusUnprocessableEntity, errList)
		return
	}
	if requestBody["current_password"] != "" && requestBody["Empty_new"] == "" {
		errList["Empty_current"] = "Please Provide new password"
		formaterror.HandleError(c, http.StatusUnprocessableEntity, errList)
		return
	}
	if requestBody["current_password"] != "" && requestBody["new_password"] != "" {
		// also check if the new password
		if len(requestBody["new_password"]) < 6 {
			errList["Invalid_password"] = "Please should be atlest 6 charcters"
			formaterror.HandleError(c, http.StatusUnprocessableEntity, errList)
			return
		}
		// if they do, check that the former password is correct
		err = security.VerifyPassword(formerUser.Password, requestBody["current_password"])

		if err != nil && err == bcrypt.ErrMismatchedHashAndPassword {
			errList["Password_mismatch"] = "The password not correct"
			formaterror.HandleError(c, http.StatusUnprocessableEntity, errList)
			return
		}

		// update both the password and the email
		newUser.Username = formerUser.Username // remeber, you cannot update the username
		newUser.Email = requestBody["email"]
		newUser.Password = requestBody["new_password"]
	}
	// The password fields not entered, so update only the email
	newUser.Username = formerUser.Username
	newUser.Email = requestBody["email"]

	newUser.Prepare()
	errorMessages := newUser.Validate("update")
	if len(errorMessages) > 0 {
		errList = errorMessages
		formaterror.HandleError(c, http.StatusUnprocessableEntity, errList)
		return
	}

	updatedUser, err := newUser.UpdateAUser(server.DB, uint32(uid))
	if err != nil {
		errList := formaterror.FormatError(err.Error())
		formaterror.HandleError(c, http.StatusInternalServerError, errList)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":   http.StatusOK,
		"response": updatedUser,
	})
}

func (server *Server) DeleteUser(c *gin.Context) {
	// clear previous error if any
	errList = map[string]string{}
	var tokenID uint32
	userID := c.Param("id")
	// check if the user id is valid
	uid, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		errList["Invalid_request"] = "Invalid Request"
		c.JSON(http.StatusBadRequest, gin.H{
			"status": http.StatusBadRequest,
			"error":  errList,
		})
		return
	}

	// get user id from the token for valid tokens
	tokenID, err = auth.ExtractTokenID(c.Request)

	if err != nil {
		errList["Unauthorized"] = "Unauthorized"
		c.JSON(http.StatusUnauthorized, gin.H{
			"status": http.StatusUnauthorized,
			"error":  errList,
		})
		return
	}
	// If the id is not the authenticated user id
	if tokenID != 0 && tokenID != uint32(uid) {
		errList["Unauthorized"] = "Unauthorized"
		c.JSON(http.StatusUnauthorized, gin.H{
			"status": http.StatusUnauthorized,
			"error":  errList,
		})
		return
	}

	user := models.User{}
	_, err = user.DeleteAUser(server.DB, uint32(uid))
	if err != nil {
		errList["Other_error"] = "Please try again later"
		c.JSON(http.StatusNotFound, gin.H{
			"status": http.StatusNotFound,
			"error":  errList,
		})
		return
	}

	// Also delete the posts, likes and the comments that this user created if any:
	comment := models.Comment{}
	like := models.Like{}
	post := models.Post{}

	_, err = post.DeleteUserPosts(server.DB, uint32(uid))
	if err != nil {
		errList["Other_error"] = "Please try again later"
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": http.StatusInternalServerError,
			"error":  err,
		})
		return
	}
	_, err = comment.DeleteUserComments(server.DB, uint32(uid))
	if err != nil {
		errList["Other_error"] = "Please try again later"
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": http.StatusInternalServerError,
			"error":  err,
		})
		return
	}
	_, err = like.DeleteUserLikes(server.DB, uint32(uid))
	if err != nil {
		errList["Other_error"] = "Please try again later"
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": http.StatusInternalServerError,
			"error":  err,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   http.StatusOK,
		"response": "User deleted",
	})
}
