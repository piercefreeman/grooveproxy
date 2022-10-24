package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TapeRequest struct {
	TapeID string `tapeID:"mode"`
}

type CacheModeRequest struct {
	Mode int `json:"mode"`
}

type DialerDefinitionRequest struct {
	Priority int `json:"priority"`

	ProxyServer   string `json:"proxyServer"`
	ProxyUsername string `json:"proxyUsername"`
	ProxyPassword string `json:"proxyPassword"`

	RequiresUrlRegex      string   `json:"requiresUrlRegex"`
	RequiresResourceTypes []string `json:"requiresResourceTypes"`
}

type DialerDefinitionRequests struct {
	Definitions []DialerDefinitionRequest `json:"definitions"`
}

func createController(recorder *Recorder, cache *Cache, dialerSession *DialerSession) *gin.Engine {
	router := gin.Default()
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Groove is running on port.")
	})

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
		var request TapeRequest
		err := json.NewDecoder(c.Request.Body).Decode(&request)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   err,
			})
			return
		}

		response, err := recorder.ExportData(request.TapeID)
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

	router.POST("/api/tape/clear", func(c *gin.Context) {
		var request TapeRequest
		err := json.NewDecoder(c.Request.Body).Decode(&request)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   err,
			})
			return
		}

		if request.TapeID == "" {
			recorder.Clear()
		} else {
			recorder.ClearTapeID(request.TapeID)
		}
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

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	})

	router.POST("/api/cache/clear", func(c *gin.Context) {
		cache.Clear()

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	})

	router.POST("/api/dialer/load", func(c *gin.Context) {
		var requests DialerDefinitionRequests
		err := json.NewDecoder(c.Request.Body).Decode(&requests)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   err,
			})
			return
		}

		dialerSession.DialerDefinitions = nil

		if len(requests.Definitions) == 0 {
			// If no requests are provided, default to passing through everything
			// so we're guaranteed to have one valid dialer
			dialerSession.DialerDefinitions = append(
				dialerSession.DialerDefinitions,
				NewDialerDefinition(0, nil, nil),
			)
		} else {
			for _, request := range requests.Definitions {
				var requestRequires *RequestRequiresDefinition = nil
				var proxy *ProxyDefinition = nil

				if request.RequiresUrlRegex != "" || len(request.RequiresResourceTypes) > 0 {
					requestRequires, err = NewRequestRequiresDefinition(request.RequiresUrlRegex, request.RequiresResourceTypes)
					if err != nil {
						c.JSON(http.StatusBadRequest, gin.H{
							"success": false,
							"error":   err,
						})
						return
					}
				}

				if request.ProxyServer != "" {
					proxy = &ProxyDefinition{
						url:      request.ProxyServer,
						username: request.ProxyUsername,
						password: request.ProxyPassword,
					}
				}

				dialerSession.DialerDefinitions = append(
					dialerSession.DialerDefinitions,
					NewDialerDefinition(
						request.Priority,
						proxy,
						requestRequires,
					),
				)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	})

	return router
}
