package server

import (
	"net/http"

	"proofread-exhibition/config"

	"github.com/gin-gonic/gin"
)

func InitRouter(engine *gin.Engine, cfg *config.GlobalConfig) {
	// 加载简单的模板页面
	engine.LoadHTMLGlob("tpl/*")

	// 提供静态文件服务（favicon.ico 等）
	engine.StaticFile("/favicon.ico", "tpl/favicon.ico")

	// 首页：展示输入框
	engine.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})

	// 兼容旧的表单提交（整页刷新）
	engine.POST("/proofread", func(c *gin.Context) {
		ProofreadHandler(c, cfg)
	})

	// JSON 接口：服务一
	engine.POST("/api/proofread/one", func(c *gin.Context) {
		ProofreadAPI1Handler(c, cfg)
	})

	// JSON 接口：服务二
	engine.POST("/api/proofread/two", func(c *gin.Context) {
		ProofreadAPI2Handler(c, cfg)
	})
}
