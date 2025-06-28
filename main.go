package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"

	"m3u8_selector/core"
	"m3u8_selector/parser"
	"m3u8_selector/tester"
)

func main() {
	baseURL := "http://tonkiang.us/"
	searchKeyword := "五星体育"
	pageLimit := 5 // 默认分页数量

	if len(os.Args) > 1 {
		searchKeyword = os.Args[1]
	}
	if len(os.Args) > 2 {
		if limit, err := strconv.Atoi(os.Args[2]); err == nil && limit > 0 {
			pageLimit = limit
		}
	}

	fmt.Printf("搜索关键词: %s\n", searchKeyword)
	fmt.Printf("搜索分页数量: %d\n", pageLimit)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	allM3uLinks := []string{}

	for page := 1; page <= pageLimit; page++ {

		fmt.Printf("\n=== 搜索第 %d 页 ===\n", page)

		params := url.Values{}
		params.Add("iptv", searchKeyword)
		if page > 1 {
			params.Add("page", fmt.Sprintf("%d", page))
		}
		searchURL := baseURL + "?" + params.Encode()

		pageLinks, err := parser.FetchPageContent(searchURL, client)
		if err != nil {
			fmt.Printf("第 %d 页搜索失败: %v\n", page, err)
			continue
		}

		allM3uLinks = append(allM3uLinks, pageLinks...)
	}

	if len(allM3uLinks) == 0 {
		fmt.Println("\n未找到任何流媒体链接。")
		return
	}

	allM3uLinks = parser.RemoveDuplicates(allM3uLinks)
	fmt.Printf("\n总共找到 %d 个唯一的流媒体链接\n", len(allM3uLinks))

	testTimeout := 8 * time.Second
	results := tester.TestAllSources(allM3uLinks, testTimeout)

	validSources := []core.M3U8Source{}
	for _, result := range results {
		if result.Valid {
			validSources = append(validSources, result)
		}
	}

	if len(validSources) == 0 {
		fmt.Println("\n没有找到可用的直播源。")
		fmt.Println("\n部分测试结果:")
		for i, result := range results {
			if i >= 5 {
				break
			}
			fmt.Printf("URL: %s\n响应时间: %v\n状态: %s\n\n", result.URL, result.Latency, result.Error)
		}
		return
	}

	sort.Slice(validSources, func(i, j int) bool {
		return validSources[i].DownloadSpeed > validSources[j].DownloadSpeed
	})

	fmt.Printf("\n=== 找到 %d 个可用的直播源，按真实下载速度排序 ===\n\n", len(validSources))

	maxDisplay := len(validSources)
	if maxDisplay > 10 {
		maxDisplay = 10
	}

	for i := 0; i < maxDisplay; i++ {
		source := validSources[i]
		fmt.Printf("第 %d 名 (下载速度: %.2f KB/s, 延迟: %v, 数据大小: %.2f KB):\n%s\n\n",
			i+1, source.DownloadSpeed, source.Latency, float64(source.DataSize)/1024, source.URL)
	}

	fmt.Printf("=== 下载速度最快的直播源 ===\n%s\n下载速度: %.2f KB/s\n延迟: %v\n数据大小: %.2f KB\n下载时间: %v\n",
		validSources[0].URL, validSources[0].DownloadSpeed, validSources[0].Latency,
		float64(validSources[0].DataSize)/1024, validSources[0].DownloadTime)
}
