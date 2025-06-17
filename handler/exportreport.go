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

func ExportReportHandler(c *gin.Context) {
	filename := c.Param("filename")
	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件名不能为空"})
		return
	}

	var req exportPDFReq
	err := c.ShouldBindQuery(&req)
	if err != nil {
		logger.Logger.Errorf("水印参数绑定失败: %v, query: %s", err, c.Request.URL.RawQuery)
		c.JSON(http.StatusBadRequest, gin.H{"error": "水印参数有误"})
		return
	}

	baseName := filename
	if strings.Contains(filename, ".") {
		baseName = filename[:strings.LastIndex(filename, ".")]
	}

	mdFilename := baseName + ".md"
	fullFilePath := filepath.Join(reportsBaseDir, baseName, mdFilename)

	absReportsBaseDir, err := filepath.Abs(reportsBaseDir)
	if err != nil {
		logger.Logger.Errorf("无法确定报告目录的绝对路径: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器配置错误 (无法确定报告目录的绝对路径)"})
		return
	}

	absFullFilePath, err := filepath.Abs(fullFilePath)
	if err != nil {
		logger.Logger.Errorf("服务器处理文件路径错误 for %s: %v", fullFilePath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器处理文件路径错误"})
		return
	}

	if !strings.HasPrefix(absFullFilePath, absReportsBaseDir) {
		logger.Logger.Warnf("无效的文件名请求 (路径遍历尝试?): %s (resolved: %s) vs base %s", filename, absFullFilePath, absReportsBaseDir)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文件名"})
		return
	}

	fileInfo, err := os.Stat(fullFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("报告文件 '%s' 未找到 at %s", mdFilename, fullFilePath)})
		} else {
			logger.Logger.Errorf("无法访问报告文件 '%s' at %s: %v", mdFilename, fullFilePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("无法访问报告文件 '%s'", mdFilename)})
		}
		return
	}

	if !fileInfo.Mode().IsRegular() {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("路径 '%s' 不是一个有效的文件", mdFilename)})
		return
	}

	mdContent, err := os.ReadFile(fullFilePath)
	if err != nil {
		logger.Logger.Errorf("读取报告文件 '%s' at %s 失败: %v", mdFilename, fullFilePath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取报告文件 '%s' 失败", mdFilename)})
		return
	}

	// 将 Markdown 内容按“指标明细表”分割
	splitMarker := "指标明细表"
	mdLines := strings.Split(string(mdContent), "\n")
	var beforeMarker, afterMarker string
	found := false
	for _, line := range mdLines {
		if strings.Contains(line, splitMarker) {
			found = true
		}
		if !found {
			beforeMarker += line + "\n"
		} else {
			afterMarker += line + "\n"
		}
	}

	// 生成两部分的 HTML
	htmlBefore := generateHTML(beforeMarker)
	htmlAfter := generateHTML(afterMarker)

	// 生成纵向和横向的 PDF
	pdfBefore, err := generatePDF(htmlBefore, "portrait")
	if err != nil {
		logger.Logger.Errorf("生成纵向PDF失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成纵向PDF失败"})
		return
	}

	pdfAfter, err := generatePDF(htmlAfter, "landscape")
	if err != nil {
		logger.Logger.Errorf("生成横向PDF失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成横向PDF失败"})
		return
	}

	// 合并两个 PDF
	mergedPdf, err := mergePDFs([][]byte{pdfBefore, pdfAfter})
	if err != nil {
		logger.Logger.Errorf("合并PDF失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "合并PDF失败"})
		return
	}

	// 添加水印（如果需要）
	if req.WmContent != "" && strings.TrimSpace(req.WmContent) != "" {
		watermarkedPdf, err := addWatermark(mergedPdf, req)
		if err != nil {
			logger.Logger.Errorf("添加水印失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "添加水印失败"})
			return
		}
		mergedPdf = watermarkedPdf
	}

	// 设置响应头并返回 PDF
	c.Header("Content-Type", "application/pdf")
	downloadFilename := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".pdf"
	encodedFilename := url.QueryEscape(downloadFilename)
	contentDisposition := fmt.Sprintf("attachment; filename*=utf-8''%s", encodedFilename)
	c.Header("Content-Disposition", contentDisposition)
	c.Header("Content-Length", fmt.Sprintf("%d", len(mergedPdf)))
	_, _ = c.Writer.Write(mergedPdf)
}
