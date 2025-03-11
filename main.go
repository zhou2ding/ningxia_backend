package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/nguyenthenguyen/docx"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
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
)

const (
	uploadDir   = "./tmp/uploads"
	maxFileSize = 1024 * 1024 * 500 // 500MB
)

type ProvinceSetting struct {
	gorm.Model
	Year              int     `json:"year" gorm:"uniqueIndex"`
	Expressway        float64 `json:"expressway"`
	NationalHighway   float64 `json:"nationalHighway"`
	ProvincialHighway float64 `json:"provincialHighway"`
	RuralRoad         float64 `json:"ruralRoad"`
}

type NationalSetting struct {
	gorm.Model
	Plan                 string  `json:"plan" gorm:"uniqueIndex"`
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
	dsn := "root:5023152@tcp(36.133.97.26:26033)/road?charset=utf8mb4&parseTime=True&loc=Local"
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&ProvinceSetting{}, &NationalSetting{})

	// 创建上传目录
	if err = os.MkdirAll(uploadDir, 0755); err != nil {
		logger.Logger.Errorf("创建上传目录失败: %v", err)
		return
	}

	r := gin.Default()
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://36.133.97.26:16044"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	config.AllowCredentials = true

	r.Use(cors.New(config))

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

		c.JSON(http.StatusOK, gin.H{"files": files})
	})

	// 计算接口
	r.POST("/api/calculate", func(c *gin.Context) {
		var req struct {
			Files []string `json:"files"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Logger.Errorf("无效请求: %v", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		quotedFiles := make([]string, len(req.Files))
		for i, f := range req.Files {
			quotedFiles[i] = strconv.Quote(f) // 处理空格和特殊字符
		}

		cmd := exec.Command("python", append([]string{"process.py"}, quotedFiles...)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Logger.Errorf("Python执行失败 [%d]: %s\n输出: %s", cmd.ProcessState.ExitCode(), err, output)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("计算失败: %s", output)})
			return
		}
		var data map[string]string
		if err = json.Unmarshal(output, &data); err != nil {
			logger.Logger.Errorf("解析结果失败: %v\n原始输出: %s", err, output)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse results"})
			return
		}

		doc, err := docx.ReadDocxFile("template.docx")
		if err != nil {
			logger.Logger.Errorf("读取模板失败: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Template error"})
			return
		}
		defer doc.Close()

		docxFile := doc.Editable()
		content := docxFile.GetContent()
		for key, value := range data {
			logger.Logger.Infof("will replace %v to %s", key, value)
			content = strings.ReplaceAll(content, key, value)
		}
		docxFile.SetContent(content)

		for i := 1; i <= docxFile.ImagesLen(); i++ {
			logger.Logger.Infof("will replace %v to %s", "word/media/image"+strconv.Itoa(i)+".jpg", "./"+strconv.Itoa(i)+".jpg")

			err = docxFile.ReplaceImage("word/media/image"+strconv.Itoa(i)+".jpg", "./"+strconv.Itoa(i)+".jpg")
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

	api := r.Group("/api/settings")
	{
		api.POST("/province", saveProvinceSettings)
		api.POST("/national", saveNationalSettings)
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
			decodedName, err := decodeGBK(name)
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
