package parser

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

func FetchPageContent(searchURL string, client *http.Client) ([]string, error) {
	fmt.Printf("正在搜索: %s\n", searchURL)

	resp, err := client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("发送 GET 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("获取网页内容失败: %d %s", resp.StatusCode, resp.Status)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取网页内容失败: %v", err)
	}
	pageContent := string(bodyBytes)

	streamLinks := []string{}

	// 方法1: 使用class属性查找链接
	classRegex := regexp.MustCompile(`class\s*=\s*[\"']([^\"']*play[^\"']*|[^\"']*stream[^\"']*|[^\"']*link[^\"']*)[\"'][^>]*>([^<]*)`)
	classMatches := classRegex.FindAllStringSubmatch(pageContent, -1)
	for _, match := range classMatches {
		// 在匹配的元素附近查找URL
		if url := extractURLFromContext(pageContent, match[0]); url != "" && isValidStreamURL(url) {
			streamLinks = append(streamLinks, url)
		}
	}

	// 方法2: 查找所有onclick事件，不依赖具体函数名
	onclickRegex := regexp.MustCompile(`onclick\s*=\s*[\"']?[a-zA-Z0-9_]+\s*\(\s*[\"']([^\"']+)[\"']?\)[\"']?`)
	onclickMatches := onclickRegex.FindAllStringSubmatch(pageContent, -1)
	for _, match := range onclickMatches {
		url := match[1]
		if isValidStreamURL(url) {
			streamLinks = append(streamLinks, url)
		}
	}

	// 方法3: 查找data-url或类似属性
	dataURLRegex := regexp.MustCompile(`data-(?:url|link|stream)\s*=\s*[\"']([^\"']+)[\"']`)
	dataURLMatches := dataURLRegex.FindAllStringSubmatch(pageContent, -1)
	for _, match := range dataURLMatches {
		url := match[1]
		if isValidStreamURL(url) {
			streamLinks = append(streamLinks, url)
		}
	}

	// 方法4: 查找href属性中的流媒体链接
	hrefRegex := regexp.MustCompile(`href\s*=\s*[\"']([^\"']+(?:\.m3u8|\.m3u|/live/|/hls/|udp://|rtmp://|rtsp://)[^\"']*)[\"']`)
	hrefMatches := hrefRegex.FindAllStringSubmatch(pageContent, -1)
	for _, match := range hrefMatches {
		url := match[1]
		if isValidStreamURL(url) {
			streamLinks = append(streamLinks, url)
		}
	}

	// 方法5: 在JavaScript代码中查找URL模式
	jsURLRegex := regexp.MustCompile(`[\"'](https?://[^\"']+(?:\.m3u8|\.m3u|/live/|/hls/)[^\"']*)[\"']`)
	jsURLMatches := jsURLRegex.FindAllStringSubmatch(pageContent, -1)
	for _, match := range jsURLMatches {
		url := match[1]
		if isValidStreamURL(url) {
			streamLinks = append(streamLinks, url)
		}
	}

	fmt.Printf("本页找到 %d 个流媒体链接\n", len(streamLinks))
	return streamLinks, nil
}

// extractURLFromContext 从给定元素的上下文中提取URL
func extractURLFromContext(pageContent, elementHTML string) string {
	// 查找元素在页面中的位置
	elementIndex := strings.Index(pageContent, elementHTML)
	if elementIndex == -1 {
		return ""
	}

	// 在元素前后200个字符范围内查找URL
	start := elementIndex - 200
	if start < 0 {
		start = 0
	}
	end := elementIndex + len(elementHTML) + 200
	if end > len(pageContent) {
		end = len(pageContent)
	}

	contextText := pageContent[start:end]

	// 在上下文中查找各种URL模式
	urlPatterns := []string{
		`onclick\s*=\s*[\"']?[a-zA-Z0-9_]+\s*\(\s*[\"']([^\"']+)[\"']`,
		`data-(?:url|link|stream)\s*=\s*[\"']([^\"']+)[\"']`,
		`href\s*=\s*[\"']([^\"']+)[\"']`,
		`src\s*=\s*[\"']([^\"']+)[\"']`,
		`[\"'](https?://[^\"'\s]+)[\"']`,
	}

	for _, pattern := range urlPatterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindAllStringSubmatch(contextText, -1)
		for _, match := range matches {
			if len(match) > 1 && isValidStreamURL(match[1]) {
				return match[1]
			}
		}
	}

	return ""
}

func isValidStreamURL(url string) bool {
	// 检查是否是支持的协议
	if !strings.HasPrefix(url, "http") && !strings.HasPrefix(url, "udp") && !strings.HasPrefix(url, "rtmp") && !strings.HasPrefix(url, "rtsp") {
		return false
	}

	// 过滤掉明显不是流媒体的链接
	lowerURL := strings.ToLower(url)

	// 排除JS、CSS、图片等静态资源
	if strings.Contains(lowerURL, ".js") || strings.Contains(lowerURL, ".css") ||
		strings.Contains(lowerURL, ".png") || strings.Contains(lowerURL, ".jpg") ||
		strings.Contains(lowerURL, ".gif") || strings.Contains(lowerURL, ".ico") ||
		strings.Contains(lowerURL, "google") || strings.Contains(lowerURL, "pagead") ||
		strings.Contains(lowerURL, "w3.org") || strings.Contains(lowerURL, "cse.js") {
		return false
	}

	// 排除明显的网页链接
	if strings.Contains(lowerURL, ".html") || strings.Contains(lowerURL, ".htm") ||
		strings.Contains(lowerURL, ".php") || strings.Contains(lowerURL, ".aspx") ||
		strings.Contains(lowerURL, ".jsp") || strings.Contains(lowerURL, ".cgi") {
		return false
	}

	// 排除搜索引擎和统计链接
	if strings.Contains(lowerURL, "google") || strings.Contains(lowerURL, "baidu") ||
		strings.Contains(lowerURL, "bing") || strings.Contains(lowerURL, "analytics") ||
		strings.Contains(lowerURL, "tracking") || strings.Contains(lowerURL, "stat") {
		return false
	}

	// 明确的流媒体链接特征 - 优先级高
	if strings.Contains(lowerURL, ".m3u8") || strings.Contains(lowerURL, ".m3u") ||
		strings.Contains(lowerURL, ".ts") || strings.Contains(lowerURL, ".m4s") ||
		strings.Contains(lowerURL, "/live/") || strings.Contains(lowerURL, "/hls/") ||
		strings.Contains(lowerURL, "udp://") || strings.Contains(lowerURL, "rtmp://") ||
		strings.Contains(lowerURL, "rtsp://") {
		return true
	}

	// 可能的流媒体链接特征 - 更宽松的检测
	if strings.Contains(lowerURL, "stream") || strings.Contains(lowerURL, "play") ||
		strings.Contains(lowerURL, "media") || strings.Contains(lowerURL, "video") ||
		strings.Contains(lowerURL, "tv") || strings.Contains(lowerURL, "channel") ||
		strings.Contains(lowerURL, "cdn") || strings.Contains(lowerURL, "live") ||
		strings.Contains(lowerURL, "iptv") || strings.Contains(lowerURL, "cam") {
		return true
	}

	// IP地址形式的链接（很多直播流使用IP地址）
	if strings.Contains(lowerURL, "://") && 
	   (strings.Contains(lowerURL, "192.168.") || strings.Contains(lowerURL, "10.") || 
	    strings.Contains(lowerURL, "172.16.") || strings.Contains(lowerURL, "172.31.") ||
	    strings.Contains(lowerURL, "127.0.0.1")) {
		return true
	}

	// 端口号形式的链接（很多直播流使用非标准端口）
	if strings.Contains(lowerURL, "://") && strings.Contains(lowerURL, ":") {
		parts := strings.Split(lowerURL, ":")
		if len(parts) >= 3 {
			portPart := parts[2]
			if len(portPart) > 0 {
				// 提取端口号
				endIdx := strings.IndexAny(portPart, "/?")
				if endIdx == -1 {
					endIdx = len(portPart)
				}
				portStr := portPart[:endIdx]
				// 检查是否是数字端口号
				for _, char := range portStr {
					if char < '0' || char > '9' {
						return false
					}
				}
				// 端口号在常用范围内
				if len(portStr) > 0 && len(portStr) <= 5 {
					return true
				}
			}
		}
	}

	// 域名中包含流媒体相关关键词的链接
	if strings.Contains(lowerURL, "cdn") || strings.Contains(lowerURL, "stream") ||
	   strings.Contains(lowerURL, "media") || strings.Contains(lowerURL, "tv") ||
	   strings.Contains(lowerURL, "live") || strings.Contains(lowerURL, "video") {
		return true
	}

	// 其他可能的流媒体链接 - 只要看起来像可能的媒体服务
	if strings.Contains(lowerURL, "api") && (strings.Contains(lowerURL, "media") || 
	   strings.Contains(lowerURL, "stream") || strings.Contains(lowerURL, "video")) {
		return true
	}

	return false
}

func RemoveDuplicates(urls []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, url := range urls {
		if !seen[url] {
			seen[url] = true
			result = append(result, url)
		}
	}
	return result
}
