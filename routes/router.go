package routes

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guohuiyuan/ky-score-system/handlers"
	"github.com/guohuiyuan/ky-score-system/middlewares"
)

func SetupRouter(templatesFS embed.FS, staticFS embed.FS) *gin.Engine {
	r := gin.Default()

	// 配置获取真实 IP (解决 Nginx/Docker 环境下 c.ClientIP() 只有内网 IP 的问题)
	r.ForwardedByClientIP = true
	if err := r.SetTrustedProxies(nil); err != nil {
		// SetTrustedProxies(nil) 意味着信任所有代理，由于只是记分系统通常可接受
		// 也可以配置真实的代理 IP 如 []string{"192.168.0.0/16"}
	}

	// 1. 从嵌入的文件系统加载模板
	subTemplatesFS, _ := fs.Sub(templatesFS, "templates")
	tmpl := template.Must(template.New("").ParseFS(subTemplatesFS,
		"public/*.tmpl",
		"admin/*.tmpl",
		"layout/*.tmpl",
	))
	r.SetHTMLTemplate(tmpl)

	// 根路径重定向到 /ky/
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/ky/")
	})

	// 统一路由前缀
	app := r.Group("/ky")
	{
		// 2. 静态文件：嵌入的静态资源 + 磁盘上的用户上传目录
		subStaticFS, _ := fs.Sub(staticFS, "static")
		app.StaticFS("/static", http.FS(subStaticFS))
		app.Static("/uploads", "./data/uploads") // 用户上传的截图仍从磁盘读取

		// 3. 前台公开路由
		app.GET("/login", handlers.LoginPage)
		app.POST("/login", handlers.LoginAction)
		app.GET("/logout", handlers.LogoutAction)

		app.GET("/", handlers.IndexPage)        // 实时排行榜
		app.GET("/stats", handlers.StatsPage)   // 数据统计分析
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
