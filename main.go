package main

import (
	"fmt"
	"log"
	"os"

	"github.com/guohuiyuan/ky-score-system/config"
	"github.com/guohuiyuan/ky-score-system/routes"
	"github.com/spf13/cobra"
)

var port string

var rootCmd = &cobra.Command{
	Use:   "ky-score-system",
	Short: "考研分数统计与实时排行榜系统",
	Long:  "基于 Go 语言和动态配置构建的高性能、极简、开箱即用的考研估分与志愿填报辅助系统。",
	Run: func(cmd *cobra.Command, args []string) {
		// 1. 确保数据目录存在 (包含上传目录)
		if err := os.MkdirAll("data/uploads", os.ModePerm); err != nil {
			log.Fatalf("无法创建数据目录: %v", err)
		}

		// 2. 加载 JSON 配置
		config.LoadConfig()

		// 3. 初始化数据库
		config.InitDB()

		// 4. 初始化路由
		r := routes.SetupRouter()

		// 5. 启动 HTTP 服务
		log.Printf("服务启动成功，访问地址: http://localhost:%s/ky", port)
		if err := r.Run(":" + port); err != nil {
			log.Fatalf("服务启动失败: %v", err)
		}
	},
}

func init() {
	rootCmd.Flags().StringVarP(&port, "port", "p", "8080", "HTTP 服务监听端口")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
