package handlers

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// HandleProofUpload 处理截图上传，返回相对路径或错误
func HandleProofUpload(c *gin.Context) (string, error) {
	file, err := c.FormFile("proof")
	if err != nil {
		return "", nil
	}

	ext := filepath.Ext(file.Filename)
	newFileName := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	
	// 1. 物理保存路径：存放到 data 目录下
	savePath := fmt.Sprintf("data/uploads/%s", newFileName)
	
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		return "", err
	}

	// 2. 数据库保存路径(URL)：因为在路由中配置了前缀 /ky 和静态目录 /uploads
	// 所以前端访问图片的完整相对路径应该是 /ky/uploads/xxx.png
	return "/ky/uploads/" + newFileName, nil
}