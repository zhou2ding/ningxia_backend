package handler

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gomarkdown/markdown"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"io"
	"net/http"
	"net/url"
	"ningxia_backend/pkg/logger"
	"os"
	"os/exec"
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
        @page {
            size: A4;
            margin: 2.54cm 3.18cm; /* 上下2.54cm，左右3.18cm */
        }
        body {
            font-family: "FangSong", sans-serif; /* 正文仿宋 */
            font-size: 16pt; /* 三号字约16pt */
            line-height: 1.0; /* 单倍行距 */
            text-align: justify; /* 两端对齐 */
            text-indent: 2em; /* 首行缩进2字符 */
            margin: 0;
            padding: 0;
        }
        h1 {
            font-family: "FangSong_GBK", sans-serif; /* 方正小标宋_GBK */
            font-size: 22pt; /* 二号字约22pt */
            font-weight: bold;
            text-align: center;
            line-height: 36pt; /* 固定值36磅 */
            margin-top: 0;
            text-indent: 0; /* 标题不需要缩进 */
        }
        h2, h3, h4 {
            font-family: "FangSong_GBK", sans-serif; /* 小标题方正小标宋_GBK */
            font-size: 16pt; /* 三号字 */
            line-height: 28pt; /* 固定值28磅 */
            text-indent: 2em; /* 首行缩进 */
            text-align: justify; /* 两端对齐 */
        }
        table {
            border-collapse: collapse;
            width: 14.64cm; /* 表格总宽14.64cm */
            margin: 10pt auto; /* 水平居中 */
            font-family: "SimSun", sans-serif; /* 宋体 */
            font-size: 11pt; /* 11号字 */
			border-spacing: 0; /* 消除单元格间距 */
        }
        th, td {
            border: 0.5pt solid black; /* 0.5磅直线边框 */
            padding: 4pt 8px;
            text-align: center; /* 水平居中 */
            vertical-align: middle; /* 垂直居中 */
            font-weight: normal;
			padding-left: 0 !important; /* 强制左侧无内边距 */
			text-indent: 0 !important; /* 消除文本缩进 */
			margin-left: 0 !important;
			width: 1.08cm;
        }
        /* 其他列自动调整 */
        p {
            margin: 0 0 6pt 0;
            text-indent: 2em;
        }
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
		defer os.Remove(tempPdfPath)

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
			"pos:tl, off:55 -100", "pos:tr, off:-55 -200",
			"pos:bl, off:55 250", "pos:br, off:-55 150",
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
