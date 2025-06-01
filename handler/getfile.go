package handler

import (
	"github.com/gin-gonic/gin"
	"net/url"
	"path/filepath"
)

func GetFileHandler(c *gin.Context) {
	filename := c.Query("name")
	p, _ := url.QueryUnescape(filename)
	filename = filepath.Join(ReportsBaseDir, p)
	c.Header("Content-Disposition", "inline")
	c.File(filename)
}
