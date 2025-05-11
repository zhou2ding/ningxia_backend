package handler

const (
	uploadDir                     = "./tmp/uploads"
	maxFileSize                   = 1024 * 1024 * 1024 // 1024MB
	pdfDir                        = "./tmp/pdf"
	reportsBaseDir                = "./reports" // Base directory for saved reports
	expresswayReportBaseDir       = "./reports/expressway"
	maintenanceReportBaseDir      = "./reports/maintenance"
	constructionReportBaseDir     = "./reports/construction"
	ruralReportBaseDir            = "./reports/rural"
	nationalProvinceReportBaseDir = "./reports/nationalProvince"
	//marketReportBaseDir           = "./reports/market"
	wkhtmltopdfPath = "./wkhtmltox/bin/wkhtmltopdf.exe"
)

const (
	ReportTypeExpressway         = "EXPRESSWAY"
	ReportTypeMaintenance        = "MAINTENANCE"
	ReportTypeConstruction       = "CONSTRUCTION"
	ReportTypeRural              = "RURAL"
	ReportTypeNationalProvincial = "NATIONAL_PROVINCIAL"
	//ReportTypeMarket             = "MARKET"

	PyRespImagesKey      = "IMAGES"
	PyRespExtraImagesKey = "EXTRA_IMAGES"
	UserFont             = "FZHTJW--GB1-0"
)

var (
	ReportNameMap = map[string]string{
		ReportTypeExpressway:         "高速公路抽检路段公路技术状况监管分析报告",
		ReportTypeMaintenance:        "养护工程路段技术状况监管分析报告",
		ReportTypeConstruction:       "建设工程路段技术状况监管分析报告",
		ReportTypeRural:              "农村公路抽检路段公路技术状况监管分析报告",
		ReportTypeNationalProvincial: "普通国省干线抽检路段公路技术状况监管分析报告",
		//ReportTypeMarket:             "市场化路段抽检路段公路技术状况监管分析报告",
	}
	ReportDirs = []string{
		expresswayReportBaseDir,
		maintenanceReportBaseDir,
		constructionReportBaseDir,
		ruralReportBaseDir,
		nationalProvinceReportBaseDir,
		//marketReportBaseDir,
	}
)

type exportPDFReq struct {
	WmContent  string  `form:"wm_content"`
	WmColor    string  `form:"wm_color"`
	WmOpacity  float64 `form:"wm_opacity"`
	WmFontSize int     `form:"wm_font_size"`
	WmAngle    float64 `form:"wm_angle"`
}
