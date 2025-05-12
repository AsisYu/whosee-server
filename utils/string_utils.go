/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-29 12:15:00
 * @Description: 字符串工具
 */
package utils

// TruncateString 截断长字符串，超过最大长度时添加省略号
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
