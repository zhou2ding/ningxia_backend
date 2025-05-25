package handler

const (
	uploadDir                     = "./tmp/uploads"
	maxFileSize                   = 1024 * 1024 * 1024 // 1024MB
	pdfDir                        = "./tmp/pdf"
	outputsDir                    = "./pys/output"
	reportsBaseDir                = "./pys/output" // Base directory for saved reports
	expresswayReportBaseDir       = "./pys/output/highway_20250520205333"
	maintenanceReportBaseDir      = "./pys/output/maintain_20250522110845"
	constructionReportBaseDir     = "./pys/output/maintain_20250522110845"
	ruralReportBaseDir            = "./pys/output/rural"
	nationalProvinceReportBaseDir = "./pys/output/GSGX_20250525121953"

	wkhtmltopdfPath = "./wkhtmltox/bin/wkhtmltopdf.exe"
)

const (
	ReportTypeExpressway         = "EXPRESSWAY"
	ReportTypeMaintenance        = "MAINTENANCE"
	ReportTypeConstruction       = "CONSTRUCTION"
	ReportTypeRural              = "RURAL"
	ReportTypeNationalProvincial = "NATIONAL_PROVINCIAL"

	PyRespImagesKey      = "IMAGES"
	PyRespExtraImagesKey = "EXTRA_IMAGES"
	UserFont             = "FZHTJW--GB1-0"
)

var (
	ReportNameMap = map[string]string{
		ReportTypeNationalProvincial: "GSGX",
		ReportTypeExpressway:         "highway",
		ReportTypeMaintenance:        "maintain",
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
