package config

import (
	"encoding/json"
	"log"

	"github.com/glebarez/sqlite"
	"github.com/guohuiyuan/ky-score-system/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var DB *gorm.DB

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

	log.Println("检测到初次运行，正在注入哈工大计算机考研专属配置...")

	// 100% 安全的强类型初始化，告别 YAML 解析坑
	fields := []map[string]interface{}{
		{"Key": "politics", "Label": "政治", "Type": "number"},
		{"Key": "english", "Label": "英语一", "Type": "number"},
		{"Key": "math", "Label": "数学一", "Type": "number"},
		{"Key": "subject_408", "Label": "408/854", "Type": "number"},
		{"Key": "undergrad", "Label": "本科层次", "Type": "select", "Options": []string{"985", "211", "双非(含双一流)"}},
		{"Key": "attempt", "Label": "几战", "Type": "select", "Options": []string{"一战", "二战", "多战"}},
		{"Key": "direction", "Label": "方向", "Type": "select", "Options": []string{
			"【本部】计算学部/未来技术学院-计算机科学与技术(学硕)",
			"【本部】计算学部-软件工程(学硕)",
			"【本部】计算学部-智能科学与技术(087600)",
			"【本部】计算学部-计算机方向(专硕)",
			"【本部】计算学部-软件方向(专硕)",
			"【本部】计算学部-计算机方向(威海专硕)",
			"【深圳】计算机方向",
			"【深圳】人工智能方向",
			"【深圳】计算机方向(校企联培)",
			"【深圳】AI智能(校企联培)",
			"【威海】计算机方向",
			"【郑州】计算机方向",
			"【郑州】软件方向",
			"【重庆】计算机方向",
			"【重庆】软件方向",
			"【苏州】计算机方向",
			"【苏州】软件方向",
		}},
	}
	fieldsJSON, _ := json.Marshal(fields)

	// 配置找回密钥需要的字段
	recoveryFields := []string{"politics", "english", "math", "subject_408"}
	recoveryJSON, _ := json.Marshal(recoveryFields)

	exam := models.ExamConfig{
		MajorName:         "哈工大计算机考研 (HIT CS)",
		Fields:            datatypes.JSON(fieldsJSON),
		KeyRecoveryFields: datatypes.JSON(recoveryJSON),
	}
	DB.Create(&exam)

	// 注入一条完美的初始测试数据
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
