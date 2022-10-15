package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func createController(recorder *Recorder) *gin.Engine {
	router := gin.Default()
	router.POST("/api/tape/record", func(c *gin.Context) {
		// Start to record the requests, nullifying any ones from an old session
		recorder.mode = RecorderModeWrite
		recorder.Clear()

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	})

	router.POST("/api/tape/stop", func(c *gin.Context) {
		// Stop recording requests, but don't call Stop because we want to keep
		// the tape data around in case users access it
		recorder.mode = RecorderModeOff

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	})

	router.POST("/api/tape/retrieve", func(c *gin.Context) {
		response, err := recorder.ExportData()
		if err != nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		c.Data(http.StatusOK, "application/x-gzip", response.Bytes())
	})

	router.POST("/api/tape/load", func(c *gin.Context) {
		recorder.mode = RecorderModeRead
		recorder.Clear()

		file, _ := c.FormFile("file")

		fileHandler, err := file.Open()
		if err != nil {
			fmt.Println(err)
		}

		recorder.LoadData(fileHandler)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	})

	return router
}
