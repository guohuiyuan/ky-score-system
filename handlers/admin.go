package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/ky-score-system/config"
	"github.com/guohuiyuan/ky-score-system/models"
)

// AdminLoginPage 管理员登录页面
func AdminLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "admin/login.tmpl", gin.H{
		"Title": "管理员登录 - 考研分数统计",
	})
}

// AdminLoginAction 管理员登录逻辑
func AdminLoginAction(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	// 从 JSON 配置文件读取管理员凭证
	if username == config.AppConfig.Admin.Username && password == config.AppConfig.Admin.Password {
		// 登录成功，设置 Cookie (有效期24小时)
		c.SetCookie("admin_token", "super_secret_token", 86400, "/", "", false, true)
		c.Redirect(http.StatusFound, "/ky/admin/dashboard")
		return
	}

	// 登录失败，重新渲染页面并传入错误信息 (前端模板可以加个 {{.Error}} 提示)
	c.HTML(http.StatusUnauthorized, "admin/login.tmpl", gin.H{
		"Title": "管理员登录 - 考研分数统计",
		"Error": "账号或密码错误，请重试",
	})
}

// AdminDashboard 后台核验控制台
func AdminDashboard(c *gin.Context) {
	var records []models.ScoreRecord

	query := config.DB.Model(&models.ScoreRecord{})

	searchKw := c.Query("search")
	if searchKw != "" {
		query = query.Where("nickname LIKE ? OR ticket_prefix LIKE ?", "%"+searchKw+"%", "%"+searchKw+"%")
	}

	statusFilter := c.Query("status")
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	query.Order("created_at desc").Find(&records)

	// 获取表头配置
	var examConfig models.ExamConfig
	config.DB.First(&examConfig)

	var dynamicHeaders []map[string]interface{}
	if len(examConfig.Fields) > 0 {
		json.Unmarshal(examConfig.Fields, &dynamicHeaders)
	}

	// 组装带动态数据的结构体供模板渲染
	type RenderRecord struct {
		models.ScoreRecord
		DynamicData map[string]interface{}
	}

	var renderRecords []RenderRecord
	for _, r := range records {
		var dynData map[string]interface{}
		if len(r.DynamicData) > 0 {
			json.Unmarshal(r.DynamicData, &dynData)
		}
		renderRecords = append(renderRecords, RenderRecord{
			ScoreRecord: r,
			DynamicData: dynData,
		})
	}

	c.HTML(http.StatusOK, "admin/dashboard.tmpl", gin.H{
		"Title":          "核验管理控制台",
		"Records":        renderRecords,
		"DynamicHeaders": dynamicHeaders,
		"SearchKw":       searchKw,
		"StatusFilter":   statusFilter,
	})
}

// AdminVerifyRecord 审核记录 (通过/驳回) - 这个是供 AJAX 调用的 API，保留 JSON 返回
func AdminVerifyRecord(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required,oneof=approved rejected"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "状态必须是 approved 或 rejected"})
		return
	}

	result := config.DB.Model(&models.ScoreRecord{}).Where("id = ?", id).Update("status", req.Status)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "状态更新成功"})
}

// AdminDeleteRecord 删除记录 - 供 AJAX 调用
func AdminDeleteRecord(c *gin.Context) {
	id := c.Param("id")

	result := config.DB.Delete(&models.ScoreRecord{}, id)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "记录已删除"})
}

// AdminBatchVerify 批量审核记录
func AdminBatchVerify(c *gin.Context) {
	var req struct {
		IDs    []uint `json:"ids" binding:"required"`
		Status string `json:"status" binding:"required,oneof=approved rejected"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	result := config.DB.Model(&models.ScoreRecord{}).Where("id IN ?", req.IDs).Update("status", req.Status)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "批量更新失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "批量状态更新成功"})
}

// AdminBatchDelete 批量删除记录
func AdminBatchDelete(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	result := config.DB.Where("id IN ?", req.IDs).Delete(&models.ScoreRecord{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "批量删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "批量记录已删除"})
}
