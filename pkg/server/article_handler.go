package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"sort"
	"strings"

	"proofread-exhibition/config"

	"github.com/gin-gonic/gin"
)

type Result struct {
	Data struct {
		Errors []ErrorItem
	} `json:"data"`
}

type ErrorItem struct {
	Type          string   `json:"type"`                    // 错误类型
	Text          string   `json:"text"`                    // 原文中被标记的片段（即错误词或标点）
	Start         int      `json:"start"`                   // 在 CleanContent 中的起始 rune 下标
	End           int      `json:"end"`                     // 在 CleanContent 中的结束 rune 下标
	OriginalStart int      `json:"originalStart,omitempty"` // 在原始 HTML 中的 byte 起始
	OriginalEnd   int      `json:"originalEnd,omitempty"`   // 在原始 HTML 中的 byte 结束
	Suggestions   []string `json:"suggestions"`             // 建议替换词列表
	Message       string   `json:"message"`                 // 错误解释
	Sentence      string   `json:"sentence"`                // 所属句子（便于前端展示上下文）
}

// SecondServiceItem 第二个校阅服务的返回结构
type SecondServiceItem struct {
	Tag      string `json:"tag"`
	StartPos int    `json:"start_pos"`
	EndPos   int    `json:"end_pos"`
	Text     string `json:"text"`
}

// SecondServiceResponse 第二个服务整体响应结构
type SecondServiceResponse struct {
	Status int `json:"status"`
	Data   struct {
		Errors   []SecondServiceItem `json:"errors"`
		Duration float64             `json:"duration"`
	} `json:"data"`
	Msg string `json:"msg"`
}

// ProofreadHandler 仍保留表单提交的旧逻辑（整页刷新），目前页面主要通过前端 JS 调用 JSON 接口
func ProofreadHandler(c *gin.Context, cfg *config.GlobalConfig) {
	// 兼容旧用法：简单复用页面渲染
	c.HTML(http.StatusOK, "index.html", gin.H{})
}

// ProofreadAPI1Handler 调用服务一，返回 JSON 结果
func ProofreadAPI1Handler(c *gin.Context, cfg *config.GlobalConfig) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content 不能为空"})
		return
	}
	text := req.Content

	reqBody, err := json.Marshal(map[string]string{
		"content": text,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "请求编码失败"})
		return
	}

	resp, err := http.Post(cfg.ProofreadApiUrl, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("调用校阅接口失败: %v", err)})
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	bodyBytes, _ := io.ReadAll(resp.Body)
	var raw interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "服务一返回的不是合法 JSON"})
		return
	}
	formattedJSON, _ := json.MarshalIndent(raw, "", "  ")

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":       fmt.Sprintf("校阅接口返回错误状态码: %d", resp.StatusCode),
			"rawResponse": string(formattedJSON),
		})
		return
	}

	var result Result
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":       fmt.Sprintf("解析校阅结果失败: %v", err),
			"rawResponse": string(formattedJSON),
		})
		return
	}

	highlighted := highlightContent(text, result.Data.Errors)

	c.JSON(http.StatusOK, gin.H{
		"highlightedHtml": highlighted,
		"errorCount":      len(result.Data.Errors),
		"errors":          result.Data.Errors,
		"rawResponse":     string(formattedJSON),
	})
}

// ProofreadAPI2Handler 调用服务二，返回 JSON 结果
func ProofreadAPI2Handler(c *gin.Context, cfg *config.GlobalConfig) {
	if cfg.ProofreadApiUrl2 == "" {
		c.JSON(http.StatusOK, gin.H{
			"highlightedHtml": "",
			"rawResponse":     "",
		})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content 不能为空"})
		return
	}
	text := req.Content

	reqBody, err := json.Marshal(map[string]string{
		"content": text,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "请求编码失败"})
		return
	}

	resp2, err2 := http.Post(cfg.ProofreadApiUrl2, "application/json", bytes.NewReader(reqBody))
	if err2 != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("调用校阅服务二失败: %v", err2)})
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp2.Body)

	bodyBytes2, _ := io.ReadAll(resp2.Body)

	// 尝试格式化第二个服务的 JSON
	var raw2 interface{}
	var rawResponse2 string
	if err := json.Unmarshal(bodyBytes2, &raw2); err == nil {
		if formattedJSON2, err := json.MarshalIndent(raw2, "", "  "); err == nil {
			rawResponse2 = string(formattedJSON2)
		}
	} else {
		rawResponse2 = string(bodyBytes2)
	}

	if resp2.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":       fmt.Sprintf("服务二返回错误状态码: %d", resp2.StatusCode),
			"rawResponse": rawResponse2,
		})
		return
	}

	var respObj SecondServiceResponse
	if err := json.Unmarshal(bodyBytes2, &respObj); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":       fmt.Sprintf("解析服务二结果失败: %v", err),
			"rawResponse": rawResponse2,
		})
		return
	}

	items := respObj.Data.Errors
	highlighted2 := highlightSecondService(text, items)

	c.JSON(http.StatusOK, gin.H{
		"highlightedHtml": highlighted2,
		"errors":          items,
		"rawResponse":     rawResponse2,
	})
}

// highlightSecondService 将第二个服务的结果高亮
func highlightSecondService(content string, items []SecondServiceItem) string {
	if len(items) == 0 {
		return html.EscapeString(content)
	}

	// 按位置从后往前排序
	sorted := make([]SecondServiceItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartPos > sorted[j].StartPos
	})

	contentRunes := []rune(content)

	for _, it := range sorted {
		start := it.StartPos
		end := it.EndPos
		if start < 0 || end > len(contentRunes) || start > end {
			continue
		}

		// 根据操作类型选择不同的高亮类
		var highlightClass string
		switch strings.ToLower(it.Tag) {
		case "delete", "del", "删除":
			highlightClass = "highlight-delete"
		case "insert", "ins", "插入":
			highlightClass = "highlight-insert"
		case "replace", "rep", "替换":
			highlightClass = "highlight-replace"
		default:
			highlightClass = "highlight"
		}

		// 插入结束标记
		endTag := "</span>"
		contentRunes = insertRunes(contentRunes, end, []rune(endTag))

		// 插入开始标记（根据操作类型使用不同的CSS类）
		startTag := fmt.Sprintf(`<span class="%s">`, highlightClass)
		contentRunes = insertRunes(contentRunes, start, []rune(startTag))
	}

	return string(contentRunes)
}

// highlightContent 将原文内容按照校阅结果进行高亮标记
func highlightContent(content string, results []ErrorItem) string {
	if len(results) == 0 {
		return html.EscapeString(content)
	}

	// 按位置从后往前排序，避免插入时位置偏移
	sortedResults := make([]ErrorItem, len(results))
	copy(sortedResults, results)
	sort.Slice(sortedResults, func(i, j int) bool {
		return sortedResults[i].Start > sortedResults[j].Start
	})

	// 将内容转换为 rune 切片以正确处理中文
	contentRunes := []rune(content)

	// 从后往前插入高亮标记
	for _, result := range sortedResults {
		start := result.Start
		end := result.End

		// 确保位置有效
		if start < 0 || end > len(contentRunes) || start >= end {
			continue
		}

		// 插入结束标记
		endTag := "</span>"
		contentRunes = insertRunes(contentRunes, end, []rune(endTag))

		// 插入开始标记（仅高亮，不包含tooltip）
		startTag := `<span class="highlight">`
		contentRunes = insertRunes(contentRunes, start, []rune(startTag))
	}

	// 预防xss 先转义整个内容（包括我们插入的标签）
	//escaped := html.EscapeString(string(contentRunes))
	//
	//// 恢复我们插入的 HTML 标签（需要按照嵌套顺序恢复）
	//// 先恢复最外层的 highlight span
	//escaped = strings.ReplaceAll(escaped, "&lt;span class=&#34;highlight&#34;&gt;", "<span class=\"highlight\">")
	//// 恢复 tooltip span
	//escaped = strings.ReplaceAll(escaped, "&lt;span class=&#34;tooltip&#34;&gt;", "<span class=\"tooltip\">")
	//// 恢复 tooltip-content span
	//escaped = strings.ReplaceAll(escaped, "&lt;span class=&#34;tooltip-content&#34;&gt;", "<span class=\"tooltip-content\">")
	//escaped = strings.ReplaceAll(escaped, "&lt;span class=&#34;tooltip-content tooltip-message&#34;&gt;", "<span class=\"tooltip-content tooltip-message\">")
	//// 恢复 tooltip-message span
	//escaped = strings.ReplaceAll(escaped, "&lt;span class=&#34;tooltip-message&#34;&gt;", "<span class=\"tooltip-message\">")
	//// 恢复 tooltip-suggestion span
	//escaped = strings.ReplaceAll(escaped, "&lt;span class=&#34;tooltip-suggestion&#34;&gt;", "<span class=\"tooltip-suggestion\">")
	//// 恢复所有结束标签
	//escaped = strings.ReplaceAll(escaped, "&lt;/span&gt;", "</span>")

	return string(contentRunes)
}

// buildTooltip 构建 tooltip HTML 内容
func buildTooltip(result ErrorItem) string {
	var parts []string

	// 处理建议
	if len(result.Suggestions) > 0 {
		suggestions := result.Suggestions
		suggestionText := html.EscapeString(suggestions[0])
		if len(suggestions) > 1 {
			for i := 1; i < len(suggestions); i++ {
				suggestionText += ", " + html.EscapeString(suggestions[i])
			}
		}
		parts = append(parts, fmt.Sprintf(`<span class="tooltip-content">建议: <span class="tooltip-suggestion">%s</span></span>`, suggestionText))
	}

	// 处理消息
	if result.Message != "" {
		messageText := html.EscapeString(result.Message)
		parts = append(parts, fmt.Sprintf(`<span class="tooltip-content">原因: <span class="tooltip-message">%s</span></span>`, messageText))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "")
}

// insertRunes 在指定位置插入 rune 切片
func insertRunes(slice []rune, index int, insert []rune) []rune {
	if index < 0 || index > len(slice) {
		return slice
	}
	result := make([]rune, 0, len(slice)+len(insert))
	result = append(result, slice[:index]...)
	result = append(result, insert...)
	result = append(result, slice[index:]...)
	return result
}
