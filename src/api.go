package main

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"strings"
	"fmt"
	"github.com/satori/go.uuid"
	"gopkg.in/h2non/filetype.v1"
	log "github.com/Sirupsen/logrus"
	"flag"
)


func main(){
	fmt.Printf("Starting API Service...\n")

	log.SetLevel(log.DebugLevel)

	releaseMode := flag.Bool("release", false, "Run in release mode")
	wordlistDir := flag.String("wordlist", "../wordlists/en/misc.txt", "Path to wordlist")
	donationsDir := flag.String("donations_dir", "../donations/", "Location of the uploaded donations")
	unverifiedDonationsDir := flag.String("unverified_donations_dir", "../unverified_donations/", "Location of the uploaded but unverified donations")

	flag.Parse()
	if(*releaseMode){
		fmt.Printf("Starting gin in release mode!\n")
		gin.SetMode(gin.ReleaseMode)
	}

	fmt.Printf("Reading wordlists...\n")
	words, err := getWordLists(*wordlistDir)
	if(err != nil){
		fmt.Printf("Couldn't read wordlists...terminating!")
		log.Fatal(err)
	}

	router := gin.Default()

	router.Static("./v1/donation", *donationsDir) //serve static images

	router.Use(func(c *gin.Context) {
		if values, _ := c.Request.Header["X-Request-Id"]; len(values) > 0 {
			if(values[0] != ""){
				c.Writer.Header().Set("X-Request-Id", values[0])
			}
		}
    })

	router.GET("/v1/validate", func(c *gin.Context) {
		randomImage := getRandomImage()
		c.JSON(http.StatusOK, gin.H{"uuid": randomImage.Id, "url": randomImage.Url, "label": randomImage.Label, "provider": randomImage.Provider})
	})

	router.POST("/v1/validate/:imageid/:param", func(c *gin.Context) {
		imageId := c.Param("imageid")
		param := c.Param("param")

		parameter := false
		if(param == "yes"){
			parameter = true
		} else if(param == "no"){
			parameter = false
		} else{
			c.JSON(404, nil)
			return
		}

		err := validateDonatedPhoto(imageId, parameter)
		if(err != nil){
			c.JSON(http.StatusInternalServerError, gin.H{"Error": "Database Error: Couldn't update data"})
			return
		} else{
			c.JSON(http.StatusOK, nil)
			return
		}
	})

	router.GET("/v1/export", func(c *gin.Context) {
		tags := ""
		params := c.Request.URL.Query()
		if temp, ok := params["tags"]; ok {
			tags = temp[0]
			jsonData, err := export(strings.Split(tags, ","))
			if(err == nil){
				c.JSON(http.StatusOK, jsonData)
				return
			} else{
				c.JSON(http.StatusInternalServerError, gin.H{"Error": "Couldn't export data, please try again later."})
				return
			}
		} else {
			c.JSON(422, gin.H{"error": "no tags specified"})
			return
		}
	})

	router.GET("/v1/label", func(c *gin.Context) {
		label := words[random(0, len(words) - 1)]
		c.JSON(http.StatusOK, gin.H{"label": label})
	})

	router.POST("/v1/donate", func(c *gin.Context) {
		label := c.PostForm("label")

		file, header, err := c.Request.FormFile("image")
		if(err != nil){
			fmt.Printf("err = %s", err.Error())
			c.JSON(400, gin.H{"error": "Picture is missing"})
			return
		}

		// Create a buffer to store the header of the file
        fileHeader := make([]byte, 512)

        // Copy the file header into the buffer
        if _, err := file.Read(fileHeader); err != nil {
        	c.JSON(422, gin.H{"error": "Unable detect MIME type"})
        	return
        }

        // set position back to start.
        if _, err := file.Seek(0, 0); err != nil {
        	c.JSON(422, gin.H{"error": "Unable detect MIME type"})
        	return
        }

        if(!filetype.IsImage(fileHeader)){
        	c.JSON(422, gin.H{"error": "Unsopported MIME type detected"})
        	return
        }

		uuid := uuid.NewV4().String()
		c.SaveUploadedFile(header, (*unverifiedDonationsDir + uuid))

		err = addDonatedPhoto(uuid, label)
		if(err != nil){
			c.JSON(500, gin.H{"error": "Couldn't add photo - please try again later"})	
			return
		}

		c.JSON(http.StatusOK, nil)
	})

	router.POST("/v1/report/:imageid", func(c *gin.Context) {
		imageId := c.Param("imageid")
		var report Report
		if(c.BindJSON(&report) != nil){
			c.JSON(422, gin.H{"error": "reason missing - please provide a valid 'reason'"})
			return
		}
		err := reportImage(imageId, report.Reason)
		if(err != nil){
			c.JSON(500, gin.H{"error": "Couldn't report image - please try again later"})
			return
		}
		c.JSON(http.StatusOK, nil)
	})

	router.Run(":8081")
}