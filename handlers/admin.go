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

// AdminLoginAction 管理员登录逻辑（从数据库查询）
func AdminLoginAction(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	var admin models.AdminUser
	if err := config.DB.Where("username = ? AND password = ?", username, password).First(&admin).Error; err != nil {
		c.HTML(http.StatusUnauthorized, "admin/login.tmpl", gin.H{
			"Title": "管理员登录 - 考研分数统计",
			"Error": "账号或密码错误，请重试",
		})
		return
	}

	// 登录成功，设置 Cookie
	c.SetCookie("admin_token", "super_secret_token", 86400, "/", "", false, true)

	// 如果需要修改密码，跳转到修改密码页
	if admin.MustChangePassword {
		c.Redirect(http.StatusFound, "/ky/admin/change-password")
		return
	}

	c.Redirect(http.StatusFound, "/ky/admin/dashboard")
}

// AdminChangePasswordPage 修改密码页面
func AdminChangePasswordPage(c *gin.Context) {
	c.HTML(http.StatusOK, "admin/change_password.tmpl", gin.H{
		"Title": "修改管理员密码",
	})
}

// AdminChangePasswordAction 修改密码处理
func AdminChangePasswordAction(c *gin.Context) {
	oldPass := c.PostForm("old_password")
	newPass := c.PostForm("new_password")
	confirmPass := c.PostForm("confirm_password")

	if newPass != confirmPass {
		c.HTML(http.StatusOK, "admin/change_password.tmpl", gin.H{
			"Title": "修改管理员密码",
			"Error": "两次输入的新密码不一致",
		})
		return
	}

	if len(newPass) < 6 {
		c.HTML(http.StatusOK, "admin/change_password.tmpl", gin.H{
			"Title": "修改管理员密码",
			"Error": "新密码长度不能少于6位",
		})
		return
	}

	// 验证旧密码
	var admin models.AdminUser
	if err := config.DB.Where("password = ?", oldPass).First(&admin).Error; err != nil {
		c.HTML(http.StatusOK, "admin/change_password.tmpl", gin.H{
			"Title": "修改管理员密码",
			"Error": "旧密码错误",
		})
		return
	}

	// 更新密码
	config.DB.Model(&admin).Updates(map[string]interface{}{
		"password":             newPass,
		"must_change_password": false,
	})

	c.Redirect(http.StatusFound, "/ky/admin/dashboard")
}

// AdminDashboard 后台核验控制台
func AdminDashboard(c *gin.Context) {
	// 检查是否需要先改密码
	var admin models.AdminUser
	config.DB.First(&admin)
	if admin.MustChangePassword {
		c.Redirect(http.StatusFound, "/ky/admin/change-password")
		return
	}

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
		"DirAlias":       config.GetDirAlias(),
	})
}

// AdminVerifyRecord 审核记录 (通过/驳回)
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

// AdminDeleteRecord 删除记录
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
