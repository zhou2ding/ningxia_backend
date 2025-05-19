package handler

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"ningxia_backend/pkg/logger"
	"os"
	"path/filepath"
	"strings"
)

func UnzipHandler() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFileSize)
		// 为当前请求创建一个唯一的临时目录
		requestTempDir, err := os.MkdirTemp(uploadDir, "req-*-files")
		if err != nil {
			logger.Logger.Errorf("创建请求临时目录失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时目录失败"})
			return
		}
		var allProcessedFilePaths []string

		// 前端可能发送的ZIP和Excel文件的表单字段名
		zipFieldNames := []string{"threeDimensionalDataZip", "cicsDataZip", "previousYearDiseaseZip"}
		excelFieldNames := []string{"managementDetailFile", "unitLevelDetailFile", "roadConditionFile", "firstInspectionExcel", "secondInspectionExcel", "diseaseDataExcel"}

		// 逐个处理ZIP文件
		for _, fieldName := range zipFieldNames {
			file, err := c.FormFile(fieldName)
			if err != nil {
				if errors.Is(err, http.ErrMissingFile) {
					continue
				}
				logger.Logger.Errorf("获取ZIP文件 '%s' 失败: %v", fieldName, err)
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("处理ZIP文件 '%s' 失败: %v", fieldName, err)})
				return
			}

			// 将上传的ZIP文件保存到请求临时目录中
			tempZipPath := filepath.Join(requestTempDir, file.Filename)
			if err = c.SaveUploadedFile(file, tempZipPath); err != nil {
				logger.Logger.Errorf("保存上传的ZIP文件 '%s' 到 '%s' 失败: %v", file.Filename, tempZipPath, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存文件 '%s' 失败", file.Filename)})
				return
			}

			// 为解压后的文件创建一个子目录
			// 例如：data.zip -> data_extracted/
			baseName := strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename))
			extractDir := filepath.Join(requestTempDir, baseName+"_extracted")
			if err = os.MkdirAll(extractDir, 0755); err != nil {
				logger.Logger.Errorf("为ZIP '%s' 创建解压目录 '%s' 失败: %v", file.Filename, extractDir, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("为文件 '%s' 准备解压环境失败", file.Filename)})
				return
			}

			// 解压ZIP文件
			unzippedFiles, err := unzip(tempZipPath, extractDir)
			if err != nil {
				logger.Logger.Errorf("解压ZIP文件 '%s' 失败: %v", file.Filename, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("解压文件 '%s' 失败", file.Filename)})
				return
			}
			allProcessedFilePaths = append(allProcessedFilePaths, unzippedFiles...)

			// 解压后删除临时的ZIP文件
			if err = os.Remove(tempZipPath); err != nil {
				logger.Logger.Errorf("删除临时ZIP文件 '%s' 失败: %v", tempZipPath, err)
			}
		}

		// 逐个处理Excel文件
		for _, fieldName := range excelFieldNames {
			file, err := c.FormFile(fieldName)
			if err != nil {
				if errors.Is(err, http.ErrMissingFile) {
					// 文件是可选的，如果不存在则跳过
					logger.Logger.Infof("可选的Excel文件 '%s' 未上传", fieldName)
					continue
				}
				logger.Logger.Errorf("获取Excel文件 '%s' 失败: %v", fieldName, err)
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("处理Excel文件 '%s' 失败: %v", fieldName, err)})
				return
			}

			// 将上传的Excel文件直接保存到请求临时目录中
			destExcelPath := filepath.Join(requestTempDir, file.Filename)
			if err = c.SaveUploadedFile(file, destExcelPath); err != nil {
				logger.Logger.Errorf("保存上传的Excel文件 '%s' 到 '%s' 失败: %v", file.Filename, destExcelPath, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存文件 '%s' 失败", file.Filename)})
				return
			}
			allProcessedFilePaths = append(allProcessedFilePaths, destExcelPath)
		}
		c.JSON(http.StatusOK, gin.H{"files": allProcessedFilePaths})
	}
}
