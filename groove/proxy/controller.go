package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type CacheModeRequest struct {
	Mode int `json:"mode"`
}

type ProxyRequest struct {
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func createController(recorder *Recorder, cache *Cache, endProxy *EndProxy) *gin.Engine {
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

	router.POST("/api/cache/mode", func(c *gin.Context) {
		var request CacheModeRequest
		err := json.NewDecoder(c.Request.Body).Decode(&request)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   err,
			})
			return
		}

		cache.mode = request.Mode
		log.Printf("Cache mode set: %d\n", cache.mode)

		if cache.mode == CacheModeOff {
			cache.Clear()
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	})

	router.POST("/api/proxy/start", func(c *gin.Context) {
		var request ProxyRequest
		err := json.NewDecoder(c.Request.Body).Decode(&request)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   err,
			})
			return
		}

		endProxy.updateProxy(request.Server, request.Username, request.Password)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	})

	router.POST("/api/proxy/stop", func(c *gin.Context) {
		endProxy.disableProxy()

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	})

	return router
}
