package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/guohuiyuan/ky-score-system/config"
	"github.com/guohuiyuan/ky-score-system/models"
	"gorm.io/datatypes"
)

// IndexPage 前台排行榜主页（支持后端筛选 + 重新排名）
func IndexPage(c *gin.Context) {
	// 获取用户提供的密钥
	secretKey, err := c.Cookie("user_secret_key")
	if err != nil || secretKey == "" {
		c.Redirect(http.StatusFound, "/ky/login")
		return
	}

	// 检查该密钥是否存在于数据库中
	var userRecord models.ScoreRecord
	if err := config.DB.Where("secret_key = ?", secretKey).First(&userRecord).Error; err != nil {
		c.SetCookie("user_secret_key", "", -1, "/", "", false, true)
		c.Redirect(http.StatusFound, "/ky/login")
		return
	}

	var allRecords []models.ScoreRecord
	config.DB.Where("status = ?", "approved").Order("total_score desc").Find(&allRecords)

	// 获取考试配置
	var examConfig models.ExamConfig
	config.DB.First(&examConfig)

	var dynamicFields []map[string]interface{}
	if len(examConfig.Fields) > 0 {
		json.Unmarshal(examConfig.Fields, &dynamicFields)
	}

	// 收集当前的筛选参数
	activeFilters := make(map[string]string)
	for _, field := range dynamicFields {
		if field["Type"] == "select" {
			key := field["Key"].(string)
			val := c.Query(key)
			if val != "" {
				activeFilters[key] = val
			}
		}
	}

	// 总分筛选参数
	minScoreStr := c.Query("min_score")
	minScore := 0
	if minScoreStr != "" {
		if v, err := strconv.Atoi(minScoreStr); err == nil {
			minScore = v
		}
	}

	// 在内存中对已排序记录进行筛选
	type RenderRecord struct {
		Rank        int
		Nickname    string
		TotalScore  int
		Status      string
		SecretKey   string
		DynamicData map[string]interface{}
	}

	var filteredRecords []RenderRecord
	totalApproved := len(allRecords)

	for _, r := range allRecords {
		// 总分筛选
		if minScore > 0 && r.TotalScore < minScore {
			continue
		}

		var dynData map[string]interface{}
		if len(r.DynamicData) > 0 {
			json.Unmarshal(r.DynamicData, &dynData)
		}

		// 逐个检查筛选条件
		match := true
		for filterKey, filterVal := range activeFilters {
			if dynData == nil {
				match = false
				break
			}
			storedVal := ""
			if v, ok := dynData[filterKey]; ok {
				switch tv := v.(type) {
				case string:
					storedVal = tv
				case float64:
					storedVal = strconv.FormatFloat(tv, 'f', -1, 64)
				}
			}
			if storedVal != filterVal {
				match = false
				break
			}
		}

		if match {
			filteredRecords = append(filteredRecords, RenderRecord{
				Nickname:    r.Nickname,
				TotalScore:  r.TotalScore,
				Status:      r.Status,
				SecretKey:   r.SecretKey,
				DynamicData: dynData,
			})
		}
	}

	// 重新排名（在筛选结果内）
	for i := range filteredRecords {
		filteredRecords[i].Rank = i + 1
	}

	c.HTML(http.StatusOK, "public/index.tmpl", gin.H{
		"Title":         examConfig.MajorName + " 实时成绩排名榜",
		"ExamName":      examConfig.MajorName,
		"Records":       filteredRecords,
		"DynamicFields": dynamicFields,
		"TotalCount":    totalApproved,
		"FilteredCount": len(filteredRecords),
		"CurrentUser":   userRecord,

		"ActiveFilters": activeFilters,
		"MinScore":      minScoreStr,
	})
}

// LoginPage 密钥输入页面
func LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "public/login.tmpl", gin.H{
		"Title": "验证身份 - " + config.AppConfig.ExamName,
	})
}

// LoginAction 验证密钥
func LoginAction(c *gin.Context) {
	secretKey := c.PostForm("secret_key")
	var record models.ScoreRecord
	if err := config.DB.Where("secret_key = ?", secretKey).First(&record).Error; err != nil {
		c.HTML(http.StatusOK, "public/login.tmpl", gin.H{
			"Title": "验证身份 - " + config.AppConfig.ExamName,
			"Error": "未找到匹配的密钥，请查证后重试或尝试找回密钥。",
		})
		return
	}

	// 设置 Cookie，有效期 30 天
	c.SetCookie("user_secret_key", secretKey, 86400*30, "/", "", false, true)
	c.Redirect(http.StatusFound, "/ky/")
}

// LogoutAction 退出登录
func LogoutAction(c *gin.Context) {
	c.SetCookie("user_secret_key", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/ky/")
}

// SubmitPage 填写分数页面
func SubmitPage(c *gin.Context) {
	var userRecord models.ScoreRecord
	secretKey, err := c.Cookie("user_secret_key")
	if err == nil && secretKey != "" {
		config.DB.Where("secret_key = ?", secretKey).First(&userRecord)
	}

	var examConfig models.ExamConfig
	// 获取考试配置以渲染动态表单
	if err := config.DB.First(&examConfig).Error; err != nil {
		c.String(http.StatusOK, "系统尚未初始化考试配置，请联系管理员在数据库中添加 ExamConfig。")
		return
	}

	// 解析配置中的动态字段，传给模板渲染 input/select
	var dynamicFields []map[string]interface{}
	if len(examConfig.Fields) > 0 {
		json.Unmarshal(examConfig.Fields, &dynamicFields)
	}

	var currentDynamicData map[string]interface{}
	if userRecord.ID != 0 && len(userRecord.DynamicData) > 0 {
		json.Unmarshal(userRecord.DynamicData, &currentDynamicData)
	}

	c.HTML(http.StatusOK, "public/submit.tmpl", gin.H{
		"Title":              "填写/更新分数 - " + config.AppConfig.ExamName,
		"ExamConfig":         examConfig,
		"DynamicFields":      dynamicFields,
		"CurrentUser":        userRecord,
		"CurrentDynamicData": currentDynamicData,
	})
}

// SubmitScore 提交成绩接口
func SubmitScore(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil { // 最大 10MB
		c.JSON(http.StatusBadRequest, gin.H{"error": "表单解析失败"})
		return
	}

	// 识别是否是更新操作（基于 Cookie 密钥）
	secretKey, err := c.Cookie("user_secret_key")
	var existingRecord models.ScoreRecord
	isUpdate := false
	if err == nil && secretKey != "" {
		if config.DB.Where("secret_key = ?", secretKey).First(&existingRecord).Error == nil {
			isUpdate = true
		}
	}

	// 先加载字段配置以确定每个字段的类型
	var examConfig models.ExamConfig
	config.DB.First(&examConfig)
	var dynamicFields []map[string]interface{}
	json.Unmarshal(examConfig.Fields, &dynamicFields)

	// 构建字段类型映射表: key -> "number" / "select" / ...
	fieldTypeMap := make(map[string]string)
	for _, f := range dynamicFields {
		if k, ok := f["Key"].(string); ok {
			if t, ok := f["Type"].(string); ok {
				fieldTypeMap[k] = t
			}
		}
	}

	// 提取动态字段（根据类型决定存储为 float64 还是 string）
	dynamicMap := make(map[string]interface{})
	fixedFields := []string{"exam_id", "major", "nickname", "ticket_prefix", "total_score", "secret_key", "proof"}

	for key, values := range c.Request.MultipartForm.Value {
		if !isFixedField(key, fixedFields) && len(values) > 0 {
			if fieldTypeMap[key] == "number" {
				if valFloat, err := strconv.ParseFloat(values[0], 64); err == nil {
					// 后端分值边界校验
					if key == "politics" || key == "english" {
						if valFloat < 0 || valFloat > 100 {
							c.String(http.StatusBadRequest, "政治或英语成绩必须在0-100之间")
							return
						}
					}
					if key == "math" || key == "subject_408" {
						if valFloat < 0 || valFloat > 150 {
							c.String(http.StatusBadRequest, "数学或专业课成绩必须在0-150之间")
							return
						}
					}
					dynamicMap[key] = valFloat
				}
			} else {
				// select 等非数字类型始终存为字符串
				dynamicMap[key] = values[0]
			}
		}
	}

	var recoveryKeys []string
	json.Unmarshal(examConfig.KeyRecoveryFields, &recoveryKeys)

	// 如果未携带有效 Cookie，则视为新提交，此时需要执行全局四科查重防御
	if !isUpdate && len(recoveryKeys) > 0 {
		var allRecords []models.ScoreRecord
		config.DB.Find(&allRecords)

		for _, rec := range allRecords {
			var storedDyn map[string]interface{}
			json.Unmarshal(rec.DynamicData, &storedDyn)

			matchCount := 0
			for _, rk := range recoveryKeys {
				if storedVal, ok := storedDyn[rk].(float64); ok {
					if submittedVal, ok := dynamicMap[rk].(float64); ok {
						if storedVal == submittedVal {
							matchCount++
						}
					}
				} else if storedStr, ok := storedDyn[rk].(string); ok {
					if submittedStr, ok := dynamicMap[rk].(string); ok {
						if storedStr == submittedStr {
							matchCount++
						}
					}
				}
			}

			// 如果核心找回字段完全匹配 -> 触发【自动转为更新】机制
			if matchCount == len(recoveryKeys) {
				isUpdate = true
				existingRecord = rec      // 直接复用之前的记录
				secretKey = rec.SecretKey // 复用其密钥
				break
			}
		}
	}

	if !isUpdate { // 生成 UUID 作为安全密钥
		secretKey = uuid.New().String()
	}

	proofPath, err := HandleProofUpload(c)
	if err != nil && proofPath == "" {
		// 如果是更新且没有上传新截图，可以忽略
		if err != http.ErrMissingFile {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "图片保存失败"})
			return
		}
	}

	totalScore, _ := strconv.Atoi(c.PostForm("total_score"))
	examID, _ := strconv.Atoi(c.PostForm("exam_id"))
	ip := c.GetHeader("CF-Connecting-IP")
	if ip == "" {
		ip = c.GetHeader("X-Forwarded-For")
		if ip != "" {
			ip = strings.TrimSpace(strings.Split(ip, ",")[0])
		}
	}
	if ip == "" {
		ip = c.GetHeader("X-Real-IP")
	}
	if ip == "" {
		ip = c.ClientIP()
	}

	record := models.ScoreRecord{
		ExamID:       uint(examID),
		Major:        c.PostForm("major"),
		Nickname:     c.PostForm("nickname"),
		TicketPrefix: c.PostForm("ticket_prefix"),
		TotalScore:   totalScore,
		SecretKey:    secretKey,
		IPAddress:    ip,
		Status:       "pending",
	}

	if isUpdate {
		record.ID = existingRecord.ID
		record.CreatedAt = existingRecord.CreatedAt
		if proofPath == "" {
			record.ProofImage = existingRecord.ProofImage
		} else {
			record.ProofImage = proofPath
		}
	} else {
		record.ProofImage = proofPath
	}

	dynamicBytes, _ := json.Marshal(dynamicMap)
	record.DynamicData = datatypes.JSON(dynamicBytes)

	if isUpdate {
		if err := config.DB.Save(&record).Error; err != nil {
			c.String(http.StatusInternalServerError, "成绩更新失败，请返回重试")
			return
		}
	} else {
		if err := config.DB.Create(&record).Error; err != nil {
			c.String(http.StatusInternalServerError, "成绩保存失败，请返回重试")
			return
		}
	}

	// 设置 Cookie（仅对本机生效）
	c.SetCookie("user_secret_key", secretKey, 86400*30, "/", "", false, true)

	c.Header("Content-Type", "text/html; charset=utf-8")
	if isUpdate {
		c.String(http.StatusOK, `
			<div style="text-align:center; margin-top:50px; font-family:sans-serif;">
				<h2 style="color:#28a745;">更新成功！</h2>
				<p>您的成绩已更新，正在重新等待管理员核验。</p>
				<div style="background:#fff3cd; border:1px solid #ffe69c; padding:20px; border-radius:8px; margin: 25px auto; max-width:400px; color:#664d03; box-shadow: 0 4px 6px rgba(0,0,0,0.05);">
					<p style="margin:0 0 15px 0; font-weight:bold; font-size:1.1rem;">🔑 您的专属密钥</p>
					<h3 style="margin:0; font-family:monospace; font-size: 1.5rem; letter-spacing: 2px; user-select:all;">`+secretKey+`</h3>
					<p style="margin:15px 0 0 0; font-size:0.85rem; opacity:0.8;">请妥善保存此密钥，用于在其他设备上修改成绩。</p>
				</div>
				<a href="javascript:void(0);" onclick="copyAndRedirect('`+secretKey+`')" style="padding:10px 20px; background:#0033a0; color:white; text-decoration:none; border-radius:5px; display:inline-block; margin-top:10px;">复制密钥并返回排名</a>
			</div>
			<script>
			function copyAndRedirect(key) {
				var btn = document.querySelector('a[onclick]');
				btn.innerText = '复制中...';
				btn.style.pointerEvents = 'none';
				var fallback = function() {
					var el = document.createElement('textarea');
					el.value = key;
					document.body.appendChild(el);
					el.select();
					try { document.execCommand('copy'); } catch(e) {}
					document.body.removeChild(el);
					window.location.href = '/ky/';
				};
				if (navigator.clipboard) {
					navigator.clipboard.writeText(key).then(function() {
						window.location.href = '/ky/';
					}).catch(fallback);
				} else {
					fallback();
				}
			}
			</script>
		`)
	} else {
		c.String(http.StatusOK, `
			<div style="text-align:center; margin-top:50px; font-family:sans-serif;">
				<h2 style="color:#28a745;">提交成功！</h2>
				<p>您的成绩已记录，正在等待核验。</p>
				<div style="background:#fff3cd; border:1px solid #ffe69c; padding:20px; border-radius:8px; margin: 25px auto; max-width:400px; color:#664d03; box-shadow: 0 4px 6px rgba(0,0,0,0.05);">
					<p style="margin:0 0 15px 0; font-weight:bold; font-size:1.1rem;">⚠️ 请务必保存您的专属修改密钥</p>
					<h3 style="margin:0; font-family:monospace; font-size: 1.5rem; letter-spacing: 2px; user-select:all;">`+secretKey+`</h3>
					<p style="margin:15px 0 0 0; font-size:0.85rem; opacity:0.8;">这是您在其他设备上随时修改或删除成绩的唯一凭证！</p>
				</div>
				<a href="javascript:void(0);" onclick="copyAndRedirect('`+secretKey+`')" style="padding:10px 20px; background:#0033a0; color:white; text-decoration:none; border-radius:5px; display:inline-block;">我已了解并复制，返回排名</a>
			</div>
			<script>
			function copyAndRedirect(key) {
				var btn = document.querySelector('a[onclick]');
				btn.innerText = '复制中...';
				btn.style.pointerEvents = 'none';
				
				var fallback = function() {
					var el = document.createElement('textarea');
					el.value = key;
					document.body.appendChild(el);
					el.select();
					try { document.execCommand('copy'); } catch(e) {}
					document.body.removeChild(el);
					window.location.href = '/ky/';
				};
				
				if (navigator.clipboard) {
					navigator.clipboard.writeText(key).then(function() {
						window.location.href = '/ky/';
					}).catch(fallback);
				} else {
					fallback();
				}
			}
			</script>
		`)
	}
}

func isFixedField(key string, fixedFields []string) bool {
	for _, f := range fixedFields {
		if strings.EqualFold(key, f) {
			return true
		}
	}
	return false
}

// StatsPage 数据统计分析大盘页面
func StatsPage(c *gin.Context) {
	var examConfig models.ExamConfig
	if err := config.DB.First(&examConfig).Error; err != nil {
		c.String(http.StatusOK, "系统尚未配置此数据")
		return
	}

	var dynamicFields []map[string]interface{}
	if len(examConfig.Fields) > 0 {
		json.Unmarshal(examConfig.Fields, &dynamicFields)
	}

	var records []models.ScoreRecord
	config.DB.Where("status = ?", "approved").Find(&records)

	// 计算所需指标结构
	type NumberStat struct {
		Key   string
		Label string
		Max   float64
		Min   float64
		Avg   float64
		Data  []float64 // 供后续如果前端要画分布直方图备用，或用来生成区间段
	}
	type SelectStat struct {
		Key    string
		Label  string
		Counts map[string]int
	}

	numberStats := make(map[string]*NumberStat)
	selectStats := make(map[string]*SelectStat)

	// 初始化 TotalScore 统计
	numberStats["total_score"] = &NumberStat{Key: "total_score", Label: "总分", Min: -1}

	// 遍历定义初始化
	for _, f := range dynamicFields {
		key := f["Key"].(string)
		label := f["Label"].(string)
		fType := f["Type"].(string)

		if fType == "number" {
			numberStats[key] = &NumberStat{Key: key, Label: label, Min: -1}
		} else if fType == "select" {
			selectStats[key] = &SelectStat{Key: key, Label: label, Counts: make(map[string]int)}
		}
	}

	// 遍历所有数据聚合计算
	for _, r := range records {
		// 总分
		ts := float64(r.TotalScore)
		ns := numberStats["total_score"]
		ns.Data = append(ns.Data, ts)
		ns.Avg += ts
		if ns.Min == -1 || ts < ns.Min {
			ns.Min = ts
		}
		if ts > ns.Max {
			ns.Max = ts
		}

		var dynData map[string]interface{}
		json.Unmarshal(r.DynamicData, &dynData)

		for key, val := range dynData {
			if numStat, ok := numberStats[key]; ok {
				if numVal, isNum := val.(float64); isNum {
					numStat.Data = append(numStat.Data, numVal)
					numStat.Avg += numVal
					if numStat.Min == -1 || numVal < numStat.Min {
						numStat.Min = numVal
					}
					if numVal > numStat.Max {
						numStat.Max = numVal
					}
				}
			} else if selStat, ok := selectStats[key]; ok {
				if strVal, isStr := val.(string); isStr {
					selStat.Counts[strVal]++
				}
			}
		}
	}

	// 计算平均分
	totalCount := float64(len(records))
	if totalCount > 0 {
		for _, stat := range numberStats {
			if len(stat.Data) > 0 {
				stat.Avg = stat.Avg / float64(len(stat.Data))
			} else {
				stat.Avg = 0
			}
			if stat.Min == -1 {
				stat.Min = 0
			}
		}
	}

	// 分数段生成函数 (将具体数字列表划分为10分的区间段，用于柱状图)
	generateDistribution := func(data []float64, interval int) map[string]int {
		dist := make(map[string]int)
		for _, v := range data {
			lower := (int(v) / interval) * interval
			upper := lower + interval
			key := fmt.Sprintf("%d-%d", lower, upper)
			dist[key]++
		}
		return dist
	}

	// 生成分布图数据
	distData := make(map[string]map[string]int)
	for key, stat := range numberStats {
		// 总分可以分段大一点，单科10分一段
		interval := 10
		if key == "total_score" {
			interval = 20
		}
		distData[key] = generateDistribution(stat.Data, interval)
	}

	var userRecord models.ScoreRecord
	secretKey, err := c.Cookie("user_secret_key")
	if err == nil && secretKey != "" {
		config.DB.Where("secret_key = ?", secretKey).First(&userRecord)
	}

	c.HTML(http.StatusOK, "public/stats.tmpl", gin.H{
		"Title":       "数据统计分析 - " + examConfig.MajorName,
		"ExamName":    examConfig.MajorName,
		"NumberStats": numberStats,
		"SelectStats": selectStats,
		"DistData":    distData,
		"Count":       len(records),
		"CurrentUser": userRecord,
	})
}
