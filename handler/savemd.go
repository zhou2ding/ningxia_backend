package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"ningxia_backend/pkg/logger"
	"os"
	"path/filepath"
	"strings"
)

func SaveMdHandler(pySuffix string) func(c *gin.Context) {
	return func(c *gin.Context) {
		var req struct {
			Files      []string `json:"files"`
			ReportType string   `json:"reportType"`
			Mileage    float64  `json:"mileage"`
			PQI        float64  `json:"pqi"`
			Timestamp  int64    `json:"timestamp"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Logger.Errorf("无效请求: %v", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "请求有误"})
			return
		}

		data, err := calculate(pySuffix, req.ReportType, req.Files, req.PQI, req.Mileage)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "计算失败"})
			return
		}

		var templateFile string
		switch req.ReportType {
		case ReportTypeExpressway:
			templateFile = "templates/高速公路JSON模板.md"
		case ReportTypeMaintenance:
			templateFile = "templates/养护工程JSON模板.md"
		case ReportTypeConstruction:
			templateFile = "templates/建设工程JSON模板.md"
		case ReportTypeRural:
			templateFile = "templates/农村公路JSON模板.md"
		case ReportTypeNationalProvincial:
			templateFile = "templates/国省干线JSON模板.md"
		//case ReportTypeMarket:
		//	templateFile = "templates/市场化JSON模板.md"
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "报告类型有误"})
			return
		}

		mdBytes, err := os.ReadFile(templateFile)
		if err != nil {
			logger.Logger.Errorf("读取MD模板失败 (%s): %v", templateFile, err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取 %s 模板失败", templateFile)})
			return
		}
		content := string(mdBytes)

		for key, value := range data {
			if key != PyRespImagesKey {
				valStr := fmt.Sprintf("%v", value)
				if valStr == "" {
					content = strings.ReplaceAll(content, key, " ")
				} else {
					content = strings.ReplaceAll(content, key, valStr)
				}
			}
		}

		reportBaseName := fmt.Sprintf("%s_%d", ReportNameMap[req.ReportType], req.Timestamp)
		images, ok := data[PyRespImagesKey].([]any)
		if ok {
			for _, image := range images {
				oldImageName := fmt.Sprintf("%s", image)
				newImageName := fmt.Sprintf("%s/images/%v", reportBaseName, image)
				imageUrl := fmt.Sprintf("http://127.0.0.1:12345/file?name=%s", url.QueryEscape(newImageName))
				content = strings.ReplaceAll(content, oldImageName, imageUrl)
			}
		}

		reportFilename := fmt.Sprintf("%s.md", reportBaseName)
		reportFileFullName := filepath.Join(reportsBaseDir, reportBaseName, reportFilename)
		if err = os.MkdirAll(filepath.Dir(reportFileFullName), 0755); err != nil {
			logger.Logger.Errorf("创建报告目录 (%s): %v", reportFileFullName, err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "创建报告目录失败"})
			return
		}

		if err = os.WriteFile(reportFileFullName, []byte(content), 0644); err != nil {
			logger.Logger.Errorf("Markdown文档写入失败 (%s): %v", reportFileFullName, err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Markdown文档生成失败"})
			return
		}

		logger.Logger.Infof("Markdown报告已生成: %s", reportFileFullName)
		c.JSON(http.StatusOK, gin.H{
			"message":  "Markdown报告生成成功",
			"filename": reportFilename,
		})
	}
}
