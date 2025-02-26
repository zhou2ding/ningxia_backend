package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/nguyenthenguyen/docx"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io"
	"log"
	"net/http"
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

func main() {
	// 创建上传目录
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}

	r := gin.Default()

	// 解压接口
	r.POST("/api/unzip", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFileSize)

		file, err := c.FormFile("file")
		if err != nil {
			log.Printf("文件上传失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "文件上传失败"})
			return
		}

		tempDir, err := os.MkdirTemp(uploadDir, "unzip-*")
		if err != nil {
			log.Printf("创建临时目录失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时目录失败"})
			return
		}

		zipPath := filepath.Join(tempDir, file.Filename)
		if err = c.SaveUploadedFile(file, zipPath); err != nil {
			log.Printf("文件保存失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "文件保存失败"})
			return
		}

		files, err := unzip(zipPath, tempDir)
		if err != nil {
			log.Printf("文件解压失败: %v", err)
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
			log.Printf("无效请求: %v", err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		quotedFiles := make([]string, len(req.Files))
		for i, f := range req.Files {
			quotedFiles[i] = strconv.Quote(f) // 处理空格和特殊字符
		}

		cmd := exec.Command("D:\\Software\\Python\\Python313\\python.exe", append([]string{"process.py"}, quotedFiles...)...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Python执行失败 [%d]: %s\n输出: %s", cmd.ProcessState.ExitCode(), err, output)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("计算失败: %s", output)})
			return
		}
		var data map[string]string
		if err = json.Unmarshal(output, &data); err != nil {
			log.Printf("解析结果失败: %v\n原始输出: %s", err, output)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse results"})
			return
		}

		doc, err := docx.ReadDocxFile("template.docx")
		if err != nil {
			log.Printf("读取模板失败: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Template error"})
			return
		}
		defer doc.Close()

		docxFile := doc.Editable()
		for key, value := range data {
			log.Printf("will replace [%v] to %s", key, value)
			docxFile.Replace(fmt.Sprintf("%v", key), value, -1)
		}

		tmpFile, err := os.CreateTemp("", "report-*.docx")
		if err != nil {
			log.Printf("创建临时文件失败: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "无法创建临时文件"})
			return
		}
		defer os.Remove(tmpFile.Name())

		if err = docxFile.WriteToFile(tmpFile.Name()); err != nil {
			log.Printf("文档生成失败: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "文档生成失败"})
			return
		}

		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		c.Header("Content-Disposition", "attachment; filename=report.docx")
		http.ServeFile(c.Writer, c.Request, tmpFile.Name())
	})

	if err := r.Run(":12345"); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}

func unzip(src, dest string) ([]string, error) {
	var filenames []string
	r, err := zip.OpenReader(src)
	if err != nil {
		log.Printf("打开ZIP文件失败: %v", err)
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		name := f.Name
		if f.Flags&0x800 == 0 {
			decodedName, err := decodeGBK(name)
			if err != nil {
				log.Printf("GBK解码失败: %v", err)
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
			if err := os.MkdirAll(fpath, 0755); err != nil {
				log.Printf("创建目录失败: %v", err)
				return nil, err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			log.Printf("创建父目录失败: %v", err)
			return nil, err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			log.Printf("创建文件失败: %v", err)
			return nil, err
		}

		rc, err := f.Open()
		if err != nil {
			log.Printf("打开ZIP条目失败: %v", err)
			outFile.Close()
			return nil, err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			log.Printf("文件写入失败: %v", err)
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
		log.Printf("GBK解码失败: %v", err)
		return "", err
	}
	return string(buf), nil
}
