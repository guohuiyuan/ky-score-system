package config

import (
	"encoding/json"
	"log"
	"os"

	"github.com/glebarez/sqlite"
	"github.com/guohuiyuan/ky-score-system/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AppConfigJSON 应用级 JSON 配置结构
type AppConfigJSON struct {
	ExamName string `json:"exam_name"`
	Admin    struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"admin"`
	Fields            []map[string]interface{} `json:"fields"`
	KeyRecoveryFields []string                 `json:"key_recovery_fields"`
}

var DB *gorm.DB
var AppConfig AppConfigJSON

// LoadConfig 从 data/config.json 读取全局配置
func LoadConfig() {
	data, err := os.ReadFile("data/config.json")
	if err != nil {
		log.Fatalf("❌ 无法读取 data/config.json: %v", err)
	}
	if err := json.Unmarshal(data, &AppConfig); err != nil {
		log.Fatalf("❌ 解析 data/config.json 出错: %v", err)
	}
	log.Printf("✅ 配置文件加载成功: 考试名称=%s, 字段数=%d", AppConfig.ExamName, len(AppConfig.Fields))
}

// GetDirAlias 从已加载的配置中提取方向别名映射表
func GetDirAlias() map[string]string {
	dirAlias := make(map[string]string)
	for _, field := range AppConfig.Fields {
		if field["Key"] == "direction" {
			if aliasRaw, ok := field["Alias"]; ok {
				if aliasMap, ok := aliasRaw.(map[string]interface{}); ok {
					for k, v := range aliasMap {
						if vs, ok := v.(string); ok {
							dirAlias[k] = vs
						}
					}
				}
			}
		}
	}
	return dirAlias
}

func InitDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open("data/score.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("无法连接到 SQLite: %v", err)
	}

	DB.AutoMigrate(&models.ExamConfig{}, &models.ScoreRecord{})
	initDataIfEmpty()
}

func initDataIfEmpty() {
	var count int64
	DB.Model(&models.ExamConfig{}).Count(&count)
	if count > 0 {
		return // 数据库已有数据，跳过
	}

	log.Println("检测到初次运行，正在从 config.json 注入配置...")

	fieldsJSON, _ := json.Marshal(AppConfig.Fields)
	recoveryJSON, _ := json.Marshal(AppConfig.KeyRecoveryFields)

	exam := models.ExamConfig{
		MajorName:         AppConfig.ExamName,
		Fields:            datatypes.JSON(fieldsJSON),
		KeyRecoveryFields: datatypes.JSON(recoveryJSON),
	}
	DB.Create(&exam)

	// 注入一条测试数据
	testDynamicData, _ := json.Marshal(map[string]interface{}{
		"politics":    68,
		"english":     81,
		"math":        131,
		"subject_408": 115,
		"undergrad":   "985",
		"attempt":     "二战",
		"direction":   "【本部】计算学部/未来技术学院-计算机科学与技术(学硕)",
	})

	DB.Create(&models.ScoreRecord{
		ExamID:       exam.ID,
		Major:        "HIT CS",
		Nickname:     "c***",
		TicketPrefix: "123456",
		TotalScore:   395,
		SecretKey:    "testkey",
		IPAddress:    "127.0.0.1",
		Status:       "approved",
		DynamicData:  datatypes.JSON(testDynamicData),
	})

	log.Println("✅ 动态配置与测试数据注入成功！")
}
