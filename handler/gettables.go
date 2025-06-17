package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"ningxia_backend/pkg/logger"
	"os"
	"path/filepath"
	"strings"
)

func GetTablesHandler(c *gin.Context) {
	filename := c.Param("dirname")
	reportDirPath := filepath.Join(reportsBaseDir, filename)

	entries, err := os.ReadDir(reportDirPath)
	if err != nil {
		logger.Logger.Errorf("查看报告目录 %s 失败: %v", reportDirPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出指标明细表失败"})
		return
	}

	var tables []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".xlsx") || strings.HasSuffix(entry.Name(), ".csv") || strings.HasSuffix(entry.Name(), ".xls") {
			tables = append(tables, entry.Name())
		}
	}
	c.JSON(http.StatusOK, gin.H{"tables": tables})
}
