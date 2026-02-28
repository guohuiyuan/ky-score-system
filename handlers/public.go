package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/ky-score-system/config"
	"github.com/guohuiyuan/ky-score-system/models"
	"gorm.io/datatypes"
)

// IndexPage 前台排行榜主页
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
		// 密钥无效，清除 Cookie 并重定向回登录页
		c.SetCookie("user_secret_key", "", -1, "/", "", false, true)
		c.Redirect(http.StatusFound, "/ky/login")
		return
	}

	var records []models.ScoreRecord
	// 只查询已通过审核的成绩，按总分降序排列
	config.DB.Where("status = ?", "approved").Order("total_score desc").Find(&records)

	// 获取考试配置以提取动态表头 (为了简化，这里默认取第一条配置)
	var examConfig models.ExamConfig
	config.DB.First(&examConfig)

	// 解析动态表头
	// 例如: [{"Key":"math", "Label":"数学"}, {"Key":"politics", "Label":"政治"}]
	var dynamicHeaders []map[string]interface{}
	if len(examConfig.Fields) > 0 {
		json.Unmarshal(examConfig.Fields, &dynamicHeaders)
	}

	// 组装供前端渲染的数据结构
	type RenderRecord struct {
		Rank        int
		Nickname    string
		TotalScore  int
		Status      string
		DynamicData map[string]interface{}
	}

	var renderRecords []RenderRecord
	for i, r := range records {
		var dynData map[string]interface{}
		// 将存入数据库的 JSON 反序列化为 map，方便模板里用 {{index .DynamicData "xxx"}} 读取
		if len(r.DynamicData) > 0 {
			json.Unmarshal(r.DynamicData, &dynData)
		}

		renderRecords = append(renderRecords, RenderRecord{
			Rank:        i + 1,
			Nickname:    r.Nickname,
			TotalScore:  r.TotalScore,
			Status:      r.Status,
			DynamicData: dynData,
		})
	}

	examName := examConfig.MajorName
	if examName == "" {
		examName = "考研"
	}

	// 渲染 HTML 模板
	c.HTML(http.StatusOK, "public/index.tmpl", gin.H{
		"Title":          "考研分数实时排行榜",
		"ExamName":       examName,
		"TotalCount":     len(records),
		"DynamicHeaders": dynamicHeaders,
		"Records":        renderRecords,
		"CurrentUser":    userRecord,
	})
}

// LoginPage 密钥输入页面
func LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "public/login.tmpl", gin.H{
		"Title": "验证身份 - 哈工大计算机考研",
	})
}

// LoginAction 验证密钥
func LoginAction(c *gin.Context) {
	secretKey := c.PostForm("secret_key")
	var record models.ScoreRecord
	if err := config.DB.Where("secret_key = ?", secretKey).First(&record).Error; err != nil {
		c.HTML(http.StatusOK, "public/login.tmpl", gin.H{
			"Title": "验证身份 - 哈工大计算机考研",
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

// RecoverKeyPage 找回密钥页面
func RecoverKeyPage(c *gin.Context) {
	var examConfig models.ExamConfig
	if err := config.DB.First(&examConfig).Error; err != nil {
		c.String(http.StatusOK, "系统错误，无法获取配置")
		return
	}

	var recoveryKeys []string
	if len(examConfig.KeyRecoveryFields) > 0 {
		json.Unmarshal(examConfig.KeyRecoveryFields, &recoveryKeys)
	}

	var dynamicFields []map[string]interface{}
	if len(examConfig.Fields) > 0 {
		json.Unmarshal(examConfig.Fields, &dynamicFields)
	}

	// 筛出用于找回的配置字段
	var recoverFieldsDesc []map[string]interface{}
	for _, f := range dynamicFields {
		for _, rk := range recoveryKeys {
			if f["Key"] == rk {
				recoverFieldsDesc = append(recoverFieldsDesc, f)
			}
		}
	}

	c.HTML(http.StatusOK, "public/recover.tmpl", gin.H{
		"Title":         "找回密钥 - 哈工大计算机考研",
		"RecoverFields": recoverFieldsDesc,
	})
}

// RecoverKeyAction 找回密钥逻辑
func RecoverKeyAction(c *gin.Context) {
	var examConfig models.ExamConfig
	config.DB.First(&examConfig)
	var recoveryKeys []string
	json.Unmarshal(examConfig.KeyRecoveryFields, &recoveryKeys)

	// 获取所有的提交记录
	var allRecords []models.ScoreRecord
	config.DB.Find(&allRecords)

	var foundKey string
	for _, rec := range allRecords {
		var dynData map[string]interface{}
		json.Unmarshal(rec.DynamicData, &dynData)

		matchCount := 0
		for _, rk := range recoveryKeys {
			// 将用户提交的字符串与存的 float64 比较
			submittedVal := c.PostForm(rk)
			if storedVal, ok := dynData[rk].(float64); ok {
				if floatToStr(storedVal) == submittedVal {
					matchCount++
				}
			} else if storedStr, ok := dynData[rk].(string); ok {
				if storedStr == submittedVal {
					matchCount++
				}
			}
		}

		if matchCount == len(recoveryKeys) && matchCount > 0 {
			foundKey = rec.SecretKey
			break
		}
	}

	if foundKey != "" {
		c.JSON(http.StatusOK, gin.H{"secret_key": foundKey})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "提供的成绩无法匹配到任何已存在的记录，请确认成绩完全一致。"})
	}
}

func floatToStr(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
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
		"Title":              "填写/更新分数 - 哈工大计算机考研",
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

	if !isUpdate {
		// 查重逻辑：提取找回字段并全库比对（防止同一个人拿同一个成绩跨专业或重复提交）
		var examConfig models.ExamConfig
		config.DB.First(&examConfig)
		var recoveryKeys []string
		json.Unmarshal(examConfig.KeyRecoveryFields, &recoveryKeys)

		var allRecords []models.ScoreRecord
		config.DB.Find(&allRecords)
		for _, rec := range allRecords {
			var dynData map[string]interface{}
			json.Unmarshal(rec.DynamicData, &dynData)

			matchCount := 0
			for _, rk := range recoveryKeys {
				submittedVal := c.PostForm(rk)
				if storedVal, ok := dynData[rk].(float64); ok {
					if floatToStr(storedVal) == submittedVal {
						matchCount++
					}
				} else if storedStr, ok := dynData[rk].(string); ok {
					if storedStr == submittedVal {
						matchCount++
					}
				}
			}
			if len(recoveryKeys) > 0 && matchCount == len(recoveryKeys) {
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.String(http.StatusConflict, `<div style="text-align:center; margin-top:50px; font-family:sans-serif;">
					<h2 style="color:#dc3545;"><i class="bi bi-x-circle"></i> 提交失败：成绩查重拦截</h2>
					<p>系统检测到完全相同的核心成绩组合已被录入！为了防刷榜，系统不允许重复提交。</p>
					<p style="color:#666; font-size:0.9rem;">提示：如果您填错了想重填，请点击顶部导航的「验证身份」找回已有记录的专属密钥后，直接修改原记录。</p>
					<a href="javascript:history.back()" style="padding:10px 20px; background:#0033a0; color:white; text-decoration:none; border-radius:5px; display:inline-block; margin-top:20px;">返回修改</a>
				</div>`)
				return
			}
		}

		// 生成 8 位十六进制安全密钥
		bytes := make([]byte, 4)
		rand.Read(bytes)
		secretKey = hex.EncodeToString(bytes)
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

	record := models.ScoreRecord{
		ExamID:       uint(examID),
		Major:        c.PostForm("major"),
		Nickname:     c.PostForm("nickname"),
		TicketPrefix: c.PostForm("ticket_prefix"),
		TotalScore:   totalScore,
		SecretKey:    secretKey,
		IPAddress:    c.ClientIP(),
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

	// 提取动态字段
	dynamicMap := make(map[string]interface{})
	fixedFields := []string{"exam_id", "major", "nickname", "ticket_prefix", "total_score", "secret_key", "proof"}

	for key, values := range c.Request.PostForm {
		if !isFixedField(key, fixedFields) && len(values) > 0 {
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
			} else {
				dynamicMap[key] = values[0]
			}
		}
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
				<a href="/ky/" style="padding:10px 20px; background:#0033a0; color:white; text-decoration:none; border-radius:5px; display:inline-block; margin-top:20px;">返回排行榜</a>
			</div>
		`)
	} else {
		c.String(http.StatusOK, `
			<div style="text-align:center; margin-top:50px; font-family:sans-serif;">
				<h2 style="color:#28a745;">提交成功！</h2>
				<p>您的成绩已记录，正在等待核验。</p>
				<div style="background:#fff3cd; border:1px solid #ffe69c; padding:20px; border-radius:8px; margin: 25px auto; max-width:400px; color:#664d03; box-shadow: 0 4px 6px rgba(0,0,0,0.05);">
					<p style="margin:0 0 15px 0; font-weight:bold; font-size:1.1rem;">⚠️ 请务必保存您的专属修改密钥</p>
					<h3 style="margin:0; font-family:monospace; font-size: 2.2rem; letter-spacing: 4px; user-select:all;">`+secretKey+`</h3>
					<p style="margin:15px 0 0 0; font-size:0.85rem; opacity:0.8;">这是您在其他设备上随时修改或删除成绩的唯一凭证！</p>
				</div>
				<a href="/ky/" style="padding:10px 20px; background:#0033a0; color:white; text-decoration:none; border-radius:5px; display:inline-block;">我已了解并复制，返回排名</a>
			</div>
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
