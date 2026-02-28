package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/ky-score-system/handlers"
	"github.com/guohuiyuan/ky-score-system/middlewares"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	// 1. 加载所有模板文件
	// 注意路径匹配：需要解析 templates 及其所有子目录下的 .tmpl 文件
	r.LoadHTMLGlob("templates/**/*.tmpl")

	// 统一路由前缀
	app := r.Group("/ky")
	{
		// 2. 静态文件与上传目录
		// 确保根目录存在 uploads 文件夹，否则启动或上传时会报错
		app.Static("/uploads", "./data/uploads")
		app.Static("/static", "./static")

		// 3. 前台公开路由
		app.GET("/login", handlers.LoginPage)
		app.POST("/login", handlers.LoginAction)
		app.GET("/logout", handlers.LogoutAction)

		app.GET("/", handlers.IndexPage)        // 实时排行榜
		app.GET("/submit", handlers.SubmitPage) // 填写分数页面

		// 提交接口挂载 IP 限流中间件 (限制 60 秒一次)
		app.POST("/api/submit", middlewares.SubmitLimit(60), handlers.SubmitScore)

		// 4. 后台管理员登录
		app.GET("/admin", func(c *gin.Context) {
			c.Redirect(302, "/ky/admin/login")
		})
		app.GET("/admin/login", handlers.AdminLoginPage)
		app.POST("/admin/login", handlers.AdminLoginAction)

		// 可选：退出登录
		app.GET("/admin/logout", func(c *gin.Context) {
			c.SetCookie("admin_token", "", -1, "/", "", false, true)
			c.Redirect(302, "/ky/admin/login")
		})

		// 5. 后台需鉴权路由组
		admin := app.Group("/admin")
		admin.Use(middlewares.AdminAuth()) // 挂载管理员鉴权中间件
		{
			admin.GET("/dashboard", handlers.AdminDashboard)          // 核验控制台
			admin.POST("/api/verify/:id", handlers.AdminVerifyRecord) // 审核(通过/驳回)
			admin.GET("/change-password", handlers.AdminChangePasswordPage)
			admin.POST("/change-password", handlers.AdminChangePasswordAction)

			// 管理接口 (路径 /ky/admin/api/...)
			admin.DELETE("/api/record/:id", handlers.AdminDeleteRecord) // 删除记录
			admin.POST("/api/batch-verify", handlers.AdminBatchVerify)  // 批量审核
			admin.POST("/api/batch-delete", handlers.AdminBatchDelete)  // 批量删除
		}

		// 6. Excel 导入导出接口 (带鉴权)
		adminAPI := app.Group("/api/admin")
		adminAPI.Use(middlewares.AdminAuth())
		{
			adminAPI.GET("/excel-template", handlers.AdminDownloadExcelTemplate)
			adminAPI.POST("/import-excel", handlers.AdminImportExcel)
			adminAPI.GET("/export-excel", handlers.AdminExportExcel)
		}
	}

	return r
}
