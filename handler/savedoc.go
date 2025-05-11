package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/nguyenthenguyen/docx"
	cp "github.com/otiai10/copy"
	"net/http"
	"ningxia_backend/pkg/logger"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func SaveDocxHandler(pySuffix string) func(c *gin.Context) {
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
			templateFile = "templates/高速公路JSON模板.docx"
		case ReportTypeMaintenance:
			templateFile = "templates/养护工程JSON模板.docx"
		case ReportTypeConstruction:
			templateFile = "templates/建设工程JSON模板.docx"
		case ReportTypeRural:
			templateFile = "templates/农村公路JSON模板.docx"
		case ReportTypeNationalProvincial:
			templateFile = "templates/国省干线JSON模板.docx"
		//case ReportTypeMarket:
		//	templateFile = "templates/市场化JSON模板.docx"
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "报告类型有误"})
			return
		}

		doc, err := docx.ReadDocxFile(templateFile)
		if err != nil {
			logger.Logger.Errorf("读取模板失败: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取 %s 模板失败", templateFile)})
			return
		}
		defer doc.Close()

		docxFile := doc.Editable()
		content := docxFile.GetContent()
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
		docxFile.SetContent(content)

		// --- 5. 准备报告目录和基础名称 ---
		// 报告基础名 (不含扩展名), 例如: 高速公路...报告_1745680397
		reportBaseName := fmt.Sprintf("%s_%d", ReportNameMap[req.ReportType], req.Timestamp)
		// 报告目录路径, 例如: ./reports/高速公路...报告_1745680397
		reportPath := filepath.Join(reportsBaseDir, reportBaseName)
		// 报告图片子目录路径
		if err = os.MkdirAll(reportPath, 0755); err != nil {
			logger.Logger.Errorf("创建报告目录 (%s): %v", reportPath, err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "创建报告目录失败"})
			return
		}

		images, ok := data[PyRespImagesKey].([]any)
		imageNames := make([]string, len(images))
		if ok {
			for i, image := range images {
				imageNames[i] = fmt.Sprintf("%v", image)
				// 复制报告图片
				var srcDir string
				switch req.ReportType {
				case ReportTypeExpressway:
					srcDir = expresswayReportBaseDir + "/images"
				case ReportTypeMaintenance:
					srcDir = maintenanceReportBaseDir + "/images"
				case ReportTypeConstruction:
					srcDir = constructionReportBaseDir + "/images"
				case ReportTypeRural:
					srcDir = ruralReportBaseDir + "/images"
				case ReportTypeNationalProvincial:
					srcDir = nationalProvinceReportBaseDir + "/images"
					//case ReportTypeMarket:
					//	srcDir = marketReportBaseDir + "/images"
				}
				err = cp.Copy(filepath.Join(srcDir, imageNames[i]), filepath.Join(reportPath, "images", imageNames[i]))
				if err != nil {
					logger.Logger.Errorf("拷贝图片 %s 失败: %v", imageNames[i], err)
				}
			}
		}
		for i := 0; i < docxFile.ImagesLen(); i++ {
			if i < len(imageNames) {
				imageName := filepath.Join(reportPath, "images", imageNames[i])
				err = docxFile.ReplaceImage("word/media/image"+strconv.Itoa(i+1)+".jpeg", imageName)
				if err != nil {
					logger.Logger.Errorf("替换图片失败: %v", err)
					//c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "替换图片失败"})
					//return
				}
			}
		}

		reportFilename := fmt.Sprintf("%s.docx", reportBaseName)
		reportFileFullName := filepath.Join(reportsBaseDir, reportBaseName, reportFilename)
		if err = docxFile.WriteToFile(reportFileFullName); err != nil {
			logger.Logger.Errorf("DOCX文档写入失败 (%s): %v", reportPath, err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "文档生成失败"})
			return
		}

		logger.Logger.Infof("标准DOCX报告已生成: %s", reportFileFullName)

		// --- 9. 条件性处理 EXTRA 模板 ---
		// 定义哪些类型需要处理 extra 模板
		processExtra := map[string]bool{
			ReportTypeExpressway:         true,
			ReportTypeRural:              true,
			ReportTypeNationalProvincial: true,
		}
		if processExtra[req.ReportType] {
			// 构建 extra 模板文件路径
			ext := filepath.Ext(templateFile)                 // .docx
			base := templateFile[:len(templateFile)-len(ext)] // templates/高速公路JSON模板
			extraTemplateFile := base + "_extra" + ext        // templates/高速公路JSON模板_extra.docx

			logger.Logger.Infof("检测到报告类型 %s，尝试处理额外模板: %s", req.ReportType, extraTemplateFile)

			// 读取 extra 模板
			extraDoc, err := docx.ReadDocxFile(extraTemplateFile)
			if err != nil {
				// 读取 extra 模板失败，记录错误，但不中止请求，因为主文件已生成
				logger.Logger.Errorf("读取额外模板 %s 失败: %v。将跳过额外文件的生成。", extraTemplateFile, err)
			} else {
				defer extraDoc.Close() // 确保 extra 模板文件读取器被关闭
				extraDocxFile := extraDoc.Editable()
				extraContent := extraDocxFile.GetContent()

				// 替换 extra 模板中的文本 (使用相同的数据)
				for key, value := range data {
					if key != PyRespImagesKey {
						valStr := fmt.Sprintf("%v", value)
						placeholder := key
						if valStr == "" {
							extraContent = strings.ReplaceAll(extraContent, placeholder, " ")
						} else {
							extraContent = strings.ReplaceAll(extraContent, placeholder, valStr)
						}
					}
				}
				extraDocxFile.SetContent(extraContent)

				extraImages, extraOk := data[PyRespExtraImagesKey].([]any)
				extraImageNames := make([]string, len(images))
				if extraOk {
					for i, image := range extraImages {
						extraImageNames[i] = fmt.Sprintf("%v", image)
					}
				}

				for i := 0; i < extraDocxFile.ImagesLen(); i++ {
					if i < len(extraImageNames) {
						imageName := filepath.Join(reportPath, "images", extraImageNames[i])
						err = extraDocxFile.ReplaceImage("word/media/image"+strconv.Itoa(i+1)+".jpeg", imageName)
						if err != nil {
							logger.Logger.Errorf("替换图片失败: %v", err)
						}
					}
				}

				// 构建 extra 输出文件名和路径
				lastUnderscore := strings.LastIndex(reportBaseName, "_")
				var extraOutputFilename string
				// 确保能正确分割基础名和时间戳
				if lastUnderscore > 0 && lastUnderscore < len(reportBaseName)-1 {
					basePart := reportBaseName[:lastUnderscore]                             // 高速...报告
					tsPart := reportBaseName[lastUnderscore+1:]                             // 1745680397
					extraOutputFilename = fmt.Sprintf("%s_extra_%s.docx", basePart, tsPart) // 高速...报告_extra_1745680397.docx
				} else {
					// 如果原始文件名不含 '_' 或 '_' 在末尾，提供一个备用名称
					extraOutputFilename = fmt.Sprintf("%s_extra.docx", reportBaseName)
					logger.Logger.Warnf("无法从 '%s' 标准化解析基础名和时间戳，额外文件名设为: %s", reportBaseName, extraOutputFilename)
				}

				extraOutputFileFullName := filepath.Join(reportPath, extraOutputFilename) // 完整路径

				// 写入处理后的 extra 文档
				if err = extraDocxFile.WriteToFile(extraOutputFileFullName); err != nil {
					// 写入 extra 文件失败，记录错误，但不中止请求
					logger.Logger.Errorf("额外DOCX文档写入失败 (%s): %v", extraOutputFileFullName, err)
				} else {
					logger.Logger.Infof("额外DOCX报告已生成: %s", extraOutputFileFullName)
				}
			}
		}
		time.Sleep(45 * time.Second)
		c.JSON(http.StatusOK, gin.H{
			"message":  "docx报告生成成功",
			"filename": reportFilename,
		})
	}
}
