package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"sort"
	"strings"

	"proofread-exhibition/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

// ProofreadHandler 简单页面的表单提交处理：调用后端校阅接口并高亮展示结果
func ProofreadHandler(c *gin.Context, cfg *config.GlobalConfig) {
	text := c.PostForm("content")
	if strings.TrimSpace(text) == "" {
		c.HTML(http.StatusBadRequest, "index.html", gin.H{
			"error":   "请输入需要校阅的内容",
			"content": text,
		})
		return
	}

	// 调用配置中的校阅接口
	reqBody, err := json.Marshal(map[string]string{
		"content": text,
	})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"error":   "请求编码失败",
			"content": text,
		})
		return
	}

	resp, err := http.Post(cfg.ProofreadApiUrl, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		c.HTML(http.StatusBadGateway, "index.html", gin.H{
			"error":   fmt.Sprintf("调用校阅接口失败: %v", err),
			"content": text,
		})
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			zap.S().Errorf(err.Error())
		}
	}(resp.Body)

	bodyBytes, _ := io.ReadAll(resp.Body)
	var raw interface{}
	err = json.Unmarshal(bodyBytes, &raw)
	if err != nil {
		fmt.Println("Invalid JSON:", err)
		return
	}
	// 格式化 json，美观输出（服务一）
	formattedJSON, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.HTML(http.StatusBadGateway, "index.html", gin.H{
			"error":       fmt.Sprintf("校阅接口返回错误状态码: %d", resp.StatusCode),
			"content":     text,
			"rawResponse": string(formattedJSON),
		})
		return
	}

	var result Result
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"error":       fmt.Sprintf("解析校阅结果失败: %v", err),
			"content":     text,
			"rawResponse": string(formattedJSON),
		})
		return
	}

	highlighted := highlightContent(text, result.Data.Errors)

	// 调用第二个校阅服务（用于对比）
	var highlighted2 string
	var rawResponse2 string
	if cfg.ProofreadApiUrl2 != "" {
		//resp2, err2 := http.Post(cfg.ProofreadApiUrl2, "application/json", bytes.NewReader(reqBody))
		//if err2 == nil {
		//	defer func(Body io.ReadCloser) {
		//		_ = Body.Close()
		//	}(resp2.Body)

		//bodyBytes2, _ := io.ReadAll(resp2.Body)

		// 尝试格式化第二个服务的 JSON
		//var raw2 interface{}
		//if err := json.Unmarshal(bodyBytes2, &raw2); err == nil {
		//	if formattedJSON2, err := json.MarshalIndent(raw2, "", "  "); err == nil {
		//		rawResponse2 = string(formattedJSON2)
		//	}
		//} else {
		//	rawResponse2 = string(bodyBytes2)
		//}

		// 只有在状态码 200 且解析成功时才高亮
		//if resp2.StatusCode == http.StatusOK {
		items := []SecondServiceItem{
			{Tag: "replace", StartPos: 40, EndPos: 41, Text: "取"},
			{Tag: "replace", StartPos: 137, EndPos: 138, Text: "懈"},
			{Tag: "delete", StartPos: 175, EndPos: 176, Text: "成"},
			{Tag: "insert", StartPos: 230, EndPos: 230, Text: "十八"},
			{Tag: "delete", StartPos: 239, EndPos: 241, Text: "同志"},
		}
		//	if err := json.Unmarshal(bodyBytes2, &items); err == nil {

		highlighted2 = highlightSecondService(text, items)
		//}
		//}
		//}
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"content":      text,
		"highlighted":  template.HTML(highlighted),
		"highlighted2": template.HTML(highlighted2),
		"errorItems":   result.Data.Errors,
		"hasResult":    true,
		"originalText": text,
		"rawResponse":  string(formattedJSON),
		"rawResponse2": rawResponse2,
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

		tooltip := fmt.Sprintf(
			`<span class="tooltip-content">操作: <span class="tooltip-message">%s</span></span><span class="tooltip-content">文本: <span class="tooltip-suggestion">%s</span></span>`,
			html.EscapeString(it.Tag),
			html.EscapeString(it.Text),
		)

		endTag := "</span>"
		contentRunes = insertRunes(contentRunes, end, []rune(endTag))

		startTag := fmt.Sprintf(`<span class="highlight"><span class="tooltip">%s</span>`, tooltip)
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

		// 构建 tooltip 内容
		tooltipHTML := buildTooltip(result)

		// 插入结束标记
		endTag := "</span>"
		contentRunes = insertRunes(contentRunes, end, []rune(endTag))

		// 插入开始标记和 tooltip
		startTag := fmt.Sprintf(`<span class="highlight"><span class="tooltip">%s</span>`, tooltipHTML)
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
