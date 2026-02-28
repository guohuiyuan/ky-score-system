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

// AppConfigJSON 应用级 JSON 配置结构（不含管理员密码，密码在数据库中管理）
type AppConfigJSON struct {
	ExamName          string                   `json:"exam_name"`
	Fields            []map[string]interface{} `json:"fields"`
	KeyRecoveryFields []string                 `json:"key_recovery_fields"`
}

var DB *gorm.DB
var AppConfig AppConfigJSON

// LoadConfig 从 data/config.json 读取全局配置（若不存在则从 config.example.json 复制）
func LoadConfig() {
	configPath := "data/config.json"
	examplePath := "data/config.example.json"

	// 如果 config.json 不存在，从 example 复制一份
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Println("⚠️  data/config.json 不存在，正在从 config.example.json 创建...")
		src, err := os.ReadFile(examplePath)
		if err != nil {
			log.Fatalf("❌ 也找不到 data/config.example.json: %v", err)
		}
		if err := os.WriteFile(configPath, src, 0644); err != nil {
			log.Fatalf("❌ 写入 data/config.json 失败: %v", err)
		}
		log.Println("✅ 已从示例配置创建 data/config.json")
	}

	data, err := os.ReadFile(configPath)
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

	DB.AutoMigrate(&models.ExamConfig{}, &models.ScoreRecord{}, &models.AdminUser{})
	initDataIfEmpty()
	initAdminIfEmpty()
}

func initDataIfEmpty() {
	var count int64
	DB.Model(&models.ExamConfig{}).Count(&count)
	if count > 0 {
		return
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

// initAdminIfEmpty 如果数据库中没有管理员账号，则创建默认账号 admin/admin123（首次登录需修改密码）
func initAdminIfEmpty() {
	var count int64
	DB.Model(&models.AdminUser{}).Count(&count)
	if count > 0 {
		return
	}

	log.Println("创建默认管理员账号 admin / admin123（首次登录需修改密码）")
	DB.Create(&models.AdminUser{
		Username:           "admin",
		Password:           "admin123",
		MustChangePassword: true,
	})
}
