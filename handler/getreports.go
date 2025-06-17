package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"ningxia_backend/pkg/logger"
	"os"
	"sort"
)

func GetReports(c *gin.Context) {
	//skipDir := filepath.Join("images")

	//err := filepath.Walk(reportsBaseDir, func(path string, info os.FileInfo, err error) error {
	//	if err != nil {
	//		return err
	//	}
	//	if info.IsDir() && strings.Contains(path, skipDir) {
	//		return filepath.SkipDir
	//	}
	//	if !info.IsDir() && strings.Contains(path, ".md") && !strings.Contains(path, "_extra") {
	//		basename := filepath.Base(path)
	//		reportName := strings.TrimSuffix(basename, filepath.Ext(basename))
	//		reports = append(reports, reportName)
	//	}
	//	return nil
	//})

	reports := make([]string, 0)
	dirs, err := os.ReadDir(reportsBaseDir)
	for _, dir := range dirs {
		reports = append(reports, dir.Name())
	}
	if err != nil {
		logger.Logger.Errorf("遍历目录失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查看报表列表失败"})
		return
	}
	sort.Slice(reports, func(i, j int) bool {
		timestampI := extractTimestamp(reports[i])
		timestampJ := extractTimestamp(reports[j])
		return timestampI > timestampJ
	})
	c.JSON(http.StatusOK, reports)
}
