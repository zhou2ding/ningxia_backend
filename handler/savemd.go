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
	/*
		todo:
			1. 先读取output目录下的文件夹
			2. 根据不同的公路类型，定义不同的结构体（主要是计算指标）
			3. 调用py程序后，重新读取output目录下的文件夹，找到新增的那个文件夹
			4. 在新增的那个文件夹中，生成markdown报告
	*/
	return func(c *gin.Context) {
		var req struct {
			Files      []string `json:"files"`
			ReportType string   `json:"reportType"`
			Mileage    float64  `json:"mileage"`
			PQI        float64  `json:"pqi"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Logger.Errorf("无效请求: %v", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "请求有误"})
			return
		}

		data, reportDirFromCalc, err := calculate(pySuffix, req.ReportType, req.Files, req.PQI, req.Mileage)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "计算失败"})
			return
		}

		var templateFile string
		switch req.ReportType {
		case ReportTypeExpressway:
			templateFile = "templates/highway.md"
		case ReportTypeMaintenance:
			templateFile = "templates/maintenance.md"
		case ReportTypeConstruction:
			templateFile = "templates/maintenance.md"
		case ReportTypeRural:
			templateFile = "templates/rural.md"
		case ReportTypeNationalProvincial:
			templateFile = "templates/countryProvince.md"
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

		var tableNamesFromJSON []string
		if tables, ok := data["TABLE"]; ok {
			if tableList, ok := tables.([]any); ok {
				for _, tableNameAny := range tableList {
					if tableNameStr, ok := tableNameAny.(string); ok {
						tableNamesFromJSON = append(tableNamesFromJSON, tableNameStr)
					}
				}
			}
		} else if tables, ok := data["Tables"]; ok {
			if tableList, ok := tables.([]any); ok {
				for _, tableNameAny := range tableList {
					if tableNameStr, ok := tableNameAny.(string); ok {
						tableNamesFromJSON = append(tableNamesFromJSON, tableNameStr)
					}
				}
			}
		}

		for key, value := range data {
			if key == PyRespImagesKey || key == "Tables" || key == "TABLE" {
				// Skip image data and table lists themselves
				continue
			}
			// Check if the key is one of the table names (to avoid replacing a placeholder that is a table name itself with a non-table value)
			isKeyATableName := false
			for _, tn := range tableNamesFromJSON {
				if key == tn {
					isKeyATableName = true
					break
				}
			}
			if isKeyATableName {
				continue
			}
			valStr := fmt.Sprintf("%v", value)
			if valStr == "" {
				content = strings.ReplaceAll(content, key, " ")
			} else {
				content = strings.ReplaceAll(content, key, valStr)
			}
		}

		for _, tableName := range tableNamesFromJSON {
			tableXlsxFilePath := filepath.Join(reportDirFromCalc, tableName)

			markdownTable, err := convertExcelToMarkdown(tableXlsxFilePath)
			if err != nil {
				logger.Logger.Errorf("处理Excel表格 '%s' 失败: %v", tableXlsxFilePath, err)
				// 在报告中标记表格加载失败，而不是让整个过程失败
				errorMsg := fmt.Sprintf("\n>[表格 '%s.xlsx' 加载或转换失败: %v]\n", tableName, err)
				content = strings.ReplaceAll(content, tableName, errorMsg) // 用错误信息替换占位符
				continue
			}

			// 用生成的Markdown表格替换模板中的占位符（占位符就是表格名本身）
			content = strings.ReplaceAll(content, tableName, markdownTable)
		}

		//timeStr := strings.Split(reportDirFromCalc, "_")[1]
		//reportBaseName := fmt.Sprintf("%s_%s", ReportNameMap[req.ReportType], timeStr)
		images, ok := data[PyRespImagesKey].([]any)
		if ok {
			for _, image := range images {
				oldImageName := fmt.Sprintf("%s", image)
				newImageName := fmt.Sprintf("%s/images/%v", reportDirFromCalc, image)
				imageUrl := fmt.Sprintf("http://127.0.0.1:12345/file?name=%s", url.QueryEscape(newImageName))
				content = strings.ReplaceAll(content, oldImageName, imageUrl)
			}
		}

		// --- Save Final Markdown ---
		reportInstanceDir := reportDirFromCalc
		reportFilename := fmt.Sprintf("%s.md", filepath.Base(reportInstanceDir))

		reportFullName := filepath.Join(reportInstanceDir, reportFilename)
		if err = os.WriteFile(reportFullName, []byte(content), 0644); err != nil {
			logger.Logger.Errorf("Markdown文档写入失败 (%s): %v", reportFullName, err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Markdown文档生成失败"})
			return
		}

		logger.Logger.Infof("Markdown报告已生成: %s", reportFilename)
		c.JSON(http.StatusOK, gin.H{
			"message":  "Markdown报告生成成功",
			"filename": reportFilename,
		})
	}
}
