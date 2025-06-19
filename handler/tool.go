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
	"gorm.io/gorm/utils"
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

	goAppWorkDir, err := os.Getwd() // 获取原始工作目录
	if err != nil {
		logger.Logger.Errorf("无法获取原始工作目录: %v", err)
		return nil, "", err
	}

	// 1. 根据 reportType 确定 Python 脚本名称和参数
	switch req.ReportType {
	case ReportTypeExpressway:
		program = "highway" + pySuffix
		args = []string{
			"--root_dir", filepath.Join(goAppWorkDir, req.ExpressWay.RootPath),
			"--maintenance_unit_file", filepath.Join(goAppWorkDir, req.ExpressWay.MaintenanceUnitFile),
			"--km_index_file", filepath.Join(goAppWorkDir, req.ExpressWay.RootPath, "公里指标汇总报表.xlsx"),
			"--unit_level_file", filepath.Join(goAppWorkDir, req.ExpressWay.UnitLevelFile),
			"--pingding_file", filepath.Join(goAppWorkDir, req.ExpressWay.PingdingFile),
			"--level_file", filepath.Join(goAppWorkDir, req.ExpressWay.LevelFile),
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
			"--maintain_xlsx_file", filepath.Join(goAppWorkDir, req.Maintain.MaintainXlsxFile),
			"--after_root_dir", filepath.Join(goAppWorkDir, req.Maintain.AfterRootDir),
			"--before_root_dir", filepath.Join(goAppWorkDir, req.Maintain.BeforeRootDir),
			"--project_name", req.Maintain.ProjectName,
		}
	case ReportTypeConstruction:
		program = "construction" + pySuffix
		args = []string{
			"--maintain_xlsx_file", filepath.Join(goAppWorkDir, req.Maintain.MaintainXlsxFile),
			"--after_root_dir", filepath.Join(goAppWorkDir, req.Maintain.AfterRootDir),
			"--before_root_dir", filepath.Join(goAppWorkDir, req.Maintain.BeforeRootDir),
			"--project_name", req.Maintain.ProjectName,
		}
	case ReportTypeRural:
		program = "rural" + pySuffix
		args = []string{
			"--nc_base_dir", filepath.Join(goAppWorkDir, req.Rural.NcBaseDir),
			"--unit_xlsx", filepath.Join(goAppWorkDir, req.Rural.UnitXlsx),
			"--root_dir", filepath.Join(goAppWorkDir, req.Rural.RootDir),
			"--xlsx_file", filepath.Join(goAppWorkDir, req.Rural.XlsxFile),
			"--gy_value", req.Rural.GyValue,
			"--pqi_wd1", fmt.Sprintf("%.3f", req.Rural.PqiWd1),
			"--pqi_12", fmt.Sprintf("%.3f", req.Rural.Pqi12),
			"--pqi_34", fmt.Sprintf("%.3f", req.Rural.Pqi34),
		}
	case ReportTypeNationalProvincial:
		program = "national_provincial" + pySuffix
		args = []string{
			"--root_path", filepath.Join(goAppWorkDir, req.NationalProvince.RootPath),
			"--xlsx_file", filepath.Join(goAppWorkDir, req.NationalProvince.XlsxFile),
			"--bitumen_folder_path", filepath.Join(goAppWorkDir, req.NationalProvince.BitumenFolderPath),
			"--CICScardata", filepath.Join(goAppWorkDir, req.NationalProvince.CICScardata),
			"--unit_path", filepath.Join(goAppWorkDir, req.NationalProvince.UnitPath),
			"--file_path", filepath.Join(goAppWorkDir, req.NationalProvince.FilePath),
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

	// pys 目录
	absPysDir := filepath.Join(goAppWorkDir, pyDir, program) // 确保路径正确
	logger.Logger.Infof("将要执行的程序绝对路径: %s", absPysDir)

	// 2. 获取执行前的目录列表
	dirsBefore, err := listDirs(reportsBaseDir)
	if err != nil {
		logger.Logger.Errorf("无法列出目录 [%s] (执行前): %v", reportsBaseDir, err)
		return nil, "", fmt.Errorf("无法列出目录 (执行前): %w", err)
	}
	logger.Logger.Debugf("执行前目录列表: %v", dirsBefore)

	// 3. 执行 Python 脚本
	cmd := exec.Command(absPysDir, args...)
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
	logger.Logger.Infof("扫描基础报告目录 [%s] (执行后)...", reportsBaseDir)
	dirsAfter, err := listDirs(reportsBaseDir)
	if err != nil {
		logger.Logger.Errorf("无法列出目录 [%s] (执行后): %v", reportsBaseDir, err)
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
		logger.Logger.Error("Python脚本执行成功，但在基础报告目录 [%s] 中未找到新生成的报告子目录。", reportsBaseDir)
		return nil, "", errors.New("未找到新生成的报告目录")
	}
	if len(foundNewDirs) > 1 {
		// 如果发现多个新目录，可能意味着并发问题或脚本行为超出预期。
		logger.Logger.Warnf("在基础报告目录 [%s] 中发现多个新生成的报告子目录: %v。将使用第一个: %s", reportsBaseDir, foundNewDirs, foundNewDirs[0])
	}
	newReportDirName = foundNewDirs[0]
	logger.Logger.Infof("检测到新生成的报告目录名: %s", newReportDirName)

	// 6. 构建 output.json 的完整路径并读取内容
	jsonResultFilePath := filepath.Join(reportsBaseDir, newReportDirName, "output.json")
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

	return data, filepath.Join(reportsBaseDir, newReportDirName), nil
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

	// --- 否则，生成Markdown表格 ---
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

func processExcelFile(filePath string) (string, error) {
	// --- 1. 打开并读取Excel文件 ---
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return "", fmt.Errorf("无法打开Excel文件 '%s': %w", filePath, err)
	}
	// 注意：这里的 defer close 是一个保障，但在后续的文件重命名操作前，我们会手动关闭它。
	defer func() {
		if err = f.Close(); err != nil {
			log.Printf("警告：关闭Excel文件 '%s' 失败: %v", filePath, err)
		}
	}()

	// --- 2. 获取工作表和行数据 ---
	sheetList := f.GetSheetList()
	if len(sheetList) == 0 {
		return "", fmt.Errorf("excel文件 '%s' 中没有工作表", filePath)
	}
	sheetName := sheetList[0] // 仅处理第一个工作表

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return "", fmt.Errorf("无法从工作表 '%s' (文件 '%s') 读取行数据: %w", sheetName, filePath, err)
	}

	if len(rows) == 0 {
		log.Printf("信息：Excel文件 '%s' 的工作表 '%s' 为空。", filePath, sheetName)
		return "", nil // 空工作表，直接返回
	}

	header := rows[0]
	if len(header) == 0 {
		log.Printf("信息：Excel文件 '%s' 的工作表 '%s' 表头行为空。", filePath, sheetName)
		return "", nil // 空表头，直接返回
	}

	// --- 3. 解析表头，识别需显示的列、高亮逻辑和特殊列（如“加权”） ---
	var displayHeaderNames []string
	var displayColOriginalIndices []int
	highlightLogic := make(map[int]int)
	hasWeightedColumn := false
	weightedColumnDisplayIndex := -1
	weightedColumnOriginalExcelIndex := -1

	for i, cellName := range header {
		trimmedCellName := strings.TrimSpace(cellName)
		if strings.HasSuffix(trimmedCellName, "_合格") || strings.HasSuffix(trimmedCellName, "_Qualified") {
			if i > 0 {
				highlightLogic[i-1] = i
			}
		} else {
			displayHeaderNames = append(displayHeaderNames, trimmedCellName)
			displayColOriginalIndices = append(displayColOriginalIndices, i)
			if strings.Contains(trimmedCellName, "加权") {
				hasWeightedColumn = true
				weightedColumnDisplayIndex = len(displayHeaderNames) - 1
				weightedColumnOriginalExcelIndex = i
			}
		}
	}

	if len(displayHeaderNames) == 0 {
		log.Printf("信息：解析后，Excel文件 '%s' 没有可显示的列。", filePath)
		return "", nil
	}

	if hasWeightedColumn && (len(rows)-1 < 2) {
		log.Printf("信息：文件 '%s' 的'加权'列数据行数少于2行 (%d)，不执行单元格合并。", filePath, len(rows)-1)
		hasWeightedColumn = false
	}

	// --- 4. 生成Markdown或HTML字符串 (此任务对所有文件都执行) ---
	var resultString string
	if hasWeightedColumn {
		log.Printf("信息：文件 '%s' 检测到 '加权' 列，将生成 HTML 表格。", filePath)
		resultString = generateHtmlTable(rows, displayHeaderNames, displayColOriginalIndices, highlightLogic, weightedColumnDisplayIndex, weightedColumnOriginalExcelIndex)
	} else {
		log.Printf("信息：文件 '%s' 未检测到 '加权' 列或不满足合并条件，将生成 Markdown 表格。", filePath)
		resultString = generateMarkdownTable(rows, displayHeaderNames, displayColOriginalIndices, highlightLogic)
	}

	// --- 5. 检查是否为目标文件，如果是，则执行额外的文件创建和替换任务 ---
	fileName := filepath.Base(filePath)
	if utils.Contains(targetExcelFiles, fileName) {
		log.Printf("信息：文件 '%s' 是目标文件，将额外生成新的带样式Excel文件进行替换。", filePath)

		// 在进行文件操作前，先关闭已打开的文件句柄
		if err = f.Close(); err != nil {
			// 即便关闭失败也只记录日志，因为后续操作可能仍需进行
			log.Printf("警告：在创建新Excel前关闭原始文件 '%s' 失败: %v", filePath, err)
		}

		// 调用辅助函数来完成文件创建、保存和重命名
		err = createNewStyledExcelAndRename(filePath, rows, displayHeaderNames, displayColOriginalIndices, highlightLogic, hasWeightedColumn, weightedColumnDisplayIndex, weightedColumnOriginalExcelIndex)
		if err != nil {
			// 如果创建新Excel文件失败，将错误返回，同时返回已经生成好的Markdown/HTML字符串
			return resultString, fmt.Errorf("为目标文件 '%s' 创建新Excel时失败: %w", fileName, err)
		}
		log.Printf("信息：已成功为目标文件 '%s' 创建并替换了新的Excel文件。", fileName)
	}

	// --- 6. 返回最终结果 ---
	return resultString, nil
}

// 封装了创建、样式化、保存和重命名新Excel文件的完整逻辑。
func createNewStyledExcelAndRename(
	originalFilePath string,
	rows [][]string,
	displayHeaderNames []string,
	displayColOriginalIndices []int,
	highlightLogic map[int]int,
	hasWeightedColumn bool,
	weightedColumnDisplayIndex int,
	weightedColumnOriginalExcelIndex int,
) error {

	// 创建一个新的Excel工作簿
	newExcel := excelize.NewFile()
	newSheetName := "处理结果"
	//_ = newExcel.SetSheetName("Sheet1", newSheetName)
	index, err := newExcel.NewSheet(newSheetName)
	if err != nil {
		return fmt.Errorf("为新Excel创建工作表 '%s' 失败: %w", newSheetName, err)
	}
	newExcel.SetActiveSheet(index)
	_ = newExcel.DeleteSheet("Sheet1")

	// 定义样式
	redStyle, err := newExcel.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#D32F2F"}, Pattern: 1},
		Font:      &excelize.Font{Color: "FFFFFF"},
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "center", WrapText: true},
	})
	if err != nil {
		return fmt.Errorf("创建红色Excel样式失败: %w", err)
	}
	normalStyle, err := newExcel.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center", Horizontal: "center", WrapText: true},
	})
	if err != nil {
		return fmt.Errorf("创建普通Excel样式失败: %w", err)
	}

	// 写入表头
	for i, name := range displayHeaderNames {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = newExcel.SetCellValue(newSheetName, cell, name)
	}

	// 写入数据行
	for r := 1; r < len(rows); r++ {
		excelRow := rows[r]
		newRowIndex := r + 1
		for cIdx, dataColOriginalExcelIdx := range displayColOriginalIndices {
			cellValue := ""
			if dataColOriginalExcelIdx < len(excelRow) {
				cellValue = excelRow[dataColOriginalExcelIdx]
			}
			cellValue = strings.TrimSpace(whitespaceRegex.ReplaceAllString(cellValue, ","))
			processedCellContent := strings.ReplaceAll(strings.Trim(cellValue, ","), ",", "\n")

			applyRedBackground := checkHighlight(excelRow, dataColOriginalExcelIdx, highlightLogic)

			cellName, _ := excelize.CoordinatesToCellName(cIdx+1, newRowIndex)
			_ = newExcel.SetCellValue(newSheetName, cellName, processedCellContent)

			if applyRedBackground {
				_ = newExcel.SetCellStyle(newSheetName, cellName, cellName, redStyle)
			} else {
				_ = newExcel.SetCellStyle(newSheetName, cellName, cellName, normalStyle)
			}
		}
	}

	// 合并“加权”列
	if hasWeightedColumn {
		numDataRows := len(rows) - 1
		if numDataRows > 0 {
			startCell, _ := excelize.CoordinatesToCellName(weightedColumnDisplayIndex+1, 2)
			endCell, _ := excelize.CoordinatesToCellName(weightedColumnDisplayIndex+1, numDataRows+1)
			if err = newExcel.MergeCell(newSheetName, startCell, endCell); err != nil {
				log.Printf("警告：合并“加权”列单元格失败: %v", err)
			}
			if weightedColumnOriginalExcelIndex < len(rows[1]) {
				_ = newExcel.SetCellValue(newSheetName, startCell, strings.TrimSpace(rows[1][weightedColumnOriginalExcelIndex]))
			}
		}
	}

	ext := filepath.Ext(originalFilePath)
	baseName := strings.TrimSuffix(originalFilePath, ext) // path/to/all_disease
	tempFilePath := baseName + "_tmp" + ext
	if err = newExcel.SaveAs(tempFilePath); err != nil {
		return fmt.Errorf("无法保存临时的Excel文件 '%s': %w", tempFilePath, err)
	}
	originBackupPath := baseName + "_origin" + ext // path/to/all_disease_origin.xlsx

	if err = os.Rename(originalFilePath, originBackupPath); err != nil {
		_ = os.Remove(tempFilePath) // 清理临时文件
		return fmt.Errorf("无法将原始文件重命名为 '%s': %w", originBackupPath, err)
	}

	if err = os.Rename(tempFilePath, originalFilePath); err != nil {
		// 尝试恢复原始文件名，以减少破坏性
		_ = os.Rename(originBackupPath, originalFilePath)
		return fmt.Errorf("无法将临时文件重命名为 '%s': %w。已尝试恢复原始文件名。", originalFilePath, err)
	}

	return nil
}

// 是一个辅助函数，用于生成HTML格式的表格字符串。
func generateHtmlTable(rows [][]string, displayHeaderNames []string, displayColOriginalIndices []int, highlightLogic map[int]int, weightedColumnDisplayIndex int, weightedColumnOriginalExcelIndex int) string {
	var htmlTable strings.Builder
	htmlTable.WriteString("<table>\n")
	htmlTable.WriteString("  <thead>\n    <tr>\n")
	for _, name := range displayHeaderNames {
		htmlTable.WriteString("      <th>" + html.EscapeString(name) + "</th>\n")
	}
	htmlTable.WriteString("    </tr>\n  </thead>\n")
	htmlTable.WriteString("  <tbody>\n")
	var weightedValueProcessed string
	if len(rows) > 1 && weightedColumnOriginalExcelIndex < len(rows[1]) {
		rawValue := strings.TrimSpace(rows[1][weightedColumnOriginalExcelIndex])
		processed := strings.Trim(whitespaceRegex.ReplaceAllString(rawValue, ","), ",")
		weightedValueProcessed = html.EscapeString(strings.ReplaceAll(processed, "|", "\\|"))
	}

	for r := 1; r < len(rows); r++ {
		excelRow := rows[r]
		htmlTable.WriteString("    <tr>\n")
		for cIdx, dataColOriginalExcelIdx := range displayColOriginalIndices {
			cellValue := ""
			if dataColOriginalExcelIdx < len(excelRow) {
				cellValue = excelRow[dataColOriginalExcelIdx]
			}
			processedCellContent := processCellContentForDisplay(cellValue, true)
			style := ""
			if checkHighlight(excelRow, dataColOriginalExcelIdx, highlightLogic) {
				style = ` style="background-color: #D32F2F; color: white; padding: 2px 4px;"`
			}
			if cIdx == weightedColumnDisplayIndex {
				if r == 1 {
					htmlTable.WriteString(fmt.Sprintf(`      <td rowspan="%d"%s>%s</td>%s`, len(rows)-1, style, weightedValueProcessed, "\n"))
				}
			} else {
				htmlTable.WriteString(fmt.Sprintf("      <td%s>%s</td>\n", style, processedCellContent))
			}
		}
		htmlTable.WriteString("    </tr>\n")
	}
	htmlTable.WriteString("  </tbody>\n</table>\n")
	return htmlTable.String()
}

// 用于生成Markdown格式的表格字符串。
func generateMarkdownTable(rows [][]string, displayHeaderNames []string, displayColOriginalIndices []int, highlightLogic map[int]int) string {
	var mdTable strings.Builder
	mdTable.WriteString("|")
	for _, name := range displayHeaderNames {
		mdTable.WriteString(" " + strings.ReplaceAll(name, "|", "\\|") + " |")
	}
	mdTable.WriteString("\n|")
	for range displayHeaderNames {
		mdTable.WriteString("---|")
	}
	mdTable.WriteString("\n")

	for r := 1; r < len(rows); r++ {
		excelRow := rows[r]
		mdTable.WriteString("|")
		for _, dataColIdx := range displayColOriginalIndices {
			cellValue := ""
			if dataColIdx < len(excelRow) {
				cellValue = excelRow[dataColIdx]
			}
			processedCellContent := processCellContentForDisplay(cellValue, false)
			if checkHighlight(excelRow, dataColIdx, highlightLogic) {
				mdTable.WriteString(fmt.Sprintf(` <span style="background-color: #D32F2F; color: white; padding: 2px 4px;">%s</span> |`, processedCellContent))
			} else {
				mdTable.WriteString(" " + processedCellContent + " |")
			}
		}
		mdTable.WriteString("\n")
	}
	return mdTable.String()
}

// 清理和格式化单元格内容，用于最终显示。
func processCellContentForDisplay(cellValue string, isHTML bool) string {
	cellValue = strings.TrimSpace(cellValue)
	if cellValue == "" {
		return ""
	}
	cellValue = strings.Trim(whitespaceRegex.ReplaceAllString(cellValue, ","), ",")
	if strings.Contains(cellValue, ",") {
		parts := strings.Split(cellValue, ",")
		var lines []string
		for i := 0; i < len(parts); i += 6 {
			end := i + 6
			if end > len(parts) {
				end = len(parts)
			}
			chunk := parts[i:end]
			var sanitizedChunk []string
			for _, p := range chunk {
				if trimmedPart := strings.TrimSpace(p); trimmedPart != "" {
					sanitizedPart := strings.ReplaceAll(trimmedPart, "|", "\\|")
					if isHTML {
						sanitizedPart = html.EscapeString(sanitizedPart)
					}
					sanitizedChunk = append(sanitizedChunk, sanitizedPart)
				}
			}
			if len(sanitizedChunk) > 0 {
				lines = append(lines, strings.Join(sanitizedChunk, ","))
			}
		}
		return strings.Join(lines, "<br>")
	}
	processed := strings.ReplaceAll(cellValue, "|", "\\|")
	if isHTML {
		processed = html.EscapeString(processed)
	}
	return processed
}

// 检查给定单元格是否需要高亮。
func checkHighlight(excelRow []string, dataColIdx int, highlightLogic map[int]int) bool {
	if qualifierColIdx, needsCheck := highlightLogic[dataColIdx]; needsCheck {
		if qualifierColIdx < len(excelRow) {
			if strings.ToUpper(strings.TrimSpace(excelRow[qualifierColIdx])) == "FALSE" {
				return true
			}
		}
	}
	return false
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
