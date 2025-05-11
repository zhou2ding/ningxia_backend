package handler

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"io"
	"log"
	"ningxia_backend/pkg/logger"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

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

func calculate(pySuffix, reportType string, files []string, pqi, mileage float64) (map[string]any, error) {
	var program string
	var jsonResultFile string
	switch reportType {
	case ReportTypeExpressway:
		program = "expressway" + pySuffix
		jsonResultFile = expresswayReportBaseDir + "/result.json"
	case ReportTypeMaintenance:
		program = "maintenance" + pySuffix
		jsonResultFile = maintenanceReportBaseDir + "/result.json"
	case ReportTypeConstruction:
		program = "construction" + pySuffix
		jsonResultFile = constructionReportBaseDir + "/result.json"
	case ReportTypeRural:
		program = "rural" + pySuffix
		jsonResultFile = ruralReportBaseDir + "/result.json"
	case ReportTypeNationalProvincial:
		program = "national_provincial" + pySuffix
		jsonResultFile = nationalProvinceReportBaseDir + "/result.json"
	//case ReportTypeMarket:
	//	program = "market" + pySuffix
	//	jsonResultFile = marketReportBaseDir + "/result.json"
	default:
		return nil, errors.New("不支持的报告类型")
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
		return nil, err
	}
	if err = json.Unmarshal(js, &data); err != nil {
		logger.Logger.Errorf("解析结果失败: %v", err)
		return nil, err
	}
	return data, nil
}

func extractTimestamp(filename string) int64 {
	lastUnderscore := strings.LastIndex(filename, "_")
	lastDot := strings.LastIndex(filename, ".")

	if lastUnderscore == -1 || lastDot == -1 || lastUnderscore >= lastDot-1 {
		return 0
	}

	timestampStr := filename[lastUnderscore+1 : lastDot]
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return 0
	}
	return timestamp
}
