package main

// import (
// 	"errors"
// 	"fmt"
// 	"strconv"
// 	"strings"
// 	"time"
// )

// // Record 表示一条预处理后的轨迹记录
// type Record struct {
// 	ID        string    // 车辆唯一标识，可用“ID”或“设备号”
// 	Device    string    // 可选：设备号
// 	Timestamp time.Time // 定位时间，转换后的 time.Time
// 	Lon       float64   // 经度
// 	Lat       float64   // 纬度
// 	Direction float64   // 方向，度数
// 	Speed     float64   // 速度
// }

// // tryParseLocation 尝试将“实时定位”字段解析为经度、纬度
// func tryParseLocation(s string) (lon, lat float64, err error) {
// 	if s == "" {
// 		return 0, 0, errors.New("location empty")
// 	}
// 	ss := strings.TrimSpace(s)
// 	var parts []string
// 	if strings.Contains(ss, "，") {
// 		parts = strings.Split(ss, "，")
// 	} else if strings.Contains(ss, ",") {
// 		parts = strings.Split(ss, ",")
// 	} else {
// 		return 0, 0, fmt.Errorf("无法解析定位格式: %s", s)
// 	}
// 	if len(parts) < 2 {
// 		return 0, 0, fmt.Errorf("定位字段分割后长度不足: %v", parts)
// 	}
// 	lonStr := strings.TrimSpace(parts[0])
// 	latStr := strings.TrimSpace(parts[1])
// 	lon, err = strconv.ParseFloat(lonStr, 64)
// 	if err != nil {
// 		return 0, 0, fmt.Errorf("解析经度失败: %v", err)
// 	}
// 	lat, err = strconv.ParseFloat(latStr, 64)
// 	if err != nil {
// 		return 0, 0, fmt.Errorf("解析纬度失败: %v", err)
// 	}
// 	return lon, lat, nil
// }

// // tryParseTimestamp 尝试解析“定位时间”字段为 time.Time
// func tryParseTimestamp(s string) (time.Time, error) {
// 	ss := strings.TrimSpace(s)
// 	if ss == "" {
// 		return time.Time{}, errors.New("timestamp empty")
// 	}
// 	// 尝试作为 UNIX 时间戳（秒或毫秒）
// 	if f, err := strconv.ParseFloat(ss, 64); err == nil {
// 		var t time.Time
// 		if f > 1e12 {
// 			// 可能是毫秒
// 			sec := int64(f) / 1000
// 			msec := int64(f) - sec*1000
// 			t = time.Unix(sec, msec*int64(time.Millisecond)).UTC()
// 		} else {
// 			// 作为秒
// 			sec := int64(f)
// 			frac := f - float64(sec)
// 			nsec := int64(frac * 1e9)
// 			t = time.Unix(sec, nsec).UTC()
// 		}
// 		return t, nil
// 	}
// 	// 尝试常见时间格式
// 	layouts := []string{
// 		"2006-01-02 15:04:05",
// 		"2006/01/02 15:04:05",
// 		time.RFC3339,
// 		"2006-01-02T15:04:05Z07:00",
// 		"2006-01-02 15:04:05.000",
// 	}
// 	for _, layout := range layouts {
// 		if t, err := time.Parse(layout, ss); err == nil {
// 			return t, nil
// 		}
// 	}
// 	return time.Time{}, fmt.Errorf("无法解析时间格式: %s", s)
// }

// func main() {
// 	// 假设 CSV 文件在当前目录，且文件名如下
// 	fileName := "车辆历史轨迹地理位置信息20250616.csv"
// 	// 打开文件
// 	f, err := os.Open(fileName)
// 	if err != nil {
// 		fmt.Printf("打开 CSV 文件失败: %v\n", err)
// 		return
// 	}
// 	defer f.Close()

// 	// 假设文件已是 UTF-8 编码，直接读取
// 	reader := bufio.NewReader(f)
// 	csvReader := csv.NewReader(reader)
// 	csvReader.FieldsPerRecord = -1 // 允许可变列数

// 	// 读取表头
// 	header, err := csvReader.Read()
// 	if err != nil {
// 		fmt.Printf("读取 CSV 表头失败: %v\n", err)
// 		return
// 	}
// 	// 处理并打印表头，去除可能的 BOM 和前后空白
// 	normalizedHeader := make([]string, len(header))
// 	fmt.Println("原始 CSV 表头列:")
// 	for i, col := range header {
// 		// 去除 BOM (U+FEFF) 及空白
// 		colClean := strings.TrimSpace(strings.Trim(col, "\uFEFF"))
// 		normalizedHeader[i] = colClean
// 		fmt.Printf("  列 %d: '%s'\n", i, colClean)
// 	}

// 	// 构建映射：原始索引 -> 规范列名；同时构建小写映射 for 识别英文列
// 	colIndex := map[string]int{}
// 	lowerIndex := map[string]int{}
// 	for i, name := range normalizedHeader {
// 		colIndex[name] = i
// 		lowerIndex[strings.ToLower(name)] = i
// 	}

// 	// 检查常见关键列名
// 	needCols := []string{"ID", "设备号", "定位时间", "实时定位"}
// 	for _, col := range needCols {
// 		if _, ok := colIndex[col]; !ok {
// 			fmt.Printf("警告：未找到列名 '%s'\n", col)
// 		}
// 	}
// 	// 检查是否存在独立的经度/纬度列
// 	hasLonLat := false
// 	lonIdx, latIdx := -1, -1
// 	// 常见中文列名“经度”、“纬度”；也可能是英文“lon”、“lat”、“longitude”、“latitude”
// 	for key, idx := range lowerIndex {
// 		switch key {
// 		case "经度", "lon", "longitude":
// 			lonIdx = idx
// 		case "纬度", "lat", "latitude":
// 			latIdx = idx
// 		}
// 	}
// 	if lonIdx >= 0 && latIdx >= 0 {
// 		hasLonLat = true
// 		fmt.Printf("检测到独立经纬度列：经度列索引=%d, 纬度列索引=%d\n", lonIdx, latIdx)
// 	} else {
// 		fmt.Println("未检测到独立经纬度列，将尝试解析“实时定位”列")
// 	}

// 	// 定位“实时定位”列索引（如存在）
// 	locIdx := -1
// 	if idx, ok := colIndex["实时定位"]; ok {
// 		locIdx = idx
// 	} else if idx, ok := lowerIndex["实时定位"]; ok {
// 		locIdx = idx
// 	}
// 	if locIdx >= 0 {
// 		fmt.Printf("“实时定位”列索引=%d\n", locIdx)
// 	} else {
// 		fmt.Println("未检测到“实时定位”列，若无独立经纬度列，无法解析经纬度")
// 	}

// 	// 定位“定位时间”列索引
// 	timeIdx := -1
// 	if idx, ok := colIndex["定位时间"]; ok {
// 		timeIdx = idx
// 	} else if idx, ok := lowerIndex["定位时间"]; ok {
// 		timeIdx = idx
// 	}
// 	if timeIdx >= 0 {
// 		fmt.Printf("“定位时间”列索引=%d\n", timeIdx)
// 	} else {
// 		fmt.Println("未检测到“定位时间”列，时间解析将失败")
// 	}

// 	// 定位“方向”、“速度”列索引
// 	dirIdx, spdIdx := -1, -1
// 	if idx, ok := colIndex["方向"]; ok {
// 		dirIdx = idx
// 	} else if idx, ok := lowerIndex["方向"]; ok {
// 		dirIdx = idx
// 	}
// 	if dirIdx >= 0 {
// 		fmt.Printf("“方向”列索引=%d\n", dirIdx)
// 	} else {
// 		fmt.Println("未检测到“方向”列")
// 	}
// 	if idx, ok := colIndex["速度"]; ok {
// 		spdIdx = idx
// 	} else if idx, ok := lowerIndex["速度"]; ok {
// 		spdIdx = idx
// 	}
// 	if spdIdx >= 0 {
// 		fmt.Printf("“速度”列索引=%d\n", spdIdx)
// 	} else {
// 		fmt.Println("未检测到“速度”列")
// 	}

// 	// 逐行读取并解析
// 	var records []Record
// 	lineNum := 1
// 	for {
// 		row, err := csvReader.Read()
// 		if err == io.EOF {
// 			break
// 		}
// 		lineNum++
// 		if err != nil {
// 			fmt.Printf("读取第 %d 行出错: %v\n", lineNum, err)
// 			continue
// 		}
// 		rec := Record{}
// 		// 解析 ID
// 		if idx, ok := colIndex["ID"]; ok && idx < len(row) {
// 			rec.ID = strings.TrimSpace(row[idx])
// 		} else if idx, ok := lowerIndex["id"]; ok && idx < len(row) {
// 			rec.ID = strings.TrimSpace(row[idx])
// 		}
// 		// 解析 设备号
// 		if idx, ok := colIndex["设备号"]; ok && idx < len(row) {
// 			rec.Device = strings.TrimSpace(row[idx])
// 		} else if idx, ok := lowerIndex["设备号"]; ok && idx < len(row) {
// 			rec.Device = strings.TrimSpace(row[idx])
// 		}

// 		// 解析 Timestamp
// 		if timeIdx >= 0 && timeIdx < len(row) {
// 			t, err := tryParseTimestamp(row[timeIdx])
// 			if err != nil {
// 				fmt.Printf("第 %d 行时间解析失败: %v\n", lineNum, err)
// 			} else {
// 				// 如需 Asia/Taipei 时区，可转换：
// 				// loc, _ := time.LoadLocation("Asia/Taipei")
// 				// rec.Timestamp = t.In(loc)
// 				rec.Timestamp = t
// 			}
// 		}

// 		// 解析经纬度
// 		if hasLonLat && lonIdx < len(row) && latIdx < len(row) {
// 			// 从独立列读取
// 			lonVal := strings.TrimSpace(row[lonIdx])
// 			latVal := strings.TrimSpace(row[latIdx])
// 			if lonVal != "" && latVal != "" {
// 				if lonF, err := strconv.ParseFloat(lonVal, 64); err == nil {
// 					rec.Lon = lonF
// 				} else {
// 					fmt.Printf("第 %d 行经度解析失败: %v\n", lineNum, err)
// 				}
// 				if latF, err := strconv.ParseFloat(latVal, 64); err == nil {
// 					rec.Lat = latF
// 				} else {
// 					fmt.Printf("第 %d 行纬度解析失败: %v\n", lineNum, err)
// 				}
// 			} else {
// 				fmt.Printf("第 %d 行经纬度列值为空\n", lineNum)
// 			}
// 		} else if locIdx >= 0 && locIdx < len(row) {
// 			// 解析“实时定位”
// 			lon, lat, err := tryParseLocation(row[locIdx])
// 			if err != nil {
// 				fmt.Printf("第 %d 行实时定位解析失败: %v\n", lineNum, err)
// 			} else {
// 				rec.Lon = lon
// 				rec.Lat = lat
// 			}
// 		} else {
// 			// 无法解析经纬度
// 			fmt.Printf("第 %d 行无法获取经纬度: 未检测到经度/纬度列或实时定位列\n", lineNum)
// 		}

// 		// 解析方向
// 		if dirIdx >= 0 && dirIdx < len(row) {
// 			s := strings.TrimSpace(row[dirIdx])
// 			if s != "" {
// 				if v, err := strconv.ParseFloat(s, 64); err == nil {
// 					rec.Direction = v
// 				} else {
// 					fmt.Printf("第 %d 行方向解析失败: %v\n", lineNum, err)
// 				}
// 			}
// 		}
// 		// 解析速度
// 		if spdIdx >= 0 && spdIdx < len(row) {
// 			s := strings.TrimSpace(row[spdIdx])
// 			if s != "" {
// 				if v, err := strconv.ParseFloat(s, 64); err == nil {
// 					rec.Speed = v
// 				} else {
// 					fmt.Printf("第 %d 行速度解析失败: %v\n", lineNum, err)
// 				}
// 			}
// 		}

// 		if rec.ID == "" {
// 			fmt.Printf("第 %d 行缺少 ID，跳过\n", lineNum)
// 			continue
// 		}
// 		// 若经纬度依然为零值或未赋值，可根据业务决定是否跳过。示例不自动跳过，保留供后续检查：
// 		records = append(records, rec)
// 	}

// 	fmt.Printf("共读取并解析 %d 条有效记录\n", len(records))

// 	// 按车辆 ID 分组，并按时间排序
// 	grouped := make(map[string][]Record)
// 	for _, r := range records {
// 		grouped[r.ID] = append(grouped[r.ID], r)
// 	}
// 	for id, recs := range grouped {
// 		sort.Slice(recs, func(i, j int) bool {
// 			return recs[i].Timestamp.Before(recs[j].Timestamp)
// 		})
// 		grouped[id] = recs
// 	}
// 	fmt.Printf("按车辆 ID 分组完成，共 %d 组\n", len(grouped))

// 	// 将预处理结果写到新的 CSV，方便检查或后续处理
// 	outFileName := "processed_轨迹数据.csv"
// 	outFile, err := os.Create(outFileName)
// 	if err != nil {
// 		fmt.Printf("创建输出文件失败: %v\n", err)
// 		return
// 	}
// 	defer outFile.Close()
// 	writer := csv.NewWriter(outFile)
// 	// 写表头
// 	headerOut := []string{"ID", "设备号", "Timestamp", "Lon", "Lat", "Direction", "Speed"}
// 	if err := writer.Write(headerOut); err != nil {
// 		fmt.Printf("写出表头失败: %v\n", err)
// 	}
// 	// 写出记录
// 	for _, recs := range grouped {
// 		for _, r := range recs {
// 			tsStr := r.Timestamp.Format(time.RFC3339)
// 			row := []string{
// 				r.ID,
// 				r.Device,
// 				tsStr,
// 				fmt.Sprintf("%.6f", r.Lon),
// 				fmt.Sprintf("%.6f", r.Lat),
// 				fmt.Sprintf("%.3f", r.Direction),
// 				fmt.Sprintf("%.3f", r.Speed),
// 			}
// 			if err := writer.Write(row); err != nil {
// 				fmt.Printf("写出记录失败: %v\n", err)
// 			}
// 		}
// 	}
// 	writer.Flush()
// 	if err := writer.Error(); err != nil {
// 		fmt.Printf("CSV 写入遇到错误: %v\n", err)
// 	} else {
// 		fmt.Printf("预处理后文件已写至当前目录: %s\n", outFileName)
// 	}

// 	// 如有需要，可在此对 grouped 进行进一步检查或逻辑处理
// }
