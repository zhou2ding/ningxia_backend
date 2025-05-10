package handler

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"ningxia_backend/pkg/logger"
	"os"
	"path/filepath"
)

func UnzipHandler() func(c *gin.Context) {
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
