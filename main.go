package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/nguyenthenguyen/docx"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"io"
	"log"
	"net/http"
	"ningxia_backend/pkg/conf"
	"ningxia_backend/pkg/logger"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	uploadDir   = "./tmp/uploads"
	maxFileSize = 1024 * 1024 * 500 // 500MB
	dbFile      = "./road.db"
)

const (
	ReportTypeExpressway         = "EXPRESSWAY"
	ReportTypePostEvaluation     = "POST_EVALUATION"
	ReportTypeConstruction       = "CONSTRUCTION"
	ReportTypeRural              = "RURAL"
	ReportTypeNationalProvincial = "NATIONAL_PROVINCIAL"
	ReportTypeMarket             = "MARKET"
)

type Road struct {
	gorm.Model `json:"-"`
	Name       string `json:"name" gorm:"unique"`
}

type ProvinceSetting struct {
	gorm.Model
	Year              int     `json:"year" gorm:"unique"`
	Expressway        float64 `json:"expressway"`
	NationalHighway   float64 `json:"nationalHighway"`
	ProvincialHighway float64 `json:"provincialHighway"`
	RuralRoad         float64 `json:"ruralRoad"`
}

type NationalSetting struct {
	gorm.Model
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

var db *gorm.DB

func main() {
	conf.InitConf("./road.yaml")
	logger.InitLogger("road")

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

	r := gin.Default()
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	r.Use(cors.New(config))

	pySuffix := conf.Conf.GetString("pySuffix")
	// 解压接口
	r.POST("/api/unzip", func(c *gin.Context) {
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
		for _, fileName := range files {
			xlsx, err := excelize.OpenFile(fileName)
			if err != nil {
				logger.Logger.Errorf("打开xlsx文件 %s 失败: %v", fileName, err)
				continue
			}

			rows, err := xlsx.GetRows(xlsx.GetSheetName(0))
			if err != nil {
				logger.Logger.Errorf("获取 %s 的 sheet[%s] 失败: %v", fileName, xlsx.GetSheetName(0), err)
				continue
			}
			roadNameIdx := -1
			for i := range rows {
				if rows[i][0] == "路线编码" {
					if i+1 >= len(rows) || i+2 >= len(rows) {
						continue
					}
					roadNameIdx = i + 1
					break
				}
			}

			if roadNameIdx >= 0 {
				roadName := rows[roadNameIdx][0]
				if roadName == "" {
					roadName = rows[roadNameIdx+1][0]
				}
				if err = db.Clauses(clause.OnConflict{DoNothing: true}).Create(&Road{Name: roadName}).Error; err != nil {
					logger.Logger.Errorf("%s 的路线名称 %s 写入数据库失败: %v", fileName, roadName, err)
					continue
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"files": files})
	})

	// 计算接口
	r.POST("/api/calculate", func(c *gin.Context) {
		var req struct {
			Files      []string `json:"files"`
			ReportType string   `json:"reportType"`
			Mileage    float64  `json:"mileage"`
			PQI        float64  `json:"pqi"`
		}
		if err = c.ShouldBindJSON(&req); err != nil {
			logger.Logger.Errorf("无效请求: %v", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		var program string
		switch req.ReportType {
		case ReportTypeExpressway:
			program = "pys/dist/expressway" + pySuffix
		case ReportTypePostEvaluation:
			program = "pys/dist/post_evaluation" + pySuffix
		case ReportTypeConstruction:
			program = "pys/dist/construction" + pySuffix
		case ReportTypeRural:
			program = "pys/dist/rural" + pySuffix
		case ReportTypeNationalProvincial:
			program = "pys/dist/national_provincial" + pySuffix
		case ReportTypeMarket:
			program = "pys/dist/market" + pySuffix
		default:
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "不支持的报告类型"})
			return
		}

		args := []string{
			"-files", strings.Join(req.Files, " "),
			"-pqi", fmt.Sprintf("%.2f", req.PQI),
			"-d", fmt.Sprintf("%.2f", req.Mileage),
		}

		cmd := exec.Command(program, args...)
		logger.Logger.Infof("execute program: %v", cmd)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Logger.Errorf("Python执行失败 [%d]: %s\n输出: %s", cmd.ProcessState.ExitCode(), err, output)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("计算失败: %s", err)})
			return
		}
		var data map[string]any
		if err = json.Unmarshal(output, &data); err != nil {
			logger.Logger.Errorf("解析结果失败: %v\n原始输出: %s", err, output)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse results"})
			return
		}

		doc, err := docx.ReadDocxFile("templates/副本表1.docx")
		if err != nil {
			logger.Logger.Errorf("读取模板失败: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Template error"})
			return
		}
		defer doc.Close()

		docxFile := doc.Editable()
		content := docxFile.GetContent()
		for key, value := range data {
			valStr := fmt.Sprintf("%v", value)
			if valStr == "" {
				content = strings.ReplaceAll(content, key, " ")
			} else {
				content = strings.ReplaceAll(content, key, valStr)
			}
		}
		docxFile.SetContent(content)

		for i := 1; i <= docxFile.ImagesLen(); i++ {
			err = docxFile.ReplaceImage("word/media/image"+strconv.Itoa(i)+".jpg", "./templates/"+strconv.Itoa(i)+".jpg")
			if err != nil {
				logger.Logger.Errorf("替换图片失败: %v", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "替换图片失败"})
				return
			}
		}

		tmpFile, err := os.CreateTemp("", "report-*.docx")
		if err != nil {
			logger.Logger.Errorf("创建临时文件失败: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "无法创建临时文件"})
			return
		}
		defer os.Remove(tmpFile.Name())

		if err = docxFile.WriteToFile(tmpFile.Name()); err != nil {
			logger.Logger.Errorf("文档生成失败: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "文档生成失败"})
			return
		}

		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		c.Header("Content-Disposition", "attachment; filename=report.docx")
		http.ServeFile(c.Writer, c.Request, tmpFile.Name())
	})

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
