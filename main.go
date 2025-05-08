package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gomarkdown/markdown"
	"github.com/nguyenthenguyen/docx"
	cp "github.com/otiai10/copy"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/font"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io"
	"log"
	"net/http"
	"net/url"
	"ningxia_backend/pkg/conf"
	"ningxia_backend/pkg/logger"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	uploadDir                     = "./tmp/uploads"
	pdfDir                        = "./tmp/pdf"
	reportsBaseDir                = "./reports" // Base directory for saved reports
	expresswayReportBaseDir       = "./reports/expressway"
	postEvaluationReportBaseDir   = "./reports/postEvaluation"
	constructionReportBaseDir     = "./reports/construction"
	ruralReportBaseDir            = "./reports/rural"
	nationalProvinceReportBaseDir = "./reports/nationalProvince"
	marketReportBaseDir           = "./reports/market"
	maxFileSize                   = 1024 * 1024 * 1024 // 1024MB
	dbFile                        = "./road.db"
	wkhtmltopdfPath               = "./wkhtmltox/bin/wkhtmltopdf.exe"
)

const (
	ReportTypeExpressway         = "EXPRESSWAY"
	ReportTypePostEvaluation     = "POST_EVALUATION"
	ReportTypeConstruction       = "CONSTRUCTION"
	ReportTypeRural              = "RURAL"
	ReportTypeNationalProvincial = "NATIONAL_PROVINCIAL"
	ReportTypeMarket             = "MARKET"

	PyRespImagesKey      = "IMAGES"
	PyRespExtraImagesKey = "EXTRA_IMAGES"
	UserFont             = "FZHTJW--GB1-0"
)

var (
	ReportNameMap = map[string]string{
		ReportTypeExpressway:         "高速公路抽检路段公路技术状况监管分析报告",
		ReportTypePostEvaluation:     "工程后评价技术状况监管分析报告",
		ReportTypeConstruction:       "建设工程路段技术状况监管分析报告",
		ReportTypeRural:              "农村路抽检路段公路技术状况监管分析报告",
		ReportTypeNationalProvincial: "普通国省干线抽检路段公路技术状况监管分析报告",
		ReportTypeMarket:             "市场化路段抽检路段公路技术状况监管分析报告",
	}
	ReportDirs = []string{
		expresswayReportBaseDir,
		postEvaluationReportBaseDir,
		constructionReportBaseDir,
		ruralReportBaseDir,
		nationalProvinceReportBaseDir,
		marketReportBaseDir,
	}
)

type Road struct {
	gorm.Model `json:"-"`
	Name       string `json:"name" gorm:"unique"`
}

type ProvinceSetting struct {
	gorm.Model        `json:"-"`
	Year              int     `json:"year" gorm:"unique"`
	Expressway        float64 `json:"expressway"`
	NationalHighway   float64 `json:"nationalHighway"`
	ProvincialHighway float64 `json:"provincialHighway"`
	RuralRoad         float64 `json:"ruralRoad"`
}

type NationalSetting struct {
	gorm.Model           `json:"-"`
	Plan                 string  `json:"plan" gorm:"unique"`
	MQIExcellent         float64 `json:"mqiExcellent"`
	POIExcellent         float64 `json:"poiExcellent"`
	BridgeRate           float64 `json:"bridgeRate"`
	RecycleRate          float64 `json:"recycleRate"`
	NationalMQIEast      float64 `json:"nationalMqiEast"`
	NationalMQICentral   float64 `json:"nationalMqiCentral"`
	NationalMQIWest      float64 `json:"nationalMqiWest"`
	NationalPOIEast      float64 `json:"nationalPoiEast"`
	NationalPOICentral   float64 `json:"nationalPoiCentral"`
	NationalPOIWest      float64 `json:"nationalPoiWest"`
	ProvincialMQIEast    float64 `json:"provincialMqiEast"`
	ProvincialMQICentral float64 `json:"provincialMqiCentral"`
	ProvincialMQIWest    float64 `json:"provincialMqiWest"`
	ProvincialPOIEast    float64 `json:"provincialPoiEast"`
	ProvincialPOICentral float64 `json:"provincialPoiCentral"`
	ProvincialPOIWest    float64 `json:"provincialPoiWest"`
	RuralMQI             float64 `json:"ruralMqi"`
	MaintenanceRate      float64 `json:"maintenanceRate"`
}

type exportPDFReq struct {
	WmContent  string  `form:"wm_content"`
	WmColor    string  `form:"wm_color"`
	WmOpacity  float64 `form:"wm_opacity"`
	WmFontSize int     `form:"wm_font_size"`
	WmAngle    float64 `form:"wm_angle"`
}

var db *gorm.DB

func init() {
	conf.InitConf("./road.yaml")
	logger.InitLogger("road")

	sourceTtfFile := "fonts/方正黑体简体.TTF"

	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		logger.Logger.Fatalf("错误：无法获取用户配置目录 (os.UserConfigDir): %v", err)
	}

	targetDefaultPdfcpuFontDir := filepath.Join(userConfigDir, "pdfcpu", "fonts")

	logger.Logger.Infof("INFO: 源TTF文件路径: %s", sourceTtfFile)
	logger.Logger.Infof("INFO: 目标pdfcpu默认用户字体目录 (用于存放 .gob): %s", targetDefaultPdfcpuFontDir)

	if _, err = os.Stat(sourceTtfFile); os.IsNotExist(err) {
		logger.Logger.Fatalf("错误: 源TTF文件 %s 不存在。请确认路径。", sourceTtfFile)
	}

	if err = os.MkdirAll(targetDefaultPdfcpuFontDir, os.ModePerm); err != nil {
		logger.Logger.Fatalf("错误：无法创建目标pdfcpu默认字体目录 %s: %v", targetDefaultPdfcpuFontDir, err)
	}

	if err = font.InstallTrueTypeFont(targetDefaultPdfcpuFontDir, sourceTtfFile); err != nil {
		logger.Logger.Fatalf("错误：安装字体 %s 到 %s 失败: %v", sourceTtfFile, targetDefaultPdfcpuFontDir, err)
	}
	logger.Logger.Infof("SUCCESS: 字体 %s 应该已安装到 %s。FZHTJW--GB1-0.gob 应已在该目录生成。", sourceTtfFile, targetDefaultPdfcpuFontDir)
	logger.Logger.Infof("请检查目录 %s", targetDefaultPdfcpuFontDir)
}

func main() {
	var err error
	db, err = gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		logger.Logger.Errorf("failed to connect database: %v", err)
		return
	}

	err = db.AutoMigrate(&ProvinceSetting{}, &NationalSetting{}, &Road{})
	if err != nil {
		logger.Logger.Errorf("failed to AutoMigrate: %v", err)
		return
	}

	// 创建上传目录
	if err = os.MkdirAll(uploadDir, 0755); err != nil {
		logger.Logger.Errorf("创建上传目录失败: %v", err)
		return
	}
	if err = os.MkdirAll(pdfDir, 0755); err != nil {
		logger.Logger.Errorf("创建pdf目录失败: %v", err)
		return
	}
	// 创建报告和图片目录
	for _, dir := range ReportDirs {
		if err = os.MkdirAll(dir+"/images", 0755); err != nil {
			logger.Logger.Errorf("创建 %s 目录失败: %v", dir, err)
			return
		}
	}

	r := gin.Default()
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	r.Use(cors.New(config))

	pySuffix := conf.Conf.GetString("pySuffix")
	// 解压接口
	r.POST("/api/unzip", unzipHandler())

	// 计算接口
	r.POST("/api/calculate/docx", saveDocxHandler(pySuffix))
	r.POST("/api/calculate/md", saveMdHandler(pySuffix))

	report := r.Group("/api/reports")
	{
		report.GET("list", getReports)
		report.GET("/view/:filename", viewMarkdownHandler) //查看md
		//report.GET("/download/:filename", downloadWordHandler)   //下载docx
		report.GET("/download/:filename", exportReportHandler)   //下载pdf
		report.DELETE("/:filename", deleteReportHandler)         // 删除报告
		report.GET("/extraExport/:filename", extraExportHandler) // 特殊导出：年度指标达标情况
	}

	r.GET("/file", getFileHandler)

	setting := r.Group("/api/settings")
	{
		setting.POST("/province", saveProvinceSettings)
		setting.POST("/national", saveNationalSettings)
		setting.GET("/province/:year", getProvinceSetting)
		setting.GET("/national/:plan", getNationalSetting)
	}

	road := r.Group("/api/road")
	{
		road.GET("list", getRoads)
	}

	if err = r.Run(":12345"); err != nil {
		logger.Logger.Errorf("启动服务器失败: %v", err)
		return
	}
}

func saveDocxHandler(pySuffix string) func(c *gin.Context) {
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
		case ReportTypePostEvaluation:
			templateFile = "templates/养护工程JSON模板.docx"
		case ReportTypeConstruction:
			templateFile = "templates/建设工程JSON模板.docx"
		case ReportTypeRural:
			templateFile = "templates/农村公路JSON模板.docx"
		case ReportTypeNationalProvincial:
			templateFile = "templates/国省干线JSON模板.docx"
		case ReportTypeMarket:
			templateFile = "templates/市场化JSON模板.docx"
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
				case ReportTypePostEvaluation:
					srcDir = postEvaluationReportBaseDir + "/images"
				case ReportTypeConstruction:
					srcDir = constructionReportBaseDir + "/images"
				case ReportTypeRural:
					srcDir = ruralReportBaseDir + "/images"
				case ReportTypeNationalProvincial:
					srcDir = nationalProvinceReportBaseDir + "/images"
				case ReportTypeMarket:
					srcDir = marketReportBaseDir + "/images"
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

func saveMdHandler(pySuffix string) func(c *gin.Context) {
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
		case ReportTypePostEvaluation:
			templateFile = "templates/养护工程JSON模板.md"
		case ReportTypeConstruction:
			templateFile = "templates/建设工程JSON模板.md"
		case ReportTypeRural:
			templateFile = "templates/农村公路JSON模板.md"
		case ReportTypeNationalProvincial:
			templateFile = "templates/国省干线JSON模板.md"
		case ReportTypeMarket:
			templateFile = "templates/市场化JSON模板.md"
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

func calculate(pySuffix, reportType string, files []string, pqi, mileage float64) (map[string]any, error) {
	var program string
	var jsonResultFile string
	switch reportType {
	case ReportTypeExpressway:
		program = "expressway" + pySuffix
		jsonResultFile = expresswayReportBaseDir + "/result.json"
	case ReportTypePostEvaluation:
		program = "post_evaluation" + pySuffix
		jsonResultFile = postEvaluationReportBaseDir + "/result.json"
	case ReportTypeConstruction:
		program = "construction" + pySuffix
		jsonResultFile = constructionReportBaseDir + "/result.json"
	case ReportTypeRural:
		program = "rural" + pySuffix
		jsonResultFile = ruralReportBaseDir + "/result.json"
	case ReportTypeNationalProvincial:
		program = "national_provincial" + pySuffix
		jsonResultFile = nationalProvinceReportBaseDir + "/result.json"
	case ReportTypeMarket:
		program = "market" + pySuffix
		jsonResultFile = marketReportBaseDir + "/result.json"
	default:
		return nil, errors.New("不支持的报告类型")
	}

	logger.Logger.Infof("python exe: %s", program)
	//args := []string{
	//	"-files", strings.Join(files, " "),
	//	"-pqi", fmt.Sprintf("%.2f", pqi),
	//	"-d", fmt.Sprintf("%.2f", mileage),
	//}

	//cmd := exec.Command(program, args...)
	//logger.Logger.Infof("execute program: %v", cmd)
	//output, err := cmd.CombinedOutput()
	//if err != nil {
	//	logger.Logger.Errorf("Python执行失败 [%d]: %s\n输出: %s", cmd.ProcessState.ExitCode(), err, output)
	//	return nil, err
	//}
	var data map[string]any
	//if err = json.Unmarshal(output, &data); err != nil {
	//	logger.Logger.Errorf("解析结果失败: %v\n原始输出: %s", err, output)
	//	return nil, err
	//}

	js, err := os.ReadFile(jsonResultFile)
	if err != nil {
		logger.Logger.Errorf("读取 %s 失败: %v", jsonResultFile, err)
		return nil, err
	}
	if err = json.Unmarshal(js, &data); err != nil {
		logger.Logger.Errorf("解析结果失败: %v", err)
		return nil, err
	}
	return data, nil
}

func unzipHandler() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFileSize)

		file, err := c.FormFile("file")
		if err != nil {
			logger.Logger.Errorf("文件上传失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "文件上传失败"})
			return
		}

		tempDir, err := os.MkdirTemp(uploadDir, "unzip-*")
		if err != nil {
			logger.Logger.Errorf("创建临时目录失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时目录失败"})
			return
		}

		zipPath := filepath.Join(tempDir, file.Filename)
		if err = c.SaveUploadedFile(file, zipPath); err != nil {
			logger.Logger.Errorf("文件保存失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "文件保存失败"})
			return
		}

		files, err := unzip(zipPath, tempDir)
		if err != nil {
			logger.Logger.Errorf("文件解压失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "文件解压失败"})
			return
		}
		//for _, fileName := range files {
		//	xlsx, err := excelize.OpenFile(fileName)
		//	if err != nil {
		//		logger.Logger.Errorf("打开xlsx文件 %s 失败: %v", fileName, err)
		//		continue
		//	}
		//
		//	rows, err := xlsx.GetRows(xlsx.GetSheetName(0))
		//	if err != nil {
		//		logger.Logger.Errorf("获取 %s 的 sheet[%s] 失败: %v", fileName, xlsx.GetSheetName(0), err)
		//		continue
		//	}
		//	roadNameIdx := -1
		//	for i := range rows {
		//		if rows[i][0] == "路线编码" {
		//			if i+1 >= len(rows) || i+2 >= len(rows) {
		//				continue
		//			}
		//			roadNameIdx = i + 1
		//			break
		//		}
		//	}
		//
		//	if roadNameIdx >= 0 {
		//		roadName := rows[roadNameIdx][0]
		//		if roadName == "" {
		//			roadName = rows[roadNameIdx+1][0]
		//		}
		//		if err = db.Clauses(clause.OnConflict{DoNothing: true}).Create(&Road{Name: roadName}).Error; err != nil {
		//			logger.Logger.Errorf("%s 的路线名称 %s 写入数据库失败: %v", fileName, roadName, err)
		//			continue
		//		}
		//	}
		//}
		c.JSON(http.StatusOK, gin.H{"files": files})
	}
}

func unzip(src, dest string) ([]string, error) {
	var filenames []string
	r, err := zip.OpenReader(src)
	if err != nil {
		logger.Logger.Errorf("打开ZIP文件失败: %v", err)
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		name := f.Name
		if f.Flags&0x800 == 0 {
			decodedName, err := decodeFileName(name)
			if err != nil {
				logger.Logger.Errorf("GBK解码失败: %v", err)
			} else {
				name = decodedName
			}
		}

		fpath := filepath.Join(dest, name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			errMsg := fmt.Sprintf("非法文件路径: %s", name)
			log.Println(errMsg)
			return nil, fmt.Errorf(errMsg)
		}

		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(fpath, 0755); err != nil {
				logger.Logger.Errorf("创建目录失败: %v", err)
				return nil, err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			logger.Logger.Errorf("创建父目录失败: %v", err)
			return nil, err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			logger.Logger.Errorf("创建文件失败: %v", err)
			return nil, err
		}

		rc, err := f.Open()
		if err != nil {
			logger.Logger.Errorf("打开ZIP条目失败: %v", err)
			outFile.Close()
			return nil, err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			logger.Logger.Errorf("文件写入失败: %v", err)
			return nil, err
		}

		filenames = append(filenames, fpath)
	}
	return filenames, nil
}

func decodeFileName(name string) (string, error) {
	// 先尝试UTF-8
	if utf8.ValidString(name) {
		return name, nil
	}

	// 尝试GBK
	gbkName, err := decodeGBK(name)
	if err == nil && gbkName != name {
		return gbkName, nil
	}

	// 尝试其他常见中文编码如GB18030
	decoder := simplifiedchinese.GB18030.NewDecoder()
	gb18030Name, _, err := transform.String(decoder, name)
	if err == nil && gb18030Name != name {
		return gb18030Name, nil
	}

	return name, fmt.Errorf("无法解码文件名")
}

func decodeGBK(s string) (string, error) {
	reader := transform.NewReader(strings.NewReader(s), simplifiedchinese.GBK.NewDecoder())
	buf, err := io.ReadAll(reader)
	if err != nil {
		logger.Logger.Errorf("GBK解码失败: %v", err)
		return "", err
	}
	return string(buf), nil
}

func saveProvinceSettings(c *gin.Context) {
	var setting ProvinceSetting
	if err := c.ShouldBindJSON(&setting); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	db.Save(&setting)
	c.JSON(200, gin.H{"message": "省厅指标保存成功"})
}

func saveNationalSettings(c *gin.Context) {
	var setting NationalSetting
	if err := c.ShouldBindJSON(&setting); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	db.Save(&setting)
	c.JSON(200, gin.H{"message": "交通部指标保存成功"})
}

func getProvinceSetting(c *gin.Context) {
	yearStr := c.Param("year")
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的年份参数"})
		return
	}

	var setting ProvinceSetting
	if err = db.Where("year = ?", year).First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("未找到%d年的省厅配置", year)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
		return
	}

	c.JSON(http.StatusOK, setting)
}

func getNationalSetting(c *gin.Context) {
	plan := c.Param("plan")

	var setting NationalSetting
	if err := db.Where("plan = ?", plan).First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("未找到计划'%s'的交通部配置", plan)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
		return
	}

	c.JSON(http.StatusOK, setting)
}

func getRoads(c *gin.Context) {
	var roads []Road
	if err := db.Find(&roads).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取路线名称失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, roads)
}

func getFileHandler(c *gin.Context) {
	filename := c.Query("name")
	p, _ := url.QueryUnescape(filename)
	filename = filepath.Join(reportsBaseDir, p)
	c.Header("Content-Disposition", "inline")
	c.File(filename)
}

func getReports(c *gin.Context) {
	skipDir := filepath.Join("images")

	reports := make([]string, 0)
	err := filepath.Walk(reportsBaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && strings.Contains(path, skipDir) {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.Contains(path, ".doc") && !strings.Contains(path, "_extra") {
			reports = append(reports, filepath.Base(path))
		}
		return nil
	})
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

func viewMarkdownHandler(c *gin.Context) {
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

func downloadWordHandler(c *gin.Context) {
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

func deleteReportHandler(c *gin.Context) {
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

func extraExportHandler(c *gin.Context) {
	originalFilename := c.Param("filename") // 例如: "普通国省干线抽检路段公路技术状况监管分析报告_1745680397.docx"
	if originalFilename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件名参数"})
		return
	}

	// --- 1. 解析原始文件名以获取目录名、基础名、时间戳和扩展名 ---
	lastDotIndex := strings.LastIndex(originalFilename, ".")
	if lastDotIndex == -1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件名格式 (缺少扩展名)"})
		return
	}
	extension := originalFilename[lastDotIndex:]     // .docx
	directoryName := originalFilename[:lastDotIndex] // 目录名，例如: 普通国省干线...报告_1745680397
	baseWithTimestamp := directoryName               // 和目录名相同

	lastUnderscoreIndex := strings.LastIndex(baseWithTimestamp, "_")
	if lastUnderscoreIndex == -1 || lastUnderscoreIndex == len(baseWithTimestamp)-1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件名格式 (无法从文件名解析基础名称和时间戳部分)"})
		return
	}
	baseReportNamePart := baseWithTimestamp[:lastUnderscoreIndex] // 例如: 普通国省干线...报告
	timestampPart := baseWithTimestamp[lastUnderscoreIndex+1:]    // 例如: 1745680397

	// --- 2. 根据基础报告名称部分确定报告类型 ---
	var reportType string
	var foundType bool
	// 优先尝试精确匹配基础报告名部分
	for rtConst, namePrefix := range ReportNameMap {
		if baseReportNamePart == namePrefix {
			reportType = rtConst
			foundType = true
			break
		}
	}
	// 如果精确匹配失败（可能名称包含额外信息？），尝试用目录名前缀匹配
	if !foundType {
		for rtConst, namePrefix := range ReportNameMap {
			if strings.HasPrefix(directoryName, namePrefix) {
				reportType = rtConst
				foundType = true
				break
			}
		}
	}

	if !foundType {
		errMsg := fmt.Sprintf("无法从文件名 '%s' 识别出报告类型", originalFilename)
		logger.Logger.Warnf(errMsg)
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	// --- 3. 检查报告类型是否允许此操作 ---
	allowedTypes := map[string]bool{
		ReportTypeExpressway:         true, // 高速公路
		ReportTypeRural:              true, // 农村公路
		ReportTypeNationalProvincial: true, // 普通国省干线
	}
	if !allowedTypes[reportType] {
		errMsg := fmt.Sprintf("报告类型 '%s' (%s) 不支持此额外导出功能", ReportNameMap[reportType], reportType)
		logger.Logger.Warnf("尝试为不支持的类型执行额外导出: %s (文件名: %s)", reportType, originalFilename)
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	// --- 4. 构建目标 "额外" 文件的名称和完整路径 ---
	// 目标文件名: 基础报告名 + "_extra_" + 时间戳 + 扩展名
	extraFilename := fmt.Sprintf("%s_extra_%s%s", baseReportNamePart, timestampPart, extension)
	// 目标文件完整路径: reports基础目录 / 目录名 / 额外文件名
	fullPathToExtraFile := filepath.Join(reportsBaseDir, directoryName, extraFilename)

	// --- 5. 检查目标 "额外" 文件是否存在 ---
	if _, err := os.Stat(fullPathToExtraFile); os.IsNotExist(err) {
		logger.Logger.Errorf("请求的额外导出文件 %s 不存在: %v", fullPathToExtraFile, err)
		// 返回 404 Not Found，表示请求的特定资源（那个 _extra 文件）未找到
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("对应的额外导出文件 (%s) 未找到", extraFilename)})
		return
	} else if err != nil {
		// 处理其他检查文件时可能发生的错误 (例如权限问题)
		logger.Logger.Errorf("检查额外导出文件 %s 时出错: %v", fullPathToExtraFile, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "检查文件状态时发生服务器内部错误"})
		return
	}

	// --- 6. 设置下载响应头 ---
	encodedFilename := url.QueryEscape(extraFilename) // 对文件名进行URL编码
	disposition := fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", extraFilename, encodedFilename)
	c.Header("Content-Disposition", disposition)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document") // Word文档的MIME类型

	// --- 7. 发送文件 ---
	c.File(fullPathToExtraFile)
}

func extractTimestamp(filename string) int64 {
	lastUnderscore := strings.LastIndex(filename, "_")
	lastDot := strings.LastIndex(filename, ".")

	if lastUnderscore == -1 || lastDot == -1 || lastUnderscore >= lastDot-1 {
		return 0
	}

	timestampStr := filename[lastUnderscore+1 : lastDot]
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return 0
	}
	return timestamp
}

func exportReportHandler(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件名不能为空"})
		return
	}

	var req exportPDFReq
	err := c.ShouldBindQuery(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "水印参数有误"})
		return
	}

	baseName := filename[:strings.LastIndex(filename, ".")]
	if !strings.HasPrefix(filename, "md") {
		filename = baseName + ".md"
	}
	fullFilePath := filepath.Join(reportsBaseDir, baseName, filename)

	absReportsBaseDir, err := filepath.Abs(reportsBaseDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器配置错误 (无法确定报告目录的绝对路径)"})
		return
	}
	absFullFilePath, err := filepath.Abs(fullFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器处理文件路径错误"})
		return
	}
	if !strings.HasPrefix(absFullFilePath, absReportsBaseDir) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件名"})
		return
	}

	fileInfo, err := os.Stat(fullFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("报告文件 '%s' 未找到", filename)})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("无法访问报告文件 '%s'", filename)})
		}
		return
	}
	if !fileInfo.Mode().IsRegular() {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("路径 '%s' 不是一个有效的文件", filename)})
		return
	}

	mdContent, err := os.ReadFile(fullFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取报告文件 '%s' 失败", filename)})
		return
	}

	htmlContentBytes := markdown.ToHTML(mdContent, nil, nil)
	htmlContentString := string(htmlContentBytes)

	// 保留之前的字体大小样式，但移除水印相关的 CSS
	htmlHeadContent := `
<head>
    <meta charset="utf-8">
    <style>
        body {
            font-family: "Microsoft YaHei", sans-serif;
            font-size: 20pt; /* 用户找到合适的字号 */
            line-height: 1.5;
            /* 移除 position: relative; 如果body不作为其他元素的定位上下文 */
        }
        h1 { font-size: 36pt; margin-top: 20pt; margin-bottom: 10pt; }
        h2 { font-size: 32pt; margin-top: 18pt; margin-bottom: 8pt; }
        h3 { font-size: 28pt; margin-top: 16pt; margin-bottom: 6pt; }
        p { font-size: 20pt; margin-top: 6pt; margin-bottom: 6pt; }
        table {
             border-collapse: collapse;
             width: 100%;
             margin-top: 10pt;
             margin-bottom: 10pt;
             font-size: 9pt;
        }
        th, td {
            border: 1px solid #ddd;
            padding: 8pt;
            text-align: left;
        }
        th { background-color: #f2f2f2; }

        /* 水印样式已移除，改用 pdfcpu 处理 */

        /* 其他 Markdown 元素样式，如 li, pre, code, blockquote 等，可按需添加 */
    </style>
</head>
`

	htmlWithHead := ""
	htmlTagIndex := strings.Index(htmlContentString, "<html")
	if htmlTagIndex != -1 {
		htmlTagEndIndex := strings.Index(htmlContentString[htmlTagIndex:], ">")
		if htmlTagEndIndex != -1 {
			insertIndex := htmlTagIndex + htmlTagEndIndex + 1
			htmlWithHead = htmlContentString[:insertIndex] + "\n" + htmlHeadContent + "\n" + htmlContentString[insertIndex:]
		} else {
			htmlWithHead = htmlHeadContent + "\n" + htmlContentString
		}
	} else {
		htmlWithHead = htmlHeadContent + "\n" + htmlContentString
	}

	htmlContentToPdf := []byte(htmlWithHead)

	// 调用 wkhtmltopdf 工具生成 PDF 字节流
	// 移除 --enable-local-file-access 选项，因为不再需要加载本地水印图片
	cmd := exec.Command(wkhtmltopdfPath, "-", "-")
	cmd.Stdin = bytes.NewReader(htmlContentToPdf)

	var pdfBytesBuffer bytes.Buffer // 用于存放 wkhtmltopdf 生成的原始 PDF 字节流
	var stderr bytes.Buffer
	cmd.Stdout = &pdfBytesBuffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		logger.Logger.Errorf("执行 wkhtmltopdf 失败: %v Stderr: %s", err, stderr.String())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成PDF失败"})
		return
	}
	logger.Logger.Infof("stderr: %s", stderr.String())

	// 设置 HTTP 响应头
	c.Header("Content-Type", "application/pdf")

	downloadFilename := strings.TrimSuffix(filename, ".md") + ".pdf"
	encodedFilename := url.QueryEscape(downloadFilename)
	contentDisposition := fmt.Sprintf("attachment; filename*=utf-8''%s", encodedFilename)
	c.Header("Content-Disposition", contentDisposition)

	if req.WmContent != "" {
		tempPdfPath := filepath.Join(pdfDir, baseName+".pdf")
		err = os.WriteFile(tempPdfPath, pdfBytesBuffer.Bytes(), 0644)
		if err != nil {
			logger.Logger.Errorf("将 PDF 内容写入临时文件 %s 失败: %v", tempPdfPath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "写入临时文件失败"})
			return
		}

		rsForWatermarking, err := os.Open(tempPdfPath)
		if err != nil {
			logger.Logger.Errorf("无法重新打开临时文件 %s 进行水印操作: %v", tempPdfPath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误 (准备水印数据失败)"})
			return
		}
		defer rsForWatermarking.Close()

		// --- 使用 pdfcpu 添加水印 ---
		// --- 水印配置数据 ---
		cnf := model.NewDefaultConfiguration()
		cnf.Unit = types.POINTS

		baseDesc := fmt.Sprintf("points:%d, rotation:%.2f, opacity:%.2f, fillcolor:%s, font:%s", req.WmFontSize, req.WmAngle, req.WmOpacity/100, req.WmColor, UserFont)
		positions := []string{
			"pos:tl, off:75 -150", "pos:tr, off:-75 -150",
			"pos:bl, off:75 300", "pos:br, off:-75 300",
		}
		watermarksForOnePage := make([]*model.Watermark, 0, len(positions))
		for _, posStr := range positions {
			fullDesc := fmt.Sprintf("%s, %s", baseDesc, posStr)
			wm, err := api.TextWatermark(req.WmContent, fullDesc, false, false, cnf.Unit)
			if err != nil {
				logger.Logger.Errorf("创建水印失败 (描述: '%s'): %v", fullDesc, err)
				continue
			}
			watermarksForOnePage = append(watermarksForOnePage, wm)
		}

		ctx, err := api.ReadContextFile(tempPdfPath)
		if err != nil {
			logger.Logger.Errorf("读取PDF信息失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取PDF信息失败"})
			return
		}
		pageCount := ctx.PageCount
		if pageCount == 0 {
			logger.Logger.Errorf("PDF文件没有页面")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "PDF文件没有页面"})
			return
		}
		watermarkMap := make(map[int][]*model.Watermark)
		for i := 1; i <= pageCount; i++ {
			watermarkMap[i] = watermarksForOnePage
		}

		watermarkedPdfBuffer := new(bytes.Buffer)
		err = api.AddWatermarksSliceMap(rsForWatermarking, watermarkedPdfBuffer, watermarkMap, cnf)
		if err != nil {
			logger.Logger.Errorf("添加水印到字节流失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "PDF添加水印失败"})
			return
		}

		// **使用添加水印后 buffer 的大小**
		c.Header("Content-Length", fmt.Sprintf("%d", watermarkedPdfBuffer.Len()))

		// **将添加水印后 buffer 的内容写入 HTTP 响应体**
		_, err = io.Copy(c.Writer, watermarkedPdfBuffer)
		if err != nil {
			logger.Logger.Errorf("将 PDF 响应写入客户端失败: %v\n", err)
		}
	} else {
		// --- 不需要添加水印，直接返回原始 PDF ---
		// **使用 wkhtmltopdf 生成的原始 PDF buffer 的大小**
		c.Header("Content-Length", fmt.Sprintf("%d", pdfBytesBuffer.Len()))

		// **将原始 PDF buffer 的内容写入 HTTP 响应体**
		_, err = io.Copy(c.Writer, &pdfBytesBuffer)
		if err != nil {
			logger.Logger.Errorf("将原始 PDF 响应写入客户端失败: %v\n", err)
		}
	}
}
