package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"ningxia_backend/pkg/logger"
	"os"
	"path/filepath"
	"strings"
)

func ViewMarkdownHandler(c *gin.Context) {
	filename := c.Param("filename")
	fileFullName := filepath.Join(reportsBaseDir, filename[:strings.LastIndex(filename, ".")], filename)
	content, err := os.ReadFile(fileFullName)
	if err != nil {
		logger.Logger.Errorf("读取报告 %s 内容失败: %v", fileFullName, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取报告内容失败"})
		return
	}
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", content)
}
