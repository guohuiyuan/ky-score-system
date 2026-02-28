package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guohuiyuan/ky-score-system/config"
	"github.com/guohuiyuan/ky-score-system/models"
	"github.com/xuri/excelize/v2"
	"gorm.io/datatypes"
)

// AdminDownloadExcelTemplate 下载 Excel 导入模板
func AdminDownloadExcelTemplate(c *gin.Context) {
	// 获取动态配置
	var examConfig models.ExamConfig
	if err := config.DB.First(&examConfig).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取配置失败"})
		return
	}

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	// 基础列
	headers := []string{"报考专业", "ID(昵称)", "准考证后6位"}

	// 动态列
	var dynamicFields []map[string]interface{}
	json.Unmarshal(examConfig.Fields, &dynamicFields)
	for _, field := range dynamicFields {
		headers = append(headers, field["Label"].(string))
	}

	headers = append(headers, "总分")

	// 写入表头
	for i, head := range headers {
		cellName, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Sheet1", cellName, head)
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=ScoreImportTemplate.xlsx")
	c.Header("Content-Transfer-Encoding", "binary")

	if err := f.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成模板失败"})
	}
}

// AdminImportExcel 批量导入 Excel 数据
func AdminImportExcel(c *gin.Context) {
	file, err := c.FormFile("excel_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取文件失败"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法打开上传文件"})
		return
	}
	defer src.Close()

	f, err := excelize.OpenReader(src)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 Excel 文件"})
		return
	}
	defer f.Close()

	rows, err := f.GetRows("Sheet1")
	if err != nil || len(rows) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到有效数据，请检查是否包含表头及数据行"})
		return
	}

	// 准备配置
	var examConfig models.ExamConfig
	config.DB.First(&examConfig)
	var dynamicFields []map[string]interface{}
	json.Unmarshal(examConfig.Fields, &dynamicFields)
	var recoveryKeys []string
	json.Unmarshal(examConfig.KeyRecoveryFields, &recoveryKeys)

	// 获取已有的全部数据用于前置查重
	var allRecords []models.ScoreRecord
	config.DB.Find(&allRecords)

	successCount := 0
	skipCount := 0

	for i, row := range rows {
		if i == 0 {
			continue // 跳过表头
		}

		// 补齐空白单元格，避免越界
		for len(row) < len(dynamicFields)+4 {
			row = append(row, "")
		}

		major := row[0]
		if major == "" {
			major = "批量导入"
		}

		nickname := row[1]
		if nickname == "" {
			nickname = fmt.Sprintf("考生_行%d", i+1)
		}

		ticketPrefix := row[2]
		if ticketPrefix == "" {
			ticketPrefix = "000000"
		}

		totalStr := row[len(dynamicFields)+3]
		totalScore, _ := strconv.Atoi(totalStr)

		dynDataMap := make(map[string]interface{})
		isRowInvalid := false
		var calculatedTotal float64 = 0

		for j, field := range dynamicFields {
			val := row[j+3]
			fieldKey := field["Key"].(string)
			switch field["Type"] {
			case "number":
				if floatVal, err := strconv.ParseFloat(val, 64); err == nil {
					// 严格校验分数有效性
					if fieldKey == "politics" || fieldKey == "english" {
						if floatVal < 0 || floatVal > 100 {
							log.Printf(">> Excel 第 %d 行跳过：[%s] 分数越界: %v", i+1, fieldKey, floatVal)
							isRowInvalid = true
						}
					}
					if fieldKey == "math" || fieldKey == "subject_408" {
						if floatVal < 0 || floatVal > 150 {
							log.Printf(">> Excel 第 %d 行跳过：[%s] 分数越界: %v", i+1, fieldKey, floatVal)
							isRowInvalid = true
						}
					}
					dynDataMap[fieldKey] = floatVal
					calculatedTotal += floatVal
				} else {
					// 若必填四个科目为空或者非数字，视为无效
					if fieldKey == "politics" || fieldKey == "english" || fieldKey == "math" || fieldKey == "subject_408" {
						log.Printf(">> Excel 第 %d 行跳过：必填核心成绩 [%s] 为空或非数字 (当前值: '%v')", i+1, fieldKey, val)
						isRowInvalid = true
					}
				}
			default:
				dynDataMap[fieldKey] = val
			}
		}

		if isRowInvalid {
			skipCount++
			continue
		}

		// 自动推断总分
		if totalScore == 0 && calculatedTotal > 0 {
			totalScore = int(calculatedTotal)
		}

		// 全局四科查重防御
		isDuplicate := false
		for _, rec := range allRecords {
			var storedDyn map[string]interface{}
			json.Unmarshal(rec.DynamicData, &storedDyn)

			matchCount := 0
			for _, rk := range recoveryKeys {
				if storedVal, ok := storedDyn[rk].(float64); ok {
					if importedValFloat, ok := dynDataMap[rk].(float64); ok {
						if storedVal == importedValFloat {
							matchCount++
						}
					}
				} else if storedStr, ok := storedDyn[rk].(string); ok {
					if importedValStr, ok := dynDataMap[rk].(string); ok {
						if storedStr == importedValStr {
							matchCount++
						}
					}
				}
			}
			if len(recoveryKeys) > 0 && matchCount == len(recoveryKeys) {
				isDuplicate = true
				break
			}
		}

		if isDuplicate {
			log.Printf(">> Excel 第 %d 行跳过：触发全局四科核心成绩防撞车重载机制 (可能是重复的成绩行)", i+1)
			skipCount++
			continue // 发现4科核心重复，跳过本条
		}

		// 否则安全，生成独立新密钥，入库
		secretKey := uuid.New().String()

		dynBytes, _ := json.Marshal(dynDataMap)

		newRec := models.ScoreRecord{
			ExamID:       examConfig.ID,
			Major:        major,
			Nickname:     nickname,
			TicketPrefix: ticketPrefix,
			TotalScore:   totalScore,
			SecretKey:    secretKey,
			IPAddress:    "ExcelImport",
			Status:       "approved", // 管理员导入默认通过
			DynamicData:  datatypes.JSON(dynBytes),
		}

		if config.DB.Create(&newRec).Error == nil {
			allRecords = append(allRecords, newRec) // 加入全集，防止同批次内部重复
			successCount++
		} else {
			skipCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       fmt.Sprintf("导入完成！成功录入 %d 条记录，跳过/查重冲突 %d 条记录。", successCount, skipCount),
		"success_count": successCount,
		"skip_count":    skipCount,
	})
}

// AdminExportExcel 导出全部成绩为 Excel
func AdminExportExcel(c *gin.Context) {
	// 获取动态配置
	var examConfig models.ExamConfig
	if err := config.DB.First(&examConfig).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取配置失败"})
		return
	}

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	// 基础列（不包含截图）
	headers := []string{"时间", "报考专业", "ID(昵称)", "准考证后6位"}

	// 动态列
	var dynamicFields []map[string]interface{}
	json.Unmarshal(examConfig.Fields, &dynamicFields)
	for _, field := range dynamicFields {
		headers = append(headers, field["Label"].(string))
	}

	headers = append(headers, "总分", "密钥", "IP地址", "状态")

	// 写入表头
	for i, head := range headers {
		cellName, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Sheet1", cellName, head)
	}

	// 获取所有记录
	var records []models.ScoreRecord
	if err := config.DB.Order("created_at desc").Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取数据失败"})
		return
	}

	// 写入数据
	for rowIndex, rec := range records {
		var dynData map[string]interface{}
		json.Unmarshal(rec.DynamicData, &dynData)

		var rowData []interface{}
		rowData = append(rowData,
			rec.CreatedAt.Format("2006-01-02 15:04:05"),
			rec.Major,
			rec.Nickname,
			rec.TicketPrefix,
		)

		for _, field := range dynamicFields {
			fieldKey := field["Key"].(string)
			if val, ok := dynData[fieldKey]; ok {
				rowData = append(rowData, val)
			} else {
				rowData = append(rowData, "")
			}
		}

		statusText := "待核验"
		if rec.Status == "approved" {
			statusText = "已通过"
		} else if rec.Status == "rejected" {
			statusText = "已驳回"
		}

		rowData = append(rowData,
			rec.TotalScore,
			rec.SecretKey,
			rec.IPAddress,
			statusText,
		)

		for colIndex, cellValue := range rowData {
			cellName, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+2)
			f.SetCellValue("Sheet1", cellName, cellValue)
		}
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=ScoreExport.xlsx")
	c.Header("Content-Transfer-Encoding", "binary")

	if err := f.Write(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出数据失败"})
	}
}
