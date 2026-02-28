package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ExamConfig 考试配置表 (用于定义不同学校/专业的动态列)
type ExamConfig struct {
	gorm.Model
	MajorName string `gorm:"size:100;unique;comment:专业名称"`
	// Fields 存储前端表单结构，例如:
	// [{"key":"math", "label":"数学", "type":"number"}, {"key":"attempt", "label":"几战", "type":"select", "options":["一战","二战"]}]
	// [{"key":"math", "label":"数学", "type":"number"}, {"key":"attempt", "label":"几战", "type":"select", "options":["一战","二战"]}]
	Fields datatypes.JSON `gorm:"comment:动态列配置(JSON格式)"`

	// KeyRecoveryFields 存储用于找回密钥的字段列表
	// 例如: ["politics", "english", "math", "subject_408"]
	KeyRecoveryFields datatypes.JSON `gorm:"comment:找回密钥所需字段列表(JSON格式)"`
}

// ScoreRecord 成绩记录表
type ScoreRecord struct {
	gorm.Model
	ExamID       uint   `gorm:"index;comment:关联的考试配置ID"`
	Major        string `gorm:"size:50;comment:报考专业"`
	Nickname     string `gorm:"size:50;comment:ID/昵称"`
	TicketPrefix string `gorm:"size:10;comment:准考证后6位"`
	TotalScore   int    `gorm:"index;comment:总分(用于排序)"`
	SecretKey    string `gorm:"size:50;comment:防篡改修改密钥"`
	IPAddress    string `gorm:"size:50;comment:提交者IP"`
	ProofImage   string `gorm:"size:255;comment:截图相对路径"`
	Status       string `gorm:"size:20;default:'pending';comment:状态: pending/approved/rejected"`

	// DynamicData 存储用户提交的动态分数/选项，例如:
	// {"math": 135, "politics": 68, "attempt": "二战"}
	DynamicData datatypes.JSON `gorm:"comment:动态字段数据(JSON格式)"`
}

// AdminUser 管理员用户表
type AdminUser struct {
	gorm.Model
	Username           string `gorm:"size:50;uniqueIndex;comment:管理员账号"`
	Password           string `gorm:"size:255;comment:管理员密码"`
	MustChangePassword bool   `gorm:"default:true;comment:是否必须修改密码"`
}
