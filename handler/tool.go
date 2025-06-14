package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gomarkdown/markdown"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"html"
	"io"
	"log"
	"ningxia_backend/pkg/logger"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var whitespaceRegex = regexp.MustCompile(`\s+`)
var outputs []string

func RefreshOutputs(src []string) {
	outputs = src[:]
}

func unzip(src, dest string) ([]string, error) {
	var filenames []string
	r, err := zip.OpenReader(src)
	if err != nil {
		logger.Logger.Errorf("打开ZIP文件失败: %v", err)
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		name := f.Name
		if f.Flags&0x800 == 0 {
			decodedName, err := decodeFileName(name)
			if err != nil {
				logger.Logger.Errorf("GBK解码失败: %v", err)
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
			if err = os.MkdirAll(fpath, 0755); err != nil {
				logger.Logger.Errorf("创建目录失败: %v", err)
				return nil, err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			logger.Logger.Errorf("创建父目录失败: %v", err)
			return nil, err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			logger.Logger.Errorf("创建文件失败: %v", err)
			return nil, err
		}

		rc, err := f.Open()
		if err != nil {
			logger.Logger.Errorf("打开ZIP条目失败: %v", err)
			outFile.Close()
			return nil, err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			logger.Logger.Errorf("文件写入失败: %v", err)
			return nil, err
		}

		filenames = append(filenames, fpath)
	}
	return filenames, nil
}

func decodeFileName(name string) (string, error) {
	// 先尝试UTF-8
	if utf8.ValidString(name) {
		return name, nil
	}

	// 尝试GBK
	gbkName, err := decodeGBK(name)
	if err == nil && gbkName != name {
		return gbkName, nil
	}

	// 尝试其他常见中文编码如GB18030
	decoder := simplifiedchinese.GB18030.NewDecoder()
	gb18030Name, _, err := transform.String(decoder, name)
	if err == nil && gb18030Name != name {
		return gb18030Name, nil
	}

	return name, fmt.Errorf("无法解码文件名")
}

func decodeGBK(s string) (string, error) {
	reader := transform.NewReader(strings.NewReader(s), simplifiedchinese.GBK.NewDecoder())
	buf, err := io.ReadAll(reader)
	if err != nil {
		logger.Logger.Errorf("GBK解码失败: %v", err)
		return "", err
	}
	return string(buf), nil
}

func calculate(pySuffix string, req *savemdReq) (map[string]any, string, error) {
	var (
		program string
		args    []string
	)

	// 1. 根据 reportType 确定 Python 脚本名称和参数
	switch req.ReportType {
	case ReportTypeExpressway:
		program = "highway" + pySuffix
		args = []string{
			"--root_dir", "..\\" + req.ExpressWay.RootPath,
			"--maintenance_unit_file", "..\\" + req.ExpressWay.MaintenanceUnitFile,
			"--km_index_file", "..\\" + filepath.Join(req.ExpressWay.RootPath, "公里指标汇总报表.xlsx"),
			"--unit_level_file", "..\\" + req.ExpressWay.UnitLevelFile,
			"--pingding_file", "..\\" + req.ExpressWay.PingdingFile,
			"--level_file", "..\\" + req.ExpressWay.LevelFile,
			"--pqi_valuewd1", fmt.Sprintf("%.3f", req.ExpressWay.PqiValuewd1),
			"--pqi_valuewd2", fmt.Sprintf("%.3f", req.ExpressWay.PqiValuewd2),
			"--threshold", fmt.Sprintf("%.3f", req.ExpressWay.Threshold),
			"--PQI_threshold", fmt.Sprintf("%.3f", req.ExpressWay.PQIThreshold),
			"--PCI_threshold", fmt.Sprintf("%.3f", req.ExpressWay.PCIThreshold),
			"--RQI_threshold", fmt.Sprintf("%.3f", req.ExpressWay.RQIThreshold),
			"--RDI_threshold", fmt.Sprintf("%.3f", req.ExpressWay.RDIThreshold),
			"--SRI_threshold", fmt.Sprintf("%.3f", req.ExpressWay.SRIThreshold),
			"--danwei", req.ExpressWay.Danwei,
		}
	case ReportTypeMaintenance:
		program = "maintain" + pySuffix
		args = []string{
			"--maintain_xlsx_file", "..\\" + req.Maintain.MaintainXlsxFile,
			"--after_root_dir", "..\\" + req.Maintain.AfterRootDir,
			"--before_root_dir", "..\\" + req.Maintain.BeforeRootDir,
			"--project_name", req.Maintain.ProjectName,
		}
	case ReportTypeConstruction:
		program = "construction" + pySuffix
		args = []string{
			"--maintain_xlsx_file", "..\\" + req.Maintain.MaintainXlsxFile,
			"--after_root_dir", "..\\" + req.Maintain.AfterRootDir,
			"--before_root_dir", "..\\" + req.Maintain.BeforeRootDir,
			"--project_name", req.Maintain.ProjectName,
		}
	case ReportTypeRural:
		program = "rural" + pySuffix
		args = []string{
			"--nc_base_dir", "..\\" + req.Rural.NcBaseDir,
			"--unit_xlsx", "..\\" + req.Rural.UnitXlsx,
			"--root_dir", "..\\" + req.Rural.RootDir,
			"--xlsx_file", "..\\" + req.Rural.XlsxFile,
			"--gy_value", req.Rural.GyValue,
			"--pqi_wd1", fmt.Sprintf("%.3f", req.Rural.PqiWd1),
			"--pqi_12", fmt.Sprintf("%.3f", req.Rural.Pqi12),
			"--pqi_34", fmt.Sprintf("%.3f", req.Rural.Pqi34),
		}
	case ReportTypeNationalProvincial:
		program = "national_provincial" + pySuffix
		args = []string{
			"--root_path", "..\\" + req.NationalProvince.RootPath,
			"--xlsx_file", "..\\" + req.NationalProvince.XlsxFile,
			"--bitumen_folder_path", "..\\" + req.NationalProvince.BitumenFolderPath,
			"--CICScardata", "..\\" + req.NationalProvince.CICScardata,
			"--unit_path", "..\\" + req.NationalProvince.UnitPath,
			"--file_path", "..\\" + req.NationalProvince.FilePath,
			"--gy_value", req.NationalProvince.GyValue,
			"--pqi_value", fmt.Sprintf("%.3f", req.NationalProvince.PqiValue),
			"--wdpqi_12", fmt.Sprintf("%.3f", req.NationalProvince.Wdpqi12),
			"--wdpqi_34", fmt.Sprintf("%.3f", req.NationalProvince.Wdpqi34),
			"--pqi_12", fmt.Sprintf("%.3f", req.NationalProvince.Pqi12),
			"--pci_12", fmt.Sprintf("%.3f", req.NationalProvince.Pci12),
			"--rqi_12", fmt.Sprintf("%.3f", req.NationalProvince.Rqi12),
			"--rdi_12", fmt.Sprintf("%.3f", req.NationalProvince.Rdi12),
			"--pqi_34", fmt.Sprintf("%.3f", req.NationalProvince.Pqi34),
			"--pci_34", fmt.Sprintf("%.3f", req.NationalProvince.Pci34),
			"--rqi_34", fmt.Sprintf("%.3f", req.NationalProvince.Rqi34),
			"--rate_12", fmt.Sprintf("%.3f", req.NationalProvince.Rate12),
			"--rate_34", fmt.Sprintf("%.3f", req.NationalProvince.Rate34),
		}
	default:
		logger.Logger.Errorf("不支持的报告类型: %v", req.ReportType)
		return nil, "", errors.New("不支持的报告类型")
	}

	logger.Logger.Infof("Python脚本名称: %s", program)

	goAppWorkDir, err := os.Getwd() // 获取原始工作目录
	if err != nil {
		logger.Logger.Errorf("无法获取原始工作目录: %v", err)
		return nil, "", err
	}
	// 切换到 pys 目录
	pysDirForChdir := filepath.Join(goAppWorkDir, "pys") // 确保路径正确
	if err = os.Chdir(pysDirForChdir); err != nil {
		logger.Logger.Errorf("os.Chdir 切换到目录 [%s] 失败: %v", pysDirForChdir, err)
		return nil, "", fmt.Errorf("切换到目录 [%s] 失败: %w", pysDirForChdir, err)
	}

	// 确保在函数结束时切回原始工作目录
	defer func() {
		if err = os.Chdir(goAppWorkDir); err != nil {
			logger.Logger.Warnf("os.Chdir 切回原始目录 [%s] 失败: %v", goAppWorkDir, err)
		} else {
			logger.Logger.Infof("Go程序当前工作目录已恢复到: %s", goAppWorkDir)
		}
	}()

	absExecutablePathInPys, err := filepath.Abs(program)
	if err != nil {
		logger.Logger.Errorf("无法获取程序 [%s] 在当前目录 [%s] 下的绝对路径: %v", program, pysDirForChdir, err)
		return nil, "", fmt.Errorf("无法获取程序 [%s] 的绝对路径: %w", program, err)
	}
	logger.Logger.Infof("将要执行的程序绝对路径: %s", absExecutablePathInPys)

	// 2. 获取执行前的目录列表
	logger.Logger.Infof("扫描基础报告目录 [%s] (执行前)...", OutPutDir)
	dirsBefore, err := listDirs(OutPutDir)
	if err != nil {
		logger.Logger.Errorf("无法列出目录 [%s] (执行前): %v", OutPutDir, err)
		return nil, "", fmt.Errorf("无法列出目录 (执行前): %w", err)
	}
	logger.Logger.Debugf("执行前目录列表: %v", dirsBefore)

	// 3. 执行 Python 脚本
	cmd := exec.Command(absExecutablePathInPys, args...)
	logger.Logger.Infof("准备执行命令: %s %s", cmd.Path, strings.Join(cmd.Args[1:], " "))

	// 获取 Python 脚本的合并输出 (stdout + stderr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		exitCode := -1
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		}
		logger.Logger.Errorf("Python脚本执行失败 (退出码 %d): %v，错误内容：%s", exitCode, err, string(output))
		return nil, "", fmt.Errorf("python脚本执行失败 (退出码 %d) : %w", exitCode, err)
	}

	logger.Logger.Info("Python脚本已执行。")

	// 4. Python 脚本执行成功，获取执行后的目录列表
	logger.Logger.Infof("扫描基础报告目录 [%s] (执行后)...", OutPutDir)
	dirsAfter, err := listDirs(OutPutDir)
	if err != nil {
		logger.Logger.Errorf("无法列出目录 [%s] (执行后): %v", OutPutDir, err)
		// 脚本可能成功执行了，但我们无法确定输出目录，这是一个问题
		return nil, "", fmt.Errorf("脚本执行成功但无法列出目录 (执行后): %w", err)
	}
	logger.Logger.Debugf("执行后目录列表: %v", dirsAfter)

	// 5. 找出新增的目录
	var newReportDirName string
	var foundNewDirs []string
	for dirName := range dirsAfter {
		if _, exists := dirsBefore[dirName]; !exists {
			foundNewDirs = append(foundNewDirs, dirName)
		}
	}
	if len(foundNewDirs) == 0 {
		logger.Logger.Error("Python脚本执行成功，但在基础报告目录 [%s] 中未找到新生成的报告子目录。", OutPutDir)
		return nil, "", errors.New("未找到新生成的报告目录")
	}
	if len(foundNewDirs) > 1 {
		// 如果发现多个新目录，可能意味着并发问题或脚本行为超出预期。
		logger.Logger.Warnf("在基础报告目录 [%s] 中发现多个新生成的报告子目录: %v。将使用第一个: %s", OutPutDir, foundNewDirs, foundNewDirs[0])
	}
	newReportDirName = foundNewDirs[0]
	logger.Logger.Infof("检测到新生成的报告目录名: %s", newReportDirName)

	// 6. 构建 output.json 的完整路径并读取内容
	jsonResultFilePath := filepath.Join(OutPutDir, newReportDirName, "output.json")
	logger.Logger.Infof("尝试读取JSON结果文件: %s", jsonResultFilePath)

	jsonFileContent, err := os.ReadFile(jsonResultFilePath)
	if err != nil {
		logger.Logger.Errorf("读取JSON结果文件 [%s] 失败: %v", jsonResultFilePath, err)
		// 返回已找到的目录名，即使文件读取失败，因为目录确实被创建了
		return nil, newReportDirName, fmt.Errorf("读取报告结果文件 [%s] 失败: %w", jsonResultFilePath, err)
	}

	// 7. 解析 JSON 数据
	var data map[string]any
	if err = json.Unmarshal(jsonFileContent, &data); err != nil {
		logger.Logger.Errorf("解析JSON结果文件 [%s] 内容失败: %v\n文件原始内容: %s", jsonResultFilePath, err, string(jsonFileContent))
		return nil, newReportDirName, fmt.Errorf("解析报告结果文件 [%s] 内容失败: %w", jsonResultFilePath, err)
	}
	logger.Logger.Infof("成功从 [%s] 解析报告数据。", jsonResultFilePath)

	return data, filepath.Join(ReportsBaseDir, newReportDirName), nil
}

func extractTimestamp(dirName string) int64 {
	lastUnderscore := strings.LastIndex(dirName, "_")

	if lastUnderscore == -1 {
		return 0
	}

	timeStr := dirName[lastUnderscore+1:]
	timestamp, err := time.Parse("20060102150405", timeStr)
	if err != nil {
		return 0
	}

	return timestamp.Unix()
}

func convertExcelToMarkdown2(filePath string) (string, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return "", fmt.Errorf("无法打开Excel文件 '%s': %w", filePath, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			logger.Logger.Errorf("关闭Excel文件 '%s' 失败: %v", filePath, err)
		}
	}()

	sheetList := f.GetSheetList()
	if len(sheetList) == 0 {
		return "", fmt.Errorf("excel文件 '%s' 中没有工作表", filePath)
	}
	sheetName := sheetList[0] // Process only the first sheet

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return "", fmt.Errorf("无法从工作表 '%s' (文件 '%s') 读取行数据: %w", sheetName, filePath, err)
	}

	if len(rows) == 0 {
		logger.Logger.Infof("Excel文件 '%s' 的工作表 '%s' 为空。", filePath, sheetName)
		return "", nil // Empty sheet
	}

	header := rows[0]
	if len(header) == 0 {
		logger.Logger.Infof("Excel文件 '%s' 的工作表 '%s' 表头行为空。", filePath, sheetName)
		return "", nil // Empty header
	}

	var mdTable strings.Builder
	var displayHeaderNames []string     // Column names to display in Markdown
	var displayColOriginalIndices []int // Original Excel column indices for the display columns
	highlightLogic := make(map[int]int) // Key: index of data column to potentially highlight, Value: index of its corresponding "_合格" column

	// 1. Parse header to identify columns to display and columns for highlight logic
	for i, cellName := range header {
		trimmedCellName := strings.TrimSpace(cellName)
		if strings.HasSuffix(trimmedCellName, "_合格") || strings.HasSuffix(trimmedCellName, "_Qualified") {
			// This is a qualifier column. It will be hidden.
			// Its left neighbor (if exists) is the data column.
			if i > 0 {
				highlightLogic[i-1] = i // Map data column index to its qualifier column index
			}
		} else {
			// This is not a qualifier column. It should be displayed.
			displayHeaderNames = append(displayHeaderNames, trimmedCellName)
			displayColOriginalIndices = append(displayColOriginalIndices, i)
		}
	}

	if len(displayHeaderNames) == 0 {
		logger.Logger.Infof("解析后，Excel文件 '%s' 没有可显示的列 (所有列都以 '_合格' 结尾或表头为空)。", filePath)
		return "", nil // No columns left to display
	}

	// 2. Build Markdown table header
	mdTable.WriteString("|")
	for _, name := range displayHeaderNames {
		sanitizedName := strings.ReplaceAll(name, "|", "\\|") // Escape pipes in header names
		mdTable.WriteString(" ")
		mdTable.WriteString(sanitizedName)
		mdTable.WriteString(" |")
	}
	mdTable.WriteString("\n")

	// 3. Build Markdown table separator
	mdTable.WriteString("|")
	for range displayHeaderNames {
		mdTable.WriteString("---|")
	}
	mdTable.WriteString("\n")

	// 4. Process data rows
	for r := 1; r < len(rows); r++ { // Start from the second row (data)
		excelRow := rows[r]
		mdTable.WriteString("|")

		for _, dataColIdx := range displayColOriginalIndices { // Iterate through columns that should be displayed
			cellValue := ""
			if dataColIdx < len(excelRow) {
				cellValue = excelRow[dataColIdx]
			}
			cellValue = strings.TrimSpace(cellValue) // Trim spaces from cell value

			// ------ MODIFIED LOGIC FOR COMMA HANDLING ------
			// 将单元格内所有连续的空白字符（包括换行、多个空格等）替换为单个英文逗号。 然后去除可能由此产生的开头或结尾的逗号。
			if cellValue != "" { // 避免对空字符串使用正则
				cellValue = whitespaceRegex.ReplaceAllString(cellValue, ",")
				cellValue = strings.Trim(cellValue, ",") // 例如，"  \n abc \n  " 会变成 ",abc," 再变成 "abc"
			}
			processedCellContent := ""
			if strings.Contains(cellValue, ",") {
				parts := strings.Split(cellValue, ",")
				var lines []string
				elementsPerLine := 6

				for i := 0; i < len(parts); i += elementsPerLine {
					end := i + elementsPerLine
					if end > len(parts) {
						end = len(parts)
					}
					chunk := parts[i:end]

					sanitizedChunk := make([]string, 0, len(chunk))
					for _, p := range chunk {
						trimmedPart := strings.TrimSpace(p)
						if trimmedPart != "" { // 只包含非空部分
							sanitizedChunk = append(sanitizedChunk, strings.ReplaceAll(trimmedPart, "|", "\\|"))
						}
					}
					if len(sanitizedChunk) > 0 { // 只有在清理后还有内容时才添加
						lines = append(lines, strings.Join(sanitizedChunk, ","))
					}
				}
				processedCellContent = strings.Join(lines, "<br>")
			} else {
				// 没有逗号（或所有逗号都被移除了），直接进行管道符转义
				processedCellContent = strings.ReplaceAll(cellValue, "|", "\\|")
			}
			// ------ END OF MODIFIED LOGIC ------

			applyRedBackground := false
			// Check if this data column has a corresponding qualifier column for highlighting
			if qualifierColIdx, needsHighlightCheck := highlightLogic[dataColIdx]; needsHighlightCheck {
				if qualifierColIdx < len(excelRow) {
					qualifierValue := strings.ToUpper(strings.TrimSpace(excelRow[qualifierColIdx]))
					if qualifierValue == "FALSE" { // "FALSE" (case-insensitive) means highlight
						applyRedBackground = true
					}
				}
			}

			if applyRedBackground {
				// Using HTML span for background color. This works in many Markdown viewers.
				cellValueFormatted := fmt.Sprintf(`<span style="background-color: #D32F2F; color: white; padding: 2px 4px;">%s</span>`, processedCellContent)
				mdTable.WriteString(" ")
				mdTable.WriteString(cellValueFormatted)
				mdTable.WriteString(" |")
			} else {
				mdTable.WriteString(" ")
				mdTable.WriteString(processedCellContent) // Use the potentially multi-line processed content
				mdTable.WriteString(" |")
			}
		}
		mdTable.WriteString("\n")
	}

	return mdTable.String(), nil
}

func convertExcelToMarkdown(filePath string) (string, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return "", fmt.Errorf("无法打开Excel文件 '%s': %w", filePath, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			logger.Logger.Errorf("关闭Excel文件 '%s' 失败: %v", filePath, err)
		}
	}()

	sheetList := f.GetSheetList()
	if len(sheetList) == 0 {
		return "", fmt.Errorf("excel文件 '%s' 中没有工作表", filePath)
	}
	sheetName := sheetList[0] // Process only the first sheet

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return "", fmt.Errorf("无法从工作表 '%s' (文件 '%s') 读取行数据: %w", sheetName, filePath, err)
	}

	if len(rows) == 0 {
		logger.Logger.Infof("Excel文件 '%s' 的工作表 '%s' 为空。", filePath, sheetName)
		return "", nil // Empty sheet
	}

	header := rows[0]
	if len(header) == 0 {
		logger.Logger.Infof("Excel文件 '%s' 的工作表 '%s' 表头行为空。", filePath, sheetName)
		return "", nil // Empty header
	}

	var displayHeaderNames []string     // Column names to display
	var displayColOriginalIndices []int // Original Excel column indices for the display columns
	highlightLogic := make(map[int]int) // Key: index of data column, Value: index of its "_合格" column

	var hasWeightedColumn bool = false
	var weightedColumnDisplayIndex int = -1       // Index within displayHeaderNames
	var weightedColumnOriginalExcelIndex int = -1 // Index within original Excel header

	// 1. Parse header
	for i, cellName := range header {
		trimmedCellName := strings.TrimSpace(cellName)
		if strings.HasSuffix(trimmedCellName, "_合格") || strings.HasSuffix(trimmedCellName, "_Qualified") {
			if i > 0 {
				highlightLogic[i-1] = i
			}
		} else {
			displayHeaderNames = append(displayHeaderNames, trimmedCellName)
			displayColOriginalIndices = append(displayColOriginalIndices, i)
			// 检查是否是“加权”列，并且通常它是最后一列
			// 为了简单起见，我们检查它是否包含“加权”并且是当前处理到的最后一列 *displayable* header
			if strings.Contains(trimmedCellName, "加权") {
				// 确认它是最后一列（或者至少是显示列中的最后一列）
				// 这里的逻辑是，如果它是最后一个被加入displayHeaderNames的含“加权”的列，就标记它
				hasWeightedColumn = true
				weightedColumnDisplayIndex = len(displayHeaderNames) - 1 // Its index in the displayHeaderNames slice
				weightedColumnOriginalExcelIndex = i                     // Its original index in Excel
			}
		}
	}

	// 如果“加权”列不是最后一列可显示列，则不进行特殊处理 (可选逻辑，根据实际需求调整)
	// 当前逻辑是：只要存在名为“加权”的可显示列，就假设它是我们关心的那一个。
	// 如果有多个“加权”列，此逻辑会以最后一个为准。
	// 如果“加权”列被 `_合格` 等后缀隐藏了，它就不会在 `displayHeaderNames` 中，`hasWeightedColumn` 会是 false。

	if len(displayHeaderNames) == 0 {
		logger.Logger.Infof("解析后，Excel文件 '%s' 没有可显示的列。", filePath)
		return "", nil
	}

	// 如果没有数据行，或者只有一行数据，则加权列合并无意义，退回普通表格
	if hasWeightedColumn && (len(rows)-1 < 2) {
		logger.Logger.Infof("文件 '%s' 的'加权'列数据行数少于2行 (%d)，不执行HTML单元格合并。", filePath, len(rows)-1)
		hasWeightedColumn = false
	}

	// --- 如果检测到“加权”列并且有足够的数据行，则生成HTML表格 ---
	if hasWeightedColumn {
		logger.Logger.Infof("文件 '%s' 检测到 '加权' 列，将生成 HTML 表格以支持单元格合并。", filePath)
		var htmlTable strings.Builder
		htmlTable.WriteString("<table>\n")

		// 2.1 Build HTML table header
		htmlTable.WriteString("  <thead>\n    <tr>\n")
		for _, name := range displayHeaderNames {
			sanitizedName := html.EscapeString(name) // HTML转义
			htmlTable.WriteString("      <th>")
			htmlTable.WriteString(sanitizedName)
			htmlTable.WriteString("</th>\n")
		}
		htmlTable.WriteString("    </tr>\n  </thead>\n")

		// 2.2 Build HTML table body
		htmlTable.WriteString("  <tbody>\n")
		var weightedValueProcessed string // 存储处理后的“加权”列的值
		if len(rows) > 1 && weightedColumnOriginalExcelIndex < len(rows[1]) {
			// 从第一行数据中获取“加权”列的值
			rawValue := rows[1][weightedColumnOriginalExcelIndex] // 取第一行数据作为代表值
			rawValue = strings.TrimSpace(rawValue)
			if rawValue != "" {
				rawValue = whitespaceRegex.ReplaceAllString(rawValue, ",")
				rawValue = strings.Trim(rawValue, ",")
			}
			// 与下方普通单元格处理逻辑类似，但不进行<br>分割，因为它是合并单元格
			weightedValueProcessed = html.EscapeString(strings.ReplaceAll(rawValue, "|", "\\|"))
		}

		for r := 1; r < len(rows); r++ { // Start from the second row (data)
			excelRow := rows[r]
			htmlTable.WriteString("    <tr>\n")

			for cIdx, dataColOriginalExcelIdx := range displayColOriginalIndices {
				cellValue := ""
				if dataColOriginalExcelIdx < len(excelRow) {
					cellValue = excelRow[dataColOriginalExcelIdx]
				}
				cellValue = strings.TrimSpace(cellValue)

				if cellValue != "" {
					cellValue = whitespaceRegex.ReplaceAllString(cellValue, ",")
					cellValue = strings.Trim(cellValue, ",")
				}

				processedCellContent := ""
				if strings.Contains(cellValue, ",") {
					parts := strings.Split(cellValue, ",")
					var lines []string
					elementsPerLine := 6
					for i := 0; i < len(parts); i += elementsPerLine {
						end := i + elementsPerLine
						if end > len(parts) {
							end = len(parts)
						}
						chunk := parts[i:end]
						var sanitizedChunk []string
						for _, p := range chunk {
							trimmedPart := strings.TrimSpace(p)
							if trimmedPart != "" {
								sanitizedChunk = append(sanitizedChunk, html.EscapeString(strings.ReplaceAll(trimmedPart, "|", "\\|")))
							}
						}
						if len(sanitizedChunk) > 0 {
							lines = append(lines, strings.Join(sanitizedChunk, ","))
						}
					}
					processedCellContent = strings.Join(lines, "<br>")
				} else {
					processedCellContent = html.EscapeString(strings.ReplaceAll(cellValue, "|", "\\|"))
				}

				applyRedBackground := false
				if qualifierColIdx, needsHighlightCheck := highlightLogic[dataColOriginalExcelIdx]; needsHighlightCheck {
					if qualifierColIdx < len(excelRow) {
						qualifierValue := strings.ToUpper(strings.TrimSpace(excelRow[qualifierColIdx]))
						if qualifierValue == "FALSE" {
							applyRedBackground = true
						}
					}
				}

				style := ""
				if applyRedBackground {
					style = ` style="background-color: #D32F2F; color: white; padding: 2px 4px;"`
				}

				if cIdx == weightedColumnDisplayIndex { // 当前是“加权”列
					if r == 1 { // 只在第一行数据输出这个单元格，并设置rowspan
						numDataRows := len(rows) - 1
						htmlTable.WriteString(fmt.Sprintf(`      <td rowspan="%d"%s>`, numDataRows, style))
						htmlTable.WriteString(weightedValueProcessed) // 使用之前提取并处理好的值
						htmlTable.WriteString("</td>\n")
					}
					// 对于后续行，不输出此“加权”列的td，因为它已被rowspan覆盖
				} else {
					htmlTable.WriteString(fmt.Sprintf("      <td%s>", style))
					htmlTable.WriteString(processedCellContent)
					htmlTable.WriteString("</td>\n")
				}
			}
			htmlTable.WriteString("    </tr>\n")
		}
		htmlTable.WriteString("  </tbody>\n")
		htmlTable.WriteString("</table>\n")
		return htmlTable.String(), nil
	}

	// --- 否则，生成Markdown表格 (现有逻辑) ---
	logger.Logger.Infof("文件 '%s' 未检测到 '加权' 列或不满足合并条件，将生成 Markdown 表格。", filePath)
	var mdTable strings.Builder
	// 2. Build Markdown table header
	mdTable.WriteString("|")
	for _, name := range displayHeaderNames {
		sanitizedName := strings.ReplaceAll(name, "|", "\\|")
		mdTable.WriteString(" ")
		mdTable.WriteString(sanitizedName)
		mdTable.WriteString(" |")
	}
	mdTable.WriteString("\n")

	// 3. Build Markdown table separator
	mdTable.WriteString("|")
	for range displayHeaderNames {
		mdTable.WriteString("---|")
	}
	mdTable.WriteString("\n")

	// 4. Process data rows
	for r := 1; r < len(rows); r++ {
		excelRow := rows[r]
		mdTable.WriteString("|")

		for _, dataColIdx := range displayColOriginalIndices {
			cellValue := ""
			if dataColIdx < len(excelRow) {
				cellValue = excelRow[dataColIdx]
			}
			cellValue = strings.TrimSpace(cellValue)

			if cellValue != "" {
				cellValue = whitespaceRegex.ReplaceAllString(cellValue, ",")
				cellValue = strings.Trim(cellValue, ",")
			}
			processedCellContent := ""
			if strings.Contains(cellValue, ",") {
				parts := strings.Split(cellValue, ",")
				var lines []string
				elementsPerLine := 6
				for i := 0; i < len(parts); i += elementsPerLine {
					end := i + elementsPerLine
					if end > len(parts) {
						end = len(parts)
					}
					chunk := parts[i:end]
					var sanitizedChunk []string
					for _, p := range chunk {
						trimmedPart := strings.TrimSpace(p)
						if trimmedPart != "" {
							sanitizedChunk = append(sanitizedChunk, strings.ReplaceAll(trimmedPart, "|", "\\|"))
						}
					}
					if len(sanitizedChunk) > 0 {
						lines = append(lines, strings.Join(sanitizedChunk, ","))
					}
				}
				processedCellContent = strings.Join(lines, "<br>")
			} else {
				processedCellContent = strings.ReplaceAll(cellValue, "|", "\\|")
			}

			applyRedBackground := false
			if qualifierColIdx, needsHighlightCheck := highlightLogic[dataColIdx]; needsHighlightCheck {
				if qualifierColIdx < len(excelRow) {
					qualifierValue := strings.ToUpper(strings.TrimSpace(excelRow[qualifierColIdx]))
					if qualifierValue == "FALSE" {
						applyRedBackground = true
					}
				}
			}

			if applyRedBackground {
				cellValueFormatted := fmt.Sprintf(`<span style="background-color: #D32F2F; color: white; padding: 2px 4px;">%s</span>`, processedCellContent)
				mdTable.WriteString(" ")
				mdTable.WriteString(cellValueFormatted)
				mdTable.WriteString(" |")
			} else {
				mdTable.WriteString(" ")
				mdTable.WriteString(processedCellContent)
				mdTable.WriteString(" |")
			}
		}
		mdTable.WriteString("\n")
	}
	return mdTable.String(), nil
}

func listDirs(basePath string) (map[string]struct{}, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("读取目录 %s 失败: %w", basePath, err)
	}
	dirs := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() {
			dirs[entry.Name()] = struct{}{}
		}
	}
	return dirs, nil
}

func generateHTML(mdContent string) string {
	htmlContentBytes := markdown.ToHTML([]byte(mdContent), nil, nil)
	htmlContentString := string(htmlContentBytes)
	htmlHeadContent := `
<head>
<meta charset="utf-8">
<style>
@page {
	size: A4;
	margin: 2.54cm 3.18cm;
}
body {
	font-family: "FangSong", "SimSun", sans-serif;
	font-size: 16pt;
	line-height: 1.0;
	text-align: justify;
	margin: 0;
	padding: 0;
}
h1 {
	font-family: "FangSong_GBK", "SimSun", sans-serif;
	font-size: 22pt;
	font-weight: bold;
	text-align: center;
	line-height: 36pt;
	margin-top: 0;
	margin-bottom: 24pt;
	text-indent: 0;
}
h2 {
    font-family: "FangSong_GBK", "SimSun", sans-serif;
    font-size: 16pt;
    font-weight: bold;
    line-height: 28pt;
    text-align: center; 
    text-indent: 0;     
    margin-top: 1.5em;
    margin-bottom: 1em;
}
h3, h4 {
	font-family: "FangSong_GBK", "SimSun", sans-serif;
	font-size: 16pt;
	font-weight: bold;
	line-height: 28pt;
	text-indent: 2em;
	text-align: justify;
	margin-top: 1.5em;
	margin-bottom: 1em;
}
p {
	font-family: "FangSong", "SimSun", sans-serif;
	font-size: 16pt;
	line-height: 1.0;
	text-align: justify;
	text-indent: 2em;
	margin: 0 0 8pt 0;
}
table {
	border-collapse: collapse;
	width: 14.64cm;
	margin: 10pt auto 20pt auto;
	font-family: "SimSun", sans-serif;
	font-size: 11pt;
	border-spacing: 0;
}
th, td {
	border: 0.5pt solid black;
	padding: 4pt 6pt;
	text-align: center;
	vertical-align: middle;
	font-weight: normal;
}
th {
	font-weight: bold;
}
td p, th p {
	font-family: "SimSun", sans-serif !important;
	font-size: 11pt !important;
	text-indent: 0em !important;
	margin: 0 !important;
	padding: 0 !important;
	line-height: normal !important;
	text-align: inherit !important;
}
ul, ol {
	text-indent: 0;
	padding-left: 2.5em;
	margin-top: 0.5em;
	margin-bottom: 0.5em;
	text-align: justify;
}
li {
	text-indent: 0;
	margin-bottom: 0.25em;
}
</style>
</head>
`
	htmlWithHead := "<html>\n" + htmlHeadContent + "\n<body>\n" + htmlContentString + "\n</body>\n</html>"
	return htmlWithHead
}

func generatePDF(htmlContent string, orientation string) ([]byte, error) {
	cmdArgs := []string{
		"--page-size", "A4",
		"--encoding", "UTF-8",
		"--disable-smart-shrinking",
		"--orientation", orientation,
		"-", "-",
	}
	cmd := exec.Command(wkhtmltopdfPath, cmdArgs...)
	cmd.Stdin = strings.NewReader(htmlContent)
	var pdfBytesBuffer bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &pdfBytesBuffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("执行 wkhtmltopdf 失败: %v Stderr: %s", err, stderr.String())
	}
	if stderr.Len() > 0 {
		logger.Logger.Infof("wkhtmltopdf stderr (warnings): %s", stderr.String())
	}
	return pdfBytesBuffer.Bytes(), nil
}

func mergePDFs(pdfs [][]byte) ([]byte, error) {
	tempFiles := make([]string, len(pdfs))
	for i, pdf := range pdfs {
		tempFile, err := os.CreateTemp("", "pdf_*.pdf")
		if err != nil {
			return nil, fmt.Errorf("创建临时文件失败: %v", err)
		}
		// 写入数据
		_, err = tempFile.Write(pdf)
		if err != nil {
			_ = tempFile.Close()           // 出错时关闭文件
			_ = os.Remove(tempFile.Name()) // 出错时删除文件
			return nil, fmt.Errorf("写入临时文件失败: %v", err)
		}
		// 立即关闭文件，释放句柄
		if err = tempFile.Close(); err != nil {
			_ = os.Remove(tempFile.Name()) // 关闭失败时删除文件
			return nil, fmt.Errorf("关闭临时文件失败: %v", err)
		}
		tempFiles[i] = tempFile.Name() // 保存文件路径
	}

	// 合并 PDF
	mergedPdfBuffer := new(bytes.Buffer)
	err := api.Merge("", tempFiles, mergedPdfBuffer, nil, false)
	if err != nil {
		// 合并失败时清理临时文件
		for _, tempFile := range tempFiles {
			_ = os.Remove(tempFile)
		}
		return nil, fmt.Errorf("合并PDF失败: %v", err)
	}

	// 合并成功后清理临时文件
	for _, tempFile := range tempFiles {
		_ = os.Remove(tempFile)
	}

	return mergedPdfBuffer.Bytes(), nil
}

func addWatermark(pdfBytes []byte, req exportPDFReq) ([]byte, error) {
	tempPdfFile, err := os.CreateTemp("", "pdf_*.pdf")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tempPdfFile.Name())
	_, err = tempPdfFile.Write(pdfBytes)
	if err != nil {
		return nil, fmt.Errorf("写入临时文件失败: %v", err)
	}
	rsForWatermarking, err := os.Open(tempPdfFile.Name())
	if err != nil {
		return nil, fmt.Errorf("无法打开临时文件: %v", err)
	}
	defer rsForWatermarking.Close()
	cnf := model.NewDefaultConfiguration()
	cnf.Unit = types.POINTS
	baseDesc := fmt.Sprintf("points:%d, rotation:%.3f, opacity:%.3f, fillcolor:%s, font:%s", req.WmFontSize, req.WmAngle, req.WmOpacity/100, req.WmColor, UserFont)
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
	ctx, err := api.ReadContextFile(tempPdfFile.Name())
	if err != nil {
		return nil, fmt.Errorf("读取PDF信息失败: %v", err)
	}
	pageCount := ctx.PageCount
	if pageCount == 0 {
		return nil, fmt.Errorf("PDF文件没有页面")
	}
	watermarkMap := make(map[int][]*model.Watermark)
	for i := 1; i <= pageCount; i++ {
		watermarkMap[i] = watermarksForOnePage
	}
	watermarkedPdfBuffer := new(bytes.Buffer)
	err = api.AddWatermarksSliceMap(rsForWatermarking, watermarkedPdfBuffer, watermarkMap, cnf)
	if err != nil {
		return nil, fmt.Errorf("添加水印失败: %v", err)
	}
	return watermarkedPdfBuffer.Bytes(), nil
}
