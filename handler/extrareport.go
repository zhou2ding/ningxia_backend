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

func ExtraExportHandler(c *gin.Context) {
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
