package server

import (
	"net/http"

	"proofread-exhibition/config"

	"github.com/gin-gonic/gin"
)

func InitRouter(engine *gin.Engine, cfg *config.GlobalConfig) {
	// 加载简单的模板页面，容器内要修改为/app/pkg/tpl/*
	engine.LoadHTMLGlob("tpl/*")

	// 提供静态文件服务（favicon.ico 等）
	engine.StaticFile("/favicon.ico", "tpl/favicon.ico")

	// 首页：展示输入框
	engine.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})

	// 提交待校阅文本并展示结果
	engine.POST("/proofread", func(c *gin.Context) {
		ProofreadHandler(c, cfg)
	})
}
