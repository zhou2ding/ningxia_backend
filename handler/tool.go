package handler

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io"
	"log"
	"ningxia_backend/pkg/logger"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

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

func calculate(pySuffix, reportType string, files []string, pqi, mileage float64) (map[string]any, string, error) {
	var (
		program        string
		jsonResultFile string
		reportDirName  string
	)
	switch reportType {
	case ReportTypeExpressway:
		program = "expressway" + pySuffix
		jsonResultFile = expresswayReportBaseDir + "/output.json"
		reportDirName = expresswayReportBaseDir
	case ReportTypeMaintenance:
		program = "maintenance" + pySuffix
		jsonResultFile = maintenanceReportBaseDir + "/output.json"
		reportDirName = maintenanceReportBaseDir
	case ReportTypeConstruction:
		program = "construction" + pySuffix
		jsonResultFile = constructionReportBaseDir + "/output.json"
		reportDirName = constructionReportBaseDir
	case ReportTypeRural:
		program = "rural" + pySuffix
		jsonResultFile = ruralReportBaseDir + "/output.json"
		reportDirName = ruralReportBaseDir
	case ReportTypeNationalProvincial:
		program = "national_provincial" + pySuffix
		jsonResultFile = nationalProvinceReportBaseDir + "/output.json"
		reportDirName = nationalProvinceReportBaseDir
	default:
		return nil, "", errors.New("不支持的报告类型")
	}

	logger.Logger.Infof("python exe: %s", program)
	//args := []string{
	//	"-files", strings.Join(files, " "),
	//	"-pqi", fmt.Sprintf("%.2f", pqi),
	//	"-d", fmt.Sprintf("%.2f", mileage),
	//}

	//cmd := exec.Command(program, args...)
	//logger.Logger.Infof("execute program: %v", cmd)
	//output, err := cmd.CombinedOutput()
	//if err != nil {
	//	logger.Logger.Errorf("Python执行失败 [%d]: %s\n输出: %s", cmd.ProcessState.ExitCode(), err, output)
	//	return nil, err
	//}
	var data map[string]any
	//if err = json.Unmarshal(output, &data); err != nil {
	//	logger.Logger.Errorf("解析结果失败: %v\n原始输出: %s", err, output)
	//	return nil, err
	//}

	js, err := os.ReadFile(jsonResultFile)
	if err != nil {
		logger.Logger.Errorf("读取 %s 失败: %v", jsonResultFile, err)
		return nil, "", err
	}
	if err = json.Unmarshal(js, &data); err != nil {
		logger.Logger.Errorf("解析结果失败: %v", err)
		return nil, "", err
	}
	return data, reportDirName, nil
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
		return "", fmt.Errorf("Excel文件 '%s' 中没有工作表", filePath)
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
		if strings.HasSuffix(trimmedCellName, "_合格") {
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
			processedCellContent := ""
			if strings.Contains(cellValue, ",") {
				parts := strings.Split(cellValue, ",")
				var lines []string
				elementsPerLine := 10 // 每15个元素换行

				for i := 0; i < len(parts); i += elementsPerLine {
					end := i + elementsPerLine
					if end > len(parts) {
						end = len(parts)
					}
					chunk := parts[i:end]
					// Sanitize each part in the chunk for pipes and trim spaces (again, as split might re-introduce them)
					for j, p := range chunk {
						chunk[j] = strings.ReplaceAll(strings.TrimSpace(p), "|", "\\|")
					}
					lines = append(lines, strings.Join(chunk, ",")) // Join parts in a chunk with comma
				}
				processedCellContent = strings.Join(lines, ",<br>") // Join lines with ",<br>" to retain comma before break
				// If the last line segment ends with a comma because it was exactly at a multiple of 15,
				// and then ",<br>" is added, it might result in ",,<br>".
				// Let's refine: only add <br> between groups.
				processedCellContent = strings.Join(lines, "<br>") // Join lines with <br>
			} else {
				// No commas, just sanitize for pipes
				processedCellContent = strings.ReplaceAll(cellValue, "|", "\\|")
			}
			// ------ END OF MODIFIED LOGIC ------

			applyRedBackground := false
			// Check if this data column has a corresponding qualifier column for highlighting
			if qualifierColIdx, needsHighlightCheck := highlightLogic[dataColIdx]; needsHighlightCheck {
				if qualifierColIdx < len(excelRow) {
					qualifierValue := strings.ToUpper(strings.TrimSpace(excelRow[qualifierColIdx]))
					if qualifierValue == "TRUE" { // Assuming "TRUE" (case-insensitive) means highlight
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
		return "", fmt.Errorf("Excel文件 '%s' 中没有工作表", filePath)
	}
	sheetName := sheetList[0]

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return "", fmt.Errorf("无法从工作表 '%s' (文件 '%s') 读取行数据: %w", sheetName, filePath, err)
	}

	if len(rows) == 0 {
		logger.Logger.Infof("Excel文件 '%s' 的工作表 '%s' 为空。", filePath, sheetName)
		return "", nil
	}

	header := rows[0]
	if len(header) == 0 {
		logger.Logger.Infof("Excel文件 '%s' 的工作表 '%s' 表头行为空。", filePath, sheetName)
		return "", nil
	}

	var mdTable strings.Builder
	var displayHeaderNames []string     // Column names to display in Markdown
	var displayColOriginalIndices []int // Original Excel column indices for the display columns
	highlightLogic := make(map[int]int) // Key: index of data column to potentially highlight, Value: index of its corresponding "_合格" column

	// 1. Parse header to identify columns to display and columns for highlight logic
	for i, cellName := range header {
		if strings.HasSuffix(cellName, "_合格") {
			// This is a qualifier column. It will be hidden.
			// Its left neighbor (if exists) is the data column.
			if i > 0 {
				// The column at index i-1 is the data column.
				// The column at index i is its qualifier.
				highlightLogic[i-1] = i
			}
		} else {
			// This is not a qualifier column. It should be displayed.
			displayHeaderNames = append(displayHeaderNames, cellName)
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
	for r := 1; r < len(rows); r++ { // Start from the second row (data)
		excelRow := rows[r]
		mdTable.WriteString("|")

		for _, dataColIdx := range displayColOriginalIndices { // Iterate through columns that should be displayed
			cellValue := ""
			if dataColIdx < len(excelRow) {
				cellValue = excelRow[dataColIdx]
			}

			applyRedBackground := false
			// Check if this data column has a corresponding qualifier column for highlighting
			if qualifierColIdx, needsHighlightCheck := highlightLogic[dataColIdx]; needsHighlightCheck {
				if qualifierColIdx < len(excelRow) {
					qualifierValue := strings.ToUpper(strings.TrimSpace(excelRow[qualifierColIdx]))
					if qualifierValue == "TRUE" {
						applyRedBackground = true
					}
				}
			}

			sanitizedCellValue := strings.ReplaceAll(cellValue, "|", "\\|")

			if applyRedBackground {
				cellValueFormatted := fmt.Sprintf(`<span style="background-color: #D32F2F; color: white; padding: 2px 4px;">%s</span>`, sanitizedCellValue)
				mdTable.WriteString(" ")
				mdTable.WriteString(cellValueFormatted)
				mdTable.WriteString(" |")
			} else {
				mdTable.WriteString(" ")
				mdTable.WriteString(sanitizedCellValue)
				mdTable.WriteString(" |")
			}
		}
		mdTable.WriteString("\n")
	}

	return mdTable.String(), nil
}
