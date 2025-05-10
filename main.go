package main

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/pdfcpu/pdfcpu/pkg/font"
	"ningxia_backend/dao"
	"ningxia_backend/handler"
	"ningxia_backend/pkg/conf"
	"ningxia_backend/pkg/logger"
	"os"
	"path/filepath"
)

const (
	uploadDir = "./tmp/uploads"
	pdfDir    = "./tmp/pdf"
)

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
	err := dao.InitDB()
	if err != nil {
		logger.Logger.Errorf("初始化数据库失败: %v", err)
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
	for _, dir := range handler.ReportDirs {
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
	r.POST("/api/unzip", handler.UnzipHandler())

	// 计算接口
	//r.POST("/api/calculate/docx", handler.SaveDocxHandler(pySuffix))
	r.POST("/api/calculate/md", handler.SaveMdHandler(pySuffix))

	report := r.Group("/api/reports")
	{
		report.GET("list", handler.GetReports)
		report.GET("/view/:filename", handler.ViewMarkdownHandler) //查看md
		//report.GET("/download/:filename", handler.DownloadWordHandler)   //下载docx
		report.GET("/export/:filename", handler.ExportReportHandler)     //下载pdf
		report.DELETE("/:filename", handler.DeleteReportHandler)         // 删除报告
		report.GET("/extraExport/:filename", handler.ExtraExportHandler) // 特殊导出：年度指标达标情况
	}

	r.GET("/file", handler.GetFileHandler)

	setting := r.Group("/api/settings")
	{
		setting.POST("/province", handler.SaveProvinceSettings)
		setting.POST("/national", handler.SaveNationalSettings)
		setting.GET("/province/:year", handler.GetProvinceSetting)
		setting.GET("/national/:plan", handler.GetNationalSetting)
	}

	road := r.Group("/api/road")
	{
		road.GET("list", handler.GetRoads)
	}

	if err = r.Run(":12345"); err != nil {
		logger.Logger.Errorf("启动服务器失败: %v", err)
		return
	}
}
