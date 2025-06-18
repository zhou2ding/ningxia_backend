package handler

const (
	uploadDir       = "./tmp/uploads"
	maxFileSize     = 1024 * 1024 * 1024 // 1024MB
	pdfDir          = "./tmp/pdf"
	reportsBaseDir  = "./pys/output" // Base directory for saved reports
	pyDir           = "./pys"
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
	reportNameMap = map[string]string{
		ReportTypeNationalProvincial: "GSGX",
		ReportTypeExpressway:         "highway",
		ReportTypeMaintenance:        "maintain",
	}

	targetExcelFiles = []string{
		"alldata.xlsx",
		"一级公路.xlsx",
		"二三四级公路.xlsx",
		"all_disease.xlsx",

		"alldata.csv",
		"一级公路.csv",
		"二三四级公路.csv",
		"all_disease.csv",
	}
)

type exportPDFReq struct {
	WmContent  string  `form:"wm_content"`
	WmColor    string  `form:"wm_color"`
	WmOpacity  float64 `form:"wm_opacity"`
	WmFontSize int     `form:"wm_font_size"`
	WmAngle    float64 `form:"wm_angle"`
}

type (
	maintain struct {
		ProjectName      string `json:"project_name"`       // 项目名称
		MaintainXlsxFile string `json:"maintain_xlsx_file"` // 第二次监测数据
		AfterRootDir     string `json:"after_root_dir"`     // 病害数据
		BeforeRootDir    string `json:"before_root_dir"`    // 第一次检测数据
	}
	expressWay struct {
		RootPath            string  `json:"root_path"`             // 三位多功能路况快速检测车数据解压后的根路径（tmp/uploads/req-xxx-files/高速数据_extracted）
		MaintenanceUnitFile string  `json:"maintenance_unit_file"` // 管养单位明细
		KmIndexFile         string  `json:"-"`                     // 三位多功能路况快速检测车数据（zip）中的公里指标汇总报表
		UnitLevelFile       string  `json:"unit_level_file"`       // 单位层级明细
		PingdingFile        string  `json:"pingding_file"`         // CICS车检测数据
		LevelFile           string  `json:"level_file"`            // 路况技术评定
		PqiValuewd1         float64 `json:"pqi_valuewd1"`          // 公路网级沥青路面PQI
		PqiValuewd2         float64 `json:"pqi_valuewd2"`          // 公路网级沥青路面PQI
		Threshold           float64 `json:"threshold"`             // 路面PQI技术等级为优的里程占比
		PQIThreshold        float64 `json:"PQI_threshold"`         // PQI（一级及二级公路）
		PCIThreshold        float64 `json:"PCI_threshold"`         // PCI（一级及二级公路）
		RQIThreshold        float64 `json:"RQI_threshold"`         // RQI（一级及二级公路）
		RDIThreshold        float64 `json:"RDI_threshold"`         // RDI（一级及二级公路）
		SRIThreshold        float64 `json:"SRI_threshold"`         // SRI（一级及二级公路）
		Danwei              string  `json:"danwei"`                // 管养单位名称
	}
	rural struct {
		NcBaseDir string  `json:"nc_base_dir"` // 三位多功能路况快速检测车数据解压后的根路径（tmp/uploads/req-xxx-files/农路_extracted）
		UnitXlsx  string  `json:"unit_xlsx"`   // 单位层级明细
		RootDir   string  `json:"root_dir"`    // 所有数据总根路径
		XlsxFile  string  `json:"xlsx_file"`   // 管养单位明细
		GyValue   string  `json:"gy_value"`    // 管养单位名称
		PqiWd1    float64 `json:"pqi_wd1"`     // 本年度上级交通运输主管部门下达的PQI指标
		Pqi12     float64 `json:"pqi_12"`      // 公路网级沥青路面PQI（一级及二级公路）
		Pqi34     float64 `json:"pqi_34"`      // 公路网级沥青路面PQI（三级及四级公路）
	}
	nationalProvince struct {
		RootPath          string  `json:"root_path"`           // 三位多功能路况快速检测车数据解压后的根路径（tmp/uploads/req-xxx-files/国省干线_extracted）
		XlsxFile          string  `json:"xlsx_file"`           // 管养单位明细
		BitumenFolderPath string  `json:"bitumen_folder_path"` // 上一年病害数据解压后的根路径（tmp/uploads/req-xxx-files/23病害数据_extracted）
		CICScardata       string  `json:"CICScardata"`         // CICS车检测数据
		UnitPath          string  `json:"unit_path"`           // 单位层级明细
		FilePath          string  `json:"file_path"`           // 路况技术评定
		GyValue           string  `json:"gy_value"`            // 管养单位名称
		PqiValue          float64 `json:"pqi_value"`           // 本年度上级交通运输主管部门下达的PQI指标
		Wdpqi12           float64 `json:"wdpqi_12"`            // 公路网级沥青路面PQI（一级及二级公路）
		Wdpqi34           float64 `json:"wdpqi_34"`            // 公路网级沥青路面PQI（三级及四级公路）
		Pqi12             float64 `json:"pqi_12"`              // PQI（一级及二级公路）
		Pci12             float64 `json:"pci_12"`              // PCI（一级及二级公路）
		Rqi12             float64 `json:"rqi_12"`              // RQI（一级及二级公路）
		Rdi12             float64 `json:"rdi_12"`              // RDI（一级及二级公路）
		Pqi34             float64 `json:"pqi_34"`              // PQI（三级及四级公路）
		Pci34             float64 `json:"pci_34"`              // PCI（三级及四级公路）
		Rqi34             float64 `json:"rqi_34"`              // RQI（三级及四级公路）
		Rate12            float64 `json:"rate_12"`             // 公路优等路率（一级及二级公路）
		Rate34            float64 `json:"rate_34"`             // 公路优等路率（三级及四级公路）
	}
	savemdReq struct {
		ReportType       string            `json:"report_type"`
		ExpressWay       *expressWay       `json:"express_way"`
		Maintain         *maintain         `json:"maintain"`
		NationalProvince *nationalProvince `json:"national_province"`
		Rural            *rural            `json:"rural"`
	}
)
