package tester

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"m3u8_selector/core" // 导入core包
)

// testStreamConnectSpeed tests the connection speed of a stream (initial screening)
func TestStreamConnectSpeed(url string, timeout time.Duration) core.M3U8Source {
	if strings.HasPrefix(url, "udp") {
		// 对于UDP链接，模拟连接测速
		start := time.Now()

		// 模拟UDP测速
		success := simulateUDPSpeedTest()
		latency := time.Since(start)

		if !success {
			return core.M3U8Source{
				URL:     url,
				Latency: latency,
				Valid:   false,
				Error:   "UDP test failed",
			}
		}

		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   true,
			Error:   "OK",
		}
	} else {
		client := &http.Client{
			Timeout: timeout,
		}

		// 先尝试HEAD请求快速检测
		start := time.Now()
		resp, err := client.Head(url)
		latency := time.Since(start)

		if err != nil {
			return core.M3U8Source{
				URL:     url,
				Latency: latency,
				Valid:   false,
				Error:   err.Error(),
			}
		}
		defer resp.Body.Close()

		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   resp.StatusCode == 200,
			Error:   fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
	}
}

// simulateUDPSpeedTest is a placeholder for simulating UDP speed test
func simulateUDPSpeedTest() bool {
	return true
}

// isM3U8Content checks if the content of a URL is likely an M3U8 playlist
func IsM3U8Content(url string, timeout time.Duration) bool {
	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}

	// Read a small portion of the body to check for #EXTM3U
	buffer := make([]byte, 1024) // Read up to 1KB
	n, err := resp.Body.Read(buffer)
	if err != nil && err != io.EOF {
		return false
	}

	content := string(buffer[:n])

	// 首先检查是否为JSON响应
	trimmedContent := strings.TrimSpace(content)
	if strings.HasPrefix(trimmedContent, "{") {
		return false // JSON响应不是M3U8
	}

	// 检查是否包含无效或错误关键词
	if strings.Contains(content, "无效") || strings.Contains(content, "invalid") || strings.Contains(content, "error") || strings.Contains(content, "not exist") {
		return false
	}

	// 检查是否为HTML页面
	if strings.Contains(content, "<html>") || strings.Contains(content, "<HTML>") {
		return false
	}

	return strings.Contains(content, "#EXTM3U")
}

// testM3U8PlaybackSpeed tests the actual playback download speed
func TestM3U8PlaybackSpeed(url string, timeout time.Duration) core.M3U8Source {
	client := &http.Client{
		Timeout: timeout,
	}

	// 首先获取M3U8播放列表
	start := time.Now()
	resp, err := client.Get(url)
	latency := time.Since(start)

	if err != nil {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   err.Error(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
	}

	// 读取M3U8内容
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   err.Error(),
		}
	}

	content := string(body)

	// 首先检查是否为JSON响应（最重要的检查）
	trimmedContent := strings.TrimSpace(content)
	if strings.HasPrefix(trimmedContent, "{") {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   "JSON response instead of M3U8",
		}
	}

	// 检查响应大小 - M3U8文件通常应该有合理的大小
	if len(body) < 100 {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   fmt.Sprintf("Response too small (%d bytes), likely an error page", len(body)),
		}
	}

	// 检查是否包含JSON错误关键词
	if strings.Contains(content, `"Ret"`) || strings.Contains(content, `"Reason"`) ||
		strings.Contains(content, "无效") || strings.Contains(content, "invalid") ||
		strings.Contains(content, "expired") || strings.Contains(content, "error") {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   "API error response (token invalid or other error)",
		}
	}

	// 检查是否为HTML错误页面
	if strings.Contains(content, "<html>") || strings.Contains(content, "<HTML>") {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   "HTML error page instead of M3U8",
		}
	}

	// 检查Content-Type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(contentType, "application/x-mpegURL") &&
		!strings.Contains(contentType, "application/vnd.apple.mpegurl") &&
		!strings.Contains(contentType, "video/mp2t") &&
		!strings.Contains(contentType, "text/plain") {
		if strings.Contains(contentType, "application/json") {
			return core.M3U8Source{
				URL:     url,
				Latency: latency,
				Valid:   false,
				Error:   "JSON response instead of M3U8",
			}
		}
	}

	if !strings.HasPrefix(content, "#EXTM3U") {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   "Not a valid M3U8 file (missing #EXTM3U)",
		}
	}

	if strings.Contains(content, "#EXT-X-PLAYLIST-TYPE:VOD") {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   "VOD (Video On Demand) stream, not live",
		}
	}

	if strings.Contains(content, "#EXT-X-ENDLIST") {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   "Fixed-length video (not live stream)",
		}
	}

	// 检查M3U8内容是否包含有效的媒体片段（#EXTINF和.ts链接）
	if !strings.Contains(content, "#EXTINF:") || (!strings.Contains(content, ".ts") && !strings.Contains(content, ".m4s")) {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   "M3U8 content does not contain valid media segments",
		}
	}

	// 检查媒体片段数量 - 确保有足够的片段（至少2个）
	extinfCount := strings.Count(content, "#EXTINF:")
	if extinfCount < 2 {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   fmt.Sprintf("Too few media segments (%d), need at least 2", extinfCount),
		}
	}

	tsURL := extractFirstTSURL(content, url)
	if tsURL == "" {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   "No TS segments found",
		}
	}

	downloadStart := time.Now()
	tsResp, err := client.Get(tsURL)
	if err != nil {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   fmt.Sprintf("TS download failed: %v", err),
		}
	}
	defer tsResp.Body.Close()

	if tsResp.StatusCode != 200 {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   fmt.Sprintf("TS HTTP %d", tsResp.StatusCode),
		}
	}

	tsBody, err := ioutil.ReadAll(tsResp.Body)
	downloadTime := time.Since(downloadStart)

	if err != nil {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   fmt.Sprintf("TS read failed: %v", err),
		}
	}

	dataSize := int64(len(tsBody))
	// 检查下载的TS片段大小，如果过小则认为无效
	if dataSize < 1024 { // 假设有效的TS片段至少1KB
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   fmt.Sprintf("TS segment too small (%d bytes)", dataSize),
		}
	}

	// 检查TS内容是否为JSON错误响应
	tsContent := string(tsBody)
	if strings.Contains(tsContent, `"Ret"`) || strings.Contains(tsContent, `"Reason"`) ||
		strings.Contains(tsContent, "无效") || strings.HasPrefix(strings.TrimSpace(tsContent), "{") {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   "TS segment returned JSON error response",
		}
	}
	downloadSpeed := float64(dataSize) / downloadTime.Seconds() / 1024

	return core.M3U8Source{
		URL:           url,
		Latency:       latency,
		DownloadSpeed: downloadSpeed,
		Valid:         true,
		Error:         "OK",
		DataSize:      dataSize,
		DownloadTime:  downloadTime,
	}
}

// testGenericStreamSpeed tests generic stream speed for non-M3U8 links
func TestGenericStreamSpeed(url string, timeout time.Duration) core.M3U8Source {
	if strings.HasPrefix(url, "udp") {
		// UDP链接使用模拟测试
		start := time.Now()
		// 模拟下载一些数据
		time.Sleep(100 * time.Millisecond) // 模拟网络延迟
		downloadTime := time.Since(start)

		// 模拟下载了1MB数据
		simulatedDataSize := int64(1024 * 1024)
		downloadSpeed := float64(simulatedDataSize) / downloadTime.Seconds() / 1024

		return core.M3U8Source{
			URL:           url,
			Latency:       downloadTime,
			DownloadSpeed: downloadSpeed,
			Valid:         true,
			Error:         "UDP simulation",
			DataSize:      simulatedDataSize,
			DownloadTime:  downloadTime,
		}
	}

	// 对于HTTP流媒体链接，下载部分内容测试速度
	client := &http.Client{
		Timeout: timeout,
	}

	start := time.Now()
	resp, err := client.Get(url)
	latency := time.Since(start)

	if err != nil {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   err.Error(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
	}

	// 下载前几KB数据测试速度
	downloadStart := time.Now()
	buffer := make([]byte, 50*1024) // 下载50KB
	n, err := resp.Body.Read(buffer)
	downloadTime := time.Since(downloadStart)

	if err != nil && n == 0 {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   fmt.Sprintf("Read failed: %v", err),
		}
	}

	// 检查下载的内容是否为JSON错误响应
	content := string(buffer[:n])
	trimmedContent := strings.TrimSpace(content)
	if strings.HasPrefix(trimmedContent, "{") {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   "JSON response instead of stream",
		}
	}

	// 检查是否包含错误关键词
	if strings.Contains(content, `"Ret"`) || strings.Contains(content, `"Reason"`) ||
		strings.Contains(content, "无效") || strings.Contains(content, "invalid") ||
		strings.Contains(content, "not exist") || strings.Contains(content, "error") {
		return core.M3U8Source{
			URL:     url,
			Latency: latency,
			Valid:   false,
			Error:   "API error response (invalid, not exist, or other error)",
		}
	}

	dataSize := int64(n)
	downloadSpeed := float64(dataSize) / downloadTime.Seconds() / 1024

	return core.M3U8Source{
		URL:           url,
		Latency:       latency,
		DownloadSpeed: downloadSpeed,
		Valid:         true,
		Error:         "OK",
		DataSize:      dataSize,
		DownloadTime:  downloadTime,
	}
}

func extractFirstTSURL(m3u8Content, baseURL string) string {
	lines := strings.Split(m3u8Content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "http") {
				return line
			} else {
				return resolveURL(baseURL, line)
			}
		}
	}
	return ""
}

func resolveURL(baseURL, relativeURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	rel, err := url.Parse(relativeURL)
	if err != nil {
		return ""
	}
	return base.ResolveReference(rel).String()
}

func TestAllSources(urls []string, timeout time.Duration) []core.M3U8Source {
	var wg sync.WaitGroup
	results := make([]core.M3U8Source, len(urls))

	fmt.Printf("正在并发测试 %d 个直播源的实际访问速度...\n", len(urls))

	semaphore := make(chan struct{}, 10)

	for i, url := range urls {
		wg.Add(1)
		go func(index int, url string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			fmt.Printf(".")
			initialResult := TestStreamConnectSpeed(url, timeout)
			if initialResult.Valid {
				if strings.HasPrefix(url, "http") {
					// 尝试判断是否为M3U8内容
					if IsM3U8Content(url, timeout) {
						results[index] = TestM3U8PlaybackSpeed(url, timeout)
					} else {
						// 对于非M3U8内容或UDP链接，使用通用的流媒体测试
						results[index] = TestGenericStreamSpeed(url, timeout)
					}
				} else {
					// 对于UDP等链接，使用通用测试
					results[index] = TestGenericStreamSpeed(url, timeout)
				}
			} else {
				results[index] = initialResult
			}
		}(i, url)
	}

	wg.Wait()
	fmt.Println()
	return results
}
