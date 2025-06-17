package handler

import (
	"archive/zip"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

func ExportTablesHandler(c *gin.Context) {
	// 1. 从URL参数中获取目录名
	dirname := c.Param("dirname")
	if dirname == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求缺少目录名称"})
		return
	}
	// 2. 构建报告所在目录的完整路径
	reportDir := filepath.Join(reportsBaseDir, dirname)
	if _, err := os.Stat(reportDir); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("报告目录 '%s' 不存在", dirname)})
		return
	}

	// 3. 在目录中查找存在的目标Excel文件
	var foundFiles []string
	for _, filename := range targetExcelFiles {
		filePath := filepath.Join(reportDir, filename)
		if _, err := os.Stat(filePath); err == nil {
			// 文件存在，将其完整路径加入列表
			foundFiles = append(foundFiles, filePath)
		}
	}

	// 4. 根据找到的文件数量执行相应操作
	switch len(foundFiles) {
	case 0:
		// 如果没有找到任何一个目标文件
		c.JSON(http.StatusNotFound, gin.H{"error": "此报告中未找到任何匹配的指标明细表"})
		return
	case 1:
		// 如果只找到一个文件，则直接发送该文件
		singleFilePath := foundFiles[0]
		fileName := filepath.Base(singleFilePath)

		// 设置HTTP头，使浏览器能正确识别并下载文件
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(fileName))
		c.File(singleFilePath)
	default:
		// 如果找到多个文件，则将它们打包成一个ZIP文件
		zipFileName := fmt.Sprintf("%s.zip", dirname)
		zipFilePath := filepath.Join(os.TempDir(), zipFileName) // 在系统临时目录中创建zip文件

		// 创建临时的zip文件
		zipFile, err := os.Create(zipFilePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时zip文件失败"})
			return
		}
		// 使用defer确保函数执行完毕后，关闭并删除这个临时zip文件
		defer zipFile.Close()
		defer os.Remove(zipFilePath)

		zipWriter := zip.NewWriter(zipFile)

		// 遍历找到的文件，并将它们逐一写入zip压缩包
		for _, fileToZip := range foundFiles {
			// 在zip压缩包内创建同名文件
			fileWriter, err := zipWriter.Create(filepath.Base(fileToZip))
			if err != nil {
				continue // 如果出错，跳过这个文件
			}

			// 打开源文件
			srcFile, err := os.Open(fileToZip)
			if err != nil {
				continue // 如果出错，跳过这个文件
			}

			// 将源文件的内容拷贝到zip的写入器中
			io.Copy(fileWriter, srcFile)
			srcFile.Close() // 关闭源文件句柄
		}

		// 必须关闭zip.Writer来确保所有数据都被写入到底层的文件中
		zipWriter.Close()

		// 发送生成的zip文件给前端
		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(zipFileName))
		c.File(zipFilePath)
	}
}
