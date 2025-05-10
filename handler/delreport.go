package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"ningxia_backend/pkg/logger"
	"os"
	"path/filepath"
	"strings"
)

func DeleteReportHandler(c *gin.Context) {
	filename := c.Param("filename")

	lastDotIndex := strings.LastIndex(filename, ".")
	if lastDotIndex == -1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件名格式"})
		return
	}
	baseName := filename[:lastDotIndex]

	reportDirPath := filepath.Join(reportsBaseDir, baseName)

	// 删除整个目录及其内容
	err := os.RemoveAll(reportDirPath)
	if err != nil {
		logger.Logger.Errorf("删除报告目录 %s 失败: %v", reportDirPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除报告失败"})
		return
	}

	logger.Logger.Infof("报告目录已删除: %s", reportDirPath)
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("报告 %s 删除成功", filename)})
}
