package handler

import (
	"github.com/gin-gonic/gin"
	"net/url"
	"path/filepath"
)

func GetFileHandler(c *gin.Context) {
	filename := c.Query("name")
	p, _ := url.QueryUnescape(filename)
	filename = filepath.Join(reportsBaseDir, p)
	c.Header("Content-Disposition", "inline")
	c.File(filename)
}
