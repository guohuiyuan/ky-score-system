package main

import (
	"log"
	"os"

	"github.com/guohuiyuan/ky-score-system/config"
	"github.com/guohuiyuan/ky-score-system/routes"
)

func main() {
	// 1. 确保数据目录存在 (包含上传目录)
	if err := os.MkdirAll("data/uploads", os.ModePerm); err != nil {
		log.Fatalf("无法创建数据目录: %v", err)
	}

	// 2. 加载 JSON 配置
	config.LoadConfig()

	// 3. 初始化数据库
	config.InitDB()

	// 3. 初始化路由
	r := routes.SetupRouter()

	// 4. 启动 HTTP 服务
	log.Println("服务启动成功，访问地址: http://localhost:8080/ky")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
