package main

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Record 表示一条预处理后的轨迹记录
type Record struct {
	ID        string    // 车辆唯一标识，可用“ID”或“设备号”
	Device    string    // 设备号
	Timestamp time.Time // 定位时间，转换后的 time.Time
	Lon       float64   // 经度
	Lat       float64   // 纬度
	Direction float64   // 方向，度数
	Speed     float64   // 速度
}

// tryParseLocation 解析“实时定位”字段为经度、纬度
func tryParseLocation(s string) (lon, lat float64, err error) {
	if s == "" {
		return 0, 0, errors.New("location empty")
	}
	ss := strings.TrimSpace(s)
	var parts []string
	if strings.Contains(ss, "，") {
		parts = strings.Split(ss, "，")
	} else if strings.Contains(ss, ",") {
		parts = strings.Split(ss, ",")
	} else {
		return 0, 0, fmt.Errorf("无法解析定位格式: %s", s)
	}
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("定位字段分割后长度不足: %v", parts)
	}
	lonStr := strings.TrimSpace(parts[0])
	latStr := strings.TrimSpace(parts[1])
	lon, err = strconv.ParseFloat(lonStr, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("解析经度失败: %v", err)
	}
	lat, err = strconv.ParseFloat(latStr, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("解析纬度失败: %v", err)
	}
	return lon, lat, nil
}

// tryParseTimestamp 解析“定位时间”字段为 time.Time
func tryParseTimestamp(s string) (time.Time, error) {
	ss := strings.TrimSpace(s)
	if ss == "" {
		return time.Time{}, errors.New("timestamp empty")
	}
	// 尝试作为 UNIX 时间戳（秒或毫秒）
	if f, err := strconv.ParseFloat(ss, 64); err == nil {
		var t time.Time
		if f > 1e12 {
			sec := int64(f) / 1000
			msec := int64(f) - sec*1000
			t = time.Unix(sec, msec*int64(time.Millisecond)).UTC()
		} else {
			sec := int64(f)
			frac := f - float64(sec)
			nsec := int64(frac * 1e9)
			t = time.Unix(sec, nsec).UTC()
		}
		// 若需要 Asia/Taipei，可：loc, _ := time.LoadLocation("Asia/Taipei"); return t.In(loc), nil
		return t, nil
	}
	// 常见时间格式
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05.000",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, ss); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析时间格式: %s", s)
}

func main() {
	// 假设 CSV 在当前目录，文件名如下，请根据实际改名
	fileName := "车辆历史轨迹地理位置信息20250616.csv"

	// 打开原始 CSV
	f, err := os.Open(fileName)
	if err != nil {
		fmt.Printf("打开原始 CSV 失败: %v\n", err)
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1

	// 读取并打印表头，帮助确认列名
	header, err := csvReader.Read()
	if err != nil {
		fmt.Printf("读取 CSV 表头失败: %v\n", err)
		return
	}
	normalizedHeader := make([]string, len(header))
	fmt.Println("原始 CSV 表头:")
	for i, col := range header {
		colClean := strings.TrimSpace(strings.Trim(col, "\uFEFF"))
		normalizedHeader[i] = colClean
		fmt.Printf("  列 %d: '%s'\n", i, colClean)
	}
	// 构建列名映射
	colIndex := map[string]int{}
	lowerIndex := map[string]int{}
	for i, name := range normalizedHeader {
		colIndex[name] = i
		lowerIndex[strings.ToLower(name)] = i
	}
	// 检测独立经度/纬度列
	hasLonLat := false
	lonIdx, latIdx := -1, -1
	for key, idx := range lowerIndex {
		switch key {
		case "经度", "lon", "longitude":
			lonIdx = idx
		case "纬度", "lat", "latitude":
			latIdx = idx
		}
	}
	if lonIdx >= 0 && latIdx >= 0 {
		hasLonLat = true
		fmt.Printf("检测到经度/纬度列：idx=%d/%d\n", lonIdx, latIdx)
	} else {
		fmt.Println("未检测到独立经纬度列，将解析“实时定位”")
	}
	// 定位“实时定位”列
	locIdx := -1
	if idx, ok := colIndex["实时定位"]; ok {
		locIdx = idx
	} else if idx, ok := lowerIndex["实时定位"]; ok {
		locIdx = idx
	}
	if locIdx >= 0 {
		fmt.Printf("“实时定位”列索引=%d\n", locIdx)
	} else {
		fmt.Println("未检测到“实时定位”列")
	}
	// 定位“定位时间”列
	timeIdx := -1
	if idx, ok := colIndex["定位时间"]; ok {
		timeIdx = idx
	} else if idx, ok := lowerIndex["定位时间"]; ok {
		timeIdx = idx
	}
	if timeIdx >= 0 {
		fmt.Printf("“定位时间”列索引=%d\n", timeIdx)
	} else {
		fmt.Println("未检测到“定位时间”列")
	}
	// 方向、速度列
	dirIdx, spdIdx := -1, -1
	if idx, ok := colIndex["方向"]; ok {
		dirIdx = idx
	} else if idx, ok := lowerIndex["方向"]; ok {
		dirIdx = idx
	}
	if dirIdx >= 0 {
		fmt.Printf("“方向”列索引=%d\n", dirIdx)
	} else {
		fmt.Println("未检测到“方向”列")
	}
	if idx, ok := colIndex["速度"]; ok {
		spdIdx = idx
	} else if idx, ok := lowerIndex["速度"]; ok {
		spdIdx = idx
	}
	if spdIdx >= 0 {
		fmt.Printf("“速度”列索引=%d\n", spdIdx)
	} else {
		fmt.Println("未检测到“速度”列")
	}

	// 读取并解析
	var records []Record
	lineNum := 1
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		lineNum++
		if err != nil {
			fmt.Printf("第 %d 行读取失败: %v\n", lineNum, err)
			continue
		}
		rec := Record{}
		// ID
		if idx, ok := colIndex["ID"]; ok && idx < len(row) {
			rec.ID = strings.TrimSpace(row[idx])
		} else if idx, ok := lowerIndex["id"]; ok && idx < len(row) {
			rec.ID = strings.TrimSpace(row[idx])
		}
		// 设备号
		if idx, ok := colIndex["设备号"]; ok && idx < len(row) {
			rec.Device = strings.TrimSpace(row[idx])
		} else if idx, ok := lowerIndex["设备号"]; ok && idx < len(row) {
			rec.Device = strings.TrimSpace(row[idx])
		}
		// 时间
		if timeIdx >= 0 && timeIdx < len(row) {
			t, err := tryParseTimestamp(row[timeIdx])
			if err != nil {
				fmt.Printf("第 %d 行时间解析失败: %v\n", lineNum, err)
			} else {
				rec.Timestamp = t
			}
		}
		// 经纬度
		if hasLonLat && lonIdx < len(row) && latIdx < len(row) {
			lonVal := strings.TrimSpace(row[lonIdx])
			latVal := strings.TrimSpace(row[latIdx])
			if lonVal != "" && latVal != "" {
				if lonF, err := strconv.ParseFloat(lonVal, 64); err == nil {
					rec.Lon = lonF
				} else {
					fmt.Printf("第 %d 行经度解析失败: %v\n", lineNum, err)
				}
				if latF, err := strconv.ParseFloat(latVal, 64); err == nil {
					rec.Lat = latF
				} else {
					fmt.Printf("第 %d 行纬度解析失败: %v\n", lineNum, err)
				}
			} else {
				fmt.Printf("第 %d 行经纬度列为空\n", lineNum)
			}
		} else if locIdx >= 0 && locIdx < len(row) {
			lon, lat, err := tryParseLocation(row[locIdx])
			if err != nil {
				fmt.Printf("第 %d 行实时定位解析失败: %v\n", lineNum, err)
			} else {
				rec.Lon = lon
				rec.Lat = lat
			}
		} else {
			fmt.Printf("第 %d 行无法获取经纬度\n", lineNum)
		}
		// 方向
		if dirIdx >= 0 && dirIdx < len(row) {
			s := strings.TrimSpace(row[dirIdx])
			if s != "" {
				if v, err := strconv.ParseFloat(s, 64); err == nil {
					rec.Direction = v
				} else {
					fmt.Printf("第 %d 行方向解析失败: %v\n", lineNum, err)
				}
			}
		}
		// 速度
		if spdIdx >= 0 && spdIdx < len(row) {
			s := strings.TrimSpace(row[spdIdx])
			if s != "" {
				if v, err := strconv.ParseFloat(s, 64); err == nil {
					rec.Speed = v
				} else {
					fmt.Printf("第 %d 行速度解析失败: %v\n", lineNum, err)
				}
			}
		}
		if rec.ID == "" {
			fmt.Printf("第 %d 行缺少 ID，跳过\n", lineNum)
			continue
		}
		records = append(records, rec)
	}
	fmt.Printf("共解析 %d 条记录\n", len(records))

	// 分组排序
	grouped := make(map[string][]Record)
	for _, r := range records {
		grouped[r.ID] = append(grouped[r.ID], r)
	}
	for id, recs := range grouped {
		sort.Slice(recs, func(i, j int) bool {
			return recs[i].Timestamp.Before(recs[j].Timestamp)
		})
		grouped[id] = recs
	}
	fmt.Printf("按 ID 分组共 %d 组\n", len(grouped))

	// 写出 CSV，带 UTF-8 BOM，UseCRLF=true
	outName := "processed_轨迹数据_forExcel.csv"
	outFile, err := os.Create(outName)
	if err != nil {
		fmt.Printf("创建输出文件失败: %v\n", err)
		return
	}
	defer outFile.Close()

	// 写 UTF-8 BOM，确保 Excel 识别 UTF-8
	_, _ = outFile.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(outFile)
	writer.UseCRLF = true
	// 如果你的系统列表分隔符是逗号，一般保持默认；若需分号，可加以下一行：
	// writer.Comma = ';'

	// 写表头：注意列名不要包含逗号
	headerOut := []string{"ID", "设备号", "Timestamp", "Lon", "Lat", "Direction", "Speed"}
	if err := writer.Write(headerOut); err != nil {
		fmt.Printf("写表头失败: %v\n", err)
	}
	// 写记录
	for _, recs := range grouped {
		for _, r := range recs {
			// Format Timestamp 为常见格式，例如本地时区：
			// loc, _ := time.LoadLocation("Asia/Taipei")
			// tsStr := r.Timestamp.In(loc).Format("2006-01-02 15:04:05")
			tsStr := r.Timestamp.Format("2006-01-02 15:04:05") // 不带时区标识，Excel 识别为文本或日期
			row := []string{
				r.ID,
				r.Device,
				tsStr,
				fmt.Sprintf("%.6f", r.Lon),
				fmt.Sprintf("%.6f", r.Lat),
				fmt.Sprintf("%.3f", r.Direction),
				fmt.Sprintf("%.3f", r.Speed),
			}
			if err := writer.Write(row); err != nil {
				fmt.Printf("写行失败: %v\n", err)
			}
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		fmt.Printf("CSV 写入错误: %v\n", err)
	} else {
		fmt.Printf("已生成带 BOM 的 CSV 文件: %s，请用 Excel 打开或“数据->自文本/CSV”导入\n", outName)
	}
}
