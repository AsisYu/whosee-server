/*
 * @Author: AsisYu
 * @Date: 2025-04-24
 * @Description: 统一API处理程序
 */
package handlers

import (
	"context"
	"dmainwhoseek/services"
	"dmainwhoseek/utils"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// WhoisHandler WHOIS查询处理程序
func WhoisHandler(c *gin.Context) {
	// 从上下文获取必要的服务和数据
	domain, _ := c.Get("domain")
	domainStr := domain.(string)
	resultChan, _ := c.Get("resultChan")
	errorChan, _ := c.Get("errorChan")
	reqCtx, _ := c.Get("requestContext")
	workerPool, _ := c.Get("workerPool")
	whoisManager, _ := c.Get("whoisManager")

	// 类型断言
	results := resultChan.(chan interface{})
	errors := errorChan.(chan error)
	requestContext := reqCtx.(context.Context)
	pool := workerPool.(*services.WorkerPool)
	manager := whoisManager.(*services.WhoisManager)

	// 提交任务到工作池
	submitted := pool.SubmitWithContext(requestContext, func() {
		// 记录开始处理时间
		startTime := time.Now()

		// 使用WhoisManager进行查询
		log.Printf("[WHOIS] 查询域名: %s", domainStr)
		response, err, fromCache := manager.Query(domainStr)

		if err != nil {
			log.Printf("[WHOIS] 查询域名 %s 失败: %v", domainStr, err)
			errors <- err
			return
		}

		// 计算处理时间
		processingTime := time.Since(startTime).Milliseconds()

		// 构建元数据
		meta := &utils.MetaInfo{
			Timestamp:  time.Now().Format(time.RFC3339),
			Cached:     fromCache,
			CachedAt:   response.CachedAt,
			Processing: processingTime,
		}

		// 构建响应数据
		result := gin.H{
			"available":      response.Available,
			"domain":         response.Domain,
			"registrar":      response.Registrar,
			"creationDate":   response.CreateDate,
			"expiryDate":     response.ExpiryDate,
			"status":         response.Status,
			"nameServers":    response.NameServers,
			"updatedDate":    response.UpdateDate,
			"statusCode":     response.StatusCode,
			"statusMessage":  response.StatusMessage,
			"sourceProvider": response.SourceProvider,
		}

		// 简化缓存日志
		if !fromCache {
			log.Printf("[WHOIS] 查询域名 %s 完成，新数据，处理时间: %dms", domainStr, processingTime)
		} else {
			log.Printf("[WHOIS] 查询域名 %s 完成，使用缓存数据，原始缓存时间: %s，处理时间: %dms", domainStr, response.CachedAt, processingTime)
		}

		// 发送结果
		results <- gin.H{
			"data": result,
			"meta": meta,
		}
	})

	if !submitted {
		log.Printf("[WHOIS] 查询域名 %s 失败: 工作池忙碌", domainStr)
		utils.ErrorResponse(c, 503, "SERVICE_BUSY", "Service is busy, please try again later")
		return
	}

	// 等待结果或超时
	select {
	case result := <-results:
		data := result.(gin.H)
		utils.SuccessResponse(c, data["data"], data["meta"].(*utils.MetaInfo))
		log.Printf("[WHOIS] 返回域名 %s 的查询结果", domainStr)
	case err := <-errors:
		log.Printf("[WHOIS] 处理域名 %s 查询请求失败: %v", domainStr, err)
		utils.ErrorResponse(c, 500, "QUERY_ERROR", err.Error())
	case <-requestContext.Done():
		log.Printf("[WHOIS] 查询域名 %s 超时", domainStr)
		utils.ErrorResponse(c, 504, "TIMEOUT", "Request timed out")
	}
}

// DNSHandler DNS查询处理程序
func DNSHandler(c *gin.Context) {
	// 从上下文获取必要的服务和数据
	domain, _ := c.Get("domain")
	domainStr := domain.(string)
	resultChan, _ := c.Get("resultChan")
	errorChan, _ := c.Get("errorChan")
	reqCtx, _ := c.Get("requestContext")
	workerPool, _ := c.Get("workerPool")

	// 类型断言
	results := resultChan.(chan interface{})
	errors := errorChan.(chan error)
	requestContext := reqCtx.(context.Context)
	pool := workerPool.(*services.WorkerPool)

	// 获取DNS检查器
	dnsCheckerValue, exists := c.Get("dnsChecker")
	if !exists || dnsCheckerValue == nil {
		utils.ErrorResponse(c, 500, "SERVICE_UNAVAILABLE", "DNS service not available")
		return
	}
	dnsChecker := dnsCheckerValue.(*services.DNSChecker)

	// 获取Redis客户端
	redisClient, _ := c.Get("redis")

	// 提交任务到工作池
	submitted := pool.SubmitWithContext(requestContext, func() {
		// 记录开始处理时间
		startTime := time.Now()

		// 创建一个包含域名和Redis客户端的上下文
		ctxWithDomain := context.WithValue(requestContext, "domain", domainStr)
		ctxWithRedis := context.WithValue(ctxWithDomain, "redis", redisClient)

		// 调用DNS查询
		log.Printf("[DNS] 工作池开始处理域名 %s 的DNS查询", domainStr)
		result, err := AsyncDNSQuery(ctxWithRedis, dnsChecker)

		if err != nil {
			log.Printf("[DNS] 查询域名 %s 的DNS记录失败: %v", domainStr, err)
			errors <- err
			return
		}

		// 计算处理时间
		processingTime := time.Since(startTime).Milliseconds()

		// 构建元数据
		meta := &utils.MetaInfo{
			Timestamp:  time.Now().Format(time.RFC3339),
			Processing: processingTime,
		}

		log.Printf("[DNS] 成功获取域名 %s 的DNS记录，处理时间: %dms", domainStr, processingTime)

		// 发送结果
		results <- gin.H{
			"data": result,
			"meta": meta,
		}
	})

	if !submitted {
		log.Printf("[DNS] 查询域名 %s 的DNS记录失败: 工作池忙碌", domainStr)
		utils.ErrorResponse(c, 503, "SERVICE_BUSY", "Service is busy, please try again later")
		return
	}

	// 等待结果或超时
	select {
	case result := <-results:
		data := result.(gin.H)
		utils.SuccessResponse(c, data["data"], data["meta"].(*utils.MetaInfo))
		log.Printf("[DNS] 成功返回域名 %s 的DNS查询结果", domainStr)
	case err := <-errors:
		log.Printf("[DNS] 处理域名 %s 的DNS查询时出错: %v", domainStr, err)
		utils.ErrorResponse(c, 500, "QUERY_ERROR", err.Error())
	case <-requestContext.Done():
		log.Printf("[DNS] 查询域名 %s 的DNS记录超时", domainStr)
		utils.ErrorResponse(c, 504, "TIMEOUT", "Request timed out")
	}
}

// ScreenshotHandler 网站截图处理程序
func ScreenshotHandler(c *gin.Context) {
	// 从上下文获取必要的服务和数据
	domain, _ := c.Get("domain")
	domainStr := domain.(string)
	resultChan, _ := c.Get("resultChan")
	errorChan, _ := c.Get("errorChan")
	reqCtx, _ := c.Get("requestContext")
	workerPool, _ := c.Get("workerPool")

	// 类型断言
	results := resultChan.(chan interface{})
	errors := errorChan.(chan error)
	requestContext := reqCtx.(context.Context)
	pool := workerPool.(*services.WorkerPool)

	// 获取截图检查器
	screenshotCheckerValue, exists := c.Get("screenshotChecker")
	if !exists || screenshotCheckerValue == nil {
		utils.ErrorResponse(c, 500, "SERVICE_UNAVAILABLE", "Screenshot service not available")
		return
	}
	screenshotChecker := screenshotCheckerValue.(*services.ScreenshotChecker)

	// 获取Redis客户端
	redisClient, _ := c.Get("redis")

	// 提交任务到工作池
	submitted := pool.SubmitWithContext(requestContext, func() {
		// 记录开始处理时间
		startTime := time.Now()

		// 创建一个包含域名和Redis客户端的上下文
		ctxWithDomain := context.WithValue(requestContext, "domain", domainStr)
		ctxWithRedis := context.WithValue(ctxWithDomain, "redis", redisClient)

		// 调用截图服务
		log.Printf("[Screenshot] 开始处理域名 %s 的截图请求", domainStr)
		result, err := AsyncScreenshot(ctxWithRedis, screenshotChecker)

		if err != nil {
			log.Printf("[Screenshot] 截图域名 %s 失败: %v", domainStr, err)
			errors <- err
			return
		}

		// 计算处理时间
		processingTime := time.Since(startTime).Milliseconds()

		// 构建元数据
		meta := &utils.MetaInfo{
			Timestamp:  time.Now().Format(time.RFC3339),
			Processing: processingTime,
		}

		log.Printf("[Screenshot] 成功生成域名 %s 的截图，处理时间: %dms", domainStr, processingTime)

		// 发送结果
		results <- gin.H{
			"data": result,
			"meta": meta,
		}
	})

	if !submitted {
		log.Printf("[Screenshot] 截图域名 %s 失败: 工作池忙碌", domainStr)
		utils.ErrorResponse(c, 503, "SERVICE_BUSY", "Service is busy, please try again later")
		return
	}

	// 等待结果或超时
	select {
	case result := <-results:
		data := result.(gin.H)
		utils.SuccessResponse(c, data["data"], data["meta"].(*utils.MetaInfo))
		log.Printf("[Screenshot] 成功返回域名 %s 的截图结果", domainStr)
	case err := <-errors:
		log.Printf("[Screenshot] 处理域名 %s 的截图请求时出错: %v", domainStr, err)
		utils.ErrorResponse(c, 500, "SCREENSHOT_ERROR", err.Error())
	case <-requestContext.Done():
		log.Printf("[Screenshot] 截图域名 %s 超时", domainStr)
		utils.ErrorResponse(c, 504, "TIMEOUT", "Request timed out")
	}
}

// ITDogHandler ITDog测速截图处理程序
func ITDogHandler(c *gin.Context) {
	// 从上下文获取必要的服务和数据
	domain, _ := c.Get("domain")
	domainStr := domain.(string)
	resultChan, _ := c.Get("resultChan")
	errorChan, _ := c.Get("errorChan")
	reqCtx, _ := c.Get("requestContext")
	workerPool, _ := c.Get("workerPool")

	// 类型断言
	results := resultChan.(chan interface{})
	errors := errorChan.(chan error)
	requestContext := reqCtx.(context.Context)
	pool := workerPool.(*services.WorkerPool)

	// 获取ITDog检查器
	itdogCheckerValue, exists := c.Get("itdogChecker")
	if !exists || itdogCheckerValue == nil {
		utils.ErrorResponse(c, 500, "SERVICE_UNAVAILABLE", "ITDog service not available")
		return
	}
	itdogChecker := itdogCheckerValue.(*services.ITDogChecker)

	// 获取Redis客户端
	redisClient, _ := c.Get("redis")

	// 提交任务到工作池
	submitted := pool.SubmitWithContext(requestContext, func() {
		// 记录开始处理时间
		startTime := time.Now()

		// 创建一个包含域名和Redis客户端的上下文
		ctxWithDomain := context.WithValue(requestContext, "domain", domainStr)
		ctxWithRedis := context.WithValue(ctxWithDomain, "redis", redisClient)

		// 调用ITDog服务
		log.Printf("[ITDog] 开始处理域名 %s 的ITDog测速截图请求", domainStr)
		result, err := AsyncItdogScreenshot(ctxWithRedis, itdogChecker)

		if err != nil {
			log.Printf("[ITDog] 处理域名 %s 的ITDog测速截图失败: %v", domainStr, err)
			errors <- err
			return
		}

		// 计算处理时间
		processingTime := time.Since(startTime).Milliseconds()

		// 构建元数据
		meta := &utils.MetaInfo{
			Timestamp:  time.Now().Format(time.RFC3339),
			Processing: processingTime,
		}

		log.Printf("[ITDog] 成功生成域名 %s 的ITDog测速截图，处理时间: %dms", domainStr, processingTime)

		// 发送结果
		results <- gin.H{
			"data": result,
			"meta": meta,
		}
	})

	if !submitted {
		log.Printf("[ITDog] 处理域名 %s 的ITDog测速截图失败: 工作池忙碌", domainStr)
		utils.ErrorResponse(c, 503, "SERVICE_BUSY", "Service is busy, please try again later")
		return
	}

	// 等待结果或超时
	select {
	case result := <-results:
		data := result.(gin.H)
		utils.SuccessResponse(c, data["data"], data["meta"].(*utils.MetaInfo))
		log.Printf("[ITDog] 成功返回域名 %s 的ITDog测速截图结果", domainStr)
	case err := <-errors:
		log.Printf("[ITDog] 处理域名 %s 的ITDog测速截图请求时出错: %v", domainStr, err)
		utils.ErrorResponse(c, 500, "ITDOG_ERROR", err.Error())
	case <-requestContext.Done():
		log.Printf("[ITDog] 处理域名 %s 的ITDog测速截图超时", domainStr)
		utils.ErrorResponse(c, 504, "TIMEOUT", "Request timed out")
	}
}

// RDAPHandler RDAP查询处理程序
func RDAPHandler(c *gin.Context) {
	// 从上下文获取必要的服务和数据
	domain, _ := c.Get("domain")
	domainStr := domain.(string)
	resultChan, _ := c.Get("resultChan")
	errorChan, _ := c.Get("errorChan")
	reqCtx, _ := c.Get("requestContext")
	workerPool, _ := c.Get("workerPool")
	whoisManager, _ := c.Get("whoisManager")

	// 类型断言
	results := resultChan.(chan interface{})
	errors := errorChan.(chan error)
	requestContext := reqCtx.(context.Context)
	pool := workerPool.(*services.WorkerPool)
	manager := whoisManager.(*services.WhoisManager)

	// 提交任务到工作池
	submitted := pool.SubmitWithContext(requestContext, func() {
		// 记录开始处理时间
		startTime := time.Now()

		// 使用WhoisManager专门查询IANA-RDAP提供商
		log.Printf("[RDAP] 查询域名: %s", domainStr)
		response, err, fromCache := manager.QueryWithProvider(domainStr, "IANA-RDAP")

		if err != nil {
			log.Printf("[RDAP] 查询域名 %s 失败: %v", domainStr, err)
			errors <- err
			return
		}

		// 计算处理时间
		processingTime := time.Since(startTime).Milliseconds()

		// 构建元数据
		meta := &utils.MetaInfo{
			Timestamp:  time.Now().Format(time.RFC3339),
			Cached:     fromCache,
			CachedAt:   response.CachedAt,
			Processing: processingTime,
		}

		// 构建RDAP专用响应数据
		result := gin.H{
			"available":      response.Available,
			"domain":         response.Domain,
			"registrar":      response.Registrar,
			"creationDate":   response.CreateDate,
			"expiryDate":     response.ExpiryDate,
			"status":         response.Status,
			"nameServers":    response.NameServers,
			"updatedDate":    response.UpdateDate,
			"statusCode":     response.StatusCode,
			"statusMessage":  response.StatusMessage,
			"sourceProvider": "IANA-RDAP",
			"protocol":       "RDAP",
		}

		// 简化缓存日志
		if !fromCache {
			log.Printf("[RDAP] 查询域名 %s 完成，新数据，处理时间: %dms", domainStr, processingTime)
		} else {
			log.Printf("[RDAP] 查询域名 %s 完成，使用缓存数据，原始缓存时间: %s，处理时间: %dms", domainStr, response.CachedAt, processingTime)
		}

		// 发送结果
		results <- gin.H{
			"data": result,
			"meta": meta,
		}
	})

	if !submitted {
		log.Printf("[RDAP] 查询域名 %s 失败: 工作池忙碌", domainStr)
		utils.ErrorResponse(c, 503, "SERVICE_BUSY", "Service is busy, please try again later")
		return
	}

	// 等待结果或超时
	select {
	case result := <-results:
		data := result.(gin.H)
		utils.SuccessResponse(c, data["data"], data["meta"].(*utils.MetaInfo))
		log.Printf("[RDAP] 返回域名 %s 的RDAP查询结果", domainStr)
	case err := <-errors:
		log.Printf("[RDAP] 处理域名 %s RDAP查询请求失败: %v", domainStr, err)
		utils.ErrorResponse(c, 500, "QUERY_ERROR", err.Error())
	case <-requestContext.Done():
		log.Printf("[RDAP] 查询域名 %s 超时", domainStr)
		utils.ErrorResponse(c, 504, "TIMEOUT", "Request timed out")
	}
}
