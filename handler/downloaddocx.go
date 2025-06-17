package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/url"
	"path/filepath"
	"strings"
)

func DownloadWordHandler(c *gin.Context) {
	filename := c.Param("filename")
	fmt.Printf("DEBUG: Received filename parameter: [%s]\n", filename)
	fmt.Printf("DEBUG: Received filename parameter (bytes): %x\n", []byte(filename))

	fileFullName := filepath.Join(reportsBaseDir, filename[:strings.LastIndex(filename, ".")], filename)
	encodedFilename := url.QueryEscape(filename)

	disposition := fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", filename, encodedFilename)
	fmt.Printf("DEBUG: Setting Content-Disposition: [%s]\n", disposition)

	c.Header("Content-Disposition", disposition)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")

	c.File(fileFullName)
}
