/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-01 00:00:00
 * @LastEditors: AsisYu 2773943729@qq.com
 * @LastEditTime: 2025-04-01 00:00:00
 * @FilePath: \dmainwhoseek\server\handlers\async_handlers.go
 * @Description: 
 */
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dmainwhoseek/services"

	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// AsyncWhoisQuery 
func AsyncWhoisQuery(ctx context.Context, rdb *redis.Client) (gin.H, error) {
	// 
	domain, ok := ctx.Value("domain").(string)
	if !ok || domain == "" {
		log.Printf("AsyncWhoisQuery: ")
		return nil, fmt.Errorf("domain not found")
	}

	// Redis
	cacheKey := fmt.Sprintf("whois:%s", domain)
	if cachedData, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		var response gin.H
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			log.Printf("AsyncWhoisQuery: , %s", domain)
			// 
			response["isCached"] = true
			response["cacheTime"] = time.Now().Format("2006-01-02 15:04:05")
			return response, nil
		}
	}

	// API key
	apiKey := strings.TrimSpace(os.Getenv("WHOISFREAKS_API_KEY"))

	// API key 
	log.Printf("AsyncWhoisQuery: API key , %s...", apiKey[:8])

	// 
	apiURL := fmt.Sprintf("https://api.whoisfreaks.com/v1.0/whois?apiKey=%s&whois=live&domainName=%s",
		url.QueryEscape(apiKey), // API key 
		url.QueryEscape(domain))

	log.Printf("AsyncWhoisQuery: URL (key): %s",
		strings.Replace(apiURL, apiKey, "HIDDEN", 1))

	log.Printf("AsyncWhoisQuery: API, %s", domain)

	// 
	cb := services.NewCircuitBreaker(
		10,          // 
		30*time.Second, // 
	)

	// 
	body, err := getWhoisDataWithCircuitBreaker(ctx, apiURL, cb)
	if err != nil {
		log.Printf(" : %v", err)
		return nil, fmt.Errorf("failed to query WHOIS API: %v", err)
	}

	log.Printf("AsyncWhoisQuery: API: %s", string(body))

	// 
	var whoisResp WhoisResponse
	if err := json.Unmarshal(body, &whoisResp); err != nil {
		log.Printf("AsyncWhoisQuery: API : %v", err)
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// 
	response := gin.H{
		"available":    whoisResp.DomainRegistered != "yes",
		"domain":       whoisResp.DomainName,
		"registrar":    whoisResp.DomainRegistrar.RegistrarName,
		"creationDate": whoisResp.CreateDate,
		"expiryDate":   whoisResp.ExpiryDate,
		"status":       whoisResp.DomainStatus,
		"nameServers":  whoisResp.NameServers,
		"updatedDate":  whoisResp.UpdateDate,
		"isCached":     false,
		"cacheTime":    time.Now().Format("2006-01-02 15:04:05"),
	}

	// 
	if resultJSON, err := json.Marshal(response); err == nil {
		isRegistered := !response["available"].(bool)
		hasError := false // 
		setCache(ctx, rdb, cacheKey, resultJSON, isRegistered, hasError)
	}

	// 
	response["isCached"] = false
	return response, nil
}

// u4f7fu7528u7194u65adu5668u4fddu62a4u7684WHOIS APIu8c03u7528
func getWhoisDataWithCircuitBreaker(ctx context.Context, apiURL string, cb *services.CircuitBreaker) ([]byte, error) {
	var responseBody []byte

	err := cb.Execute(func() error {
		// u6784u5efaHTTPu5ba2u6237u7aef
		client := &http.Client{Timeout: 10 * time.Second}
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %v", err)
		}

		// u6dfbu52a0u7528u6237u4ee3u7406
		req.Header.Set("User-Agent", "DomainWhoseek/1.0")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to query API: %v", err)
		}
		defer resp.Body.Close()

		// u68c0u67e5u54cdu5e94u72b6u6001u7801
		if resp.StatusCode != http.StatusOK {
			body, _ := ioutil.ReadAll(resp.Body)
			return fmt.Errorf("API returned error: status=%d, response=%s", resp.StatusCode, string(body))
		}

		// u8bfbu53d6u54cdu5e94u4f53
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read API response: %v", err)
		}
		
		// u4fddu5b58u54cdu5e94u4f53u5230u5916u90e8u53d8u91cf
		responseBody = body
		return nil
	})

	if err != nil {
		log.Printf("u7194u65adu5668u62a5u544au9519u8bef: %v", err)
		return nil, err
	}

	return responseBody, nil
}

// AsyncDNSQuery 
func AsyncDNSQuery(ctx context.Context, rdb *redis.Client) (gin.H, error) {
	// 
	domain, ok := ctx.Value("domain").(string)
	if !ok || domain == "" {
		log.Printf("AsyncDNSQuery: ")
		return nil, fmt.Errorf("domain not found")
	}

	startTime := time.Now()
	log.Printf("AsyncDNSQuery: , %s", domain)
	
	// Redis
	cacheKey := fmt.Sprintf("dns:%s", domain)
	log.Printf("AsyncDNSQuery: Redis, %s", cacheKey)
	if cachedData, err := rdb.Get(ctx, cacheKey).Result(); err == nil {
		var response DNSResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			// 
			if len(response.Records) == 0 || response.Domain == "" {
				log.Printf("AsyncDNSQuery: , %+v", response)
			} else {
				log.Printf("AsyncDNSQuery: , %s, %d", domain, len(response.Records))
				// 
				response.IsCached = true
				response.CacheTime = time.Now().Format("2006-01-02 15:04:05")
				return gin.H{
					"domain":     response.Domain,
					"records":    response.Records,
					"query_time": response.QueryTime,
					"is_cached":  response.IsCached,
					"cache_time": response.CacheTime,
				}, nil
			}
		} else {
			log.Printf("AsyncDNSQuery: , %v, %s", err, cachedData)
		}
	} else {
		log.Printf("AsyncDNSQuery: Redis, %s, %v", cacheKey, err)
	}

	// DNS 
	records := []DNSRecord{}
	log.Printf("AsyncDNSQuery: DNS , %s", domain)

	// A/AAAA 
	log.Printf("AsyncDNSQuery: A/AAAA , %s", domain)
	if ips, err := net.LookupIP(domain); err == nil {
		for _, ip := range ips {
			if ipv4 := ip.To4(); ipv4 != nil {
				records = append(records, DNSRecord{
					Type:  "A",
					Value: ipv4.String(),
				})
				log.Printf("AsyncDNSQuery: A , %s", ipv4.String())
			} else {
				records = append(records, DNSRecord{
					Type:  "AAAA",
					Value: ip.String(),
				})
				log.Printf("AsyncDNSQuery: AAAA , %s", ip.String())
			}
		}
	} else {
		log.Printf("AsyncDNSQuery: A/AAAA , %v", err)
	}

	// MX 
	log.Printf("AsyncDNSQuery: MX , %s", domain)
	if mxs, err := net.LookupMX(domain); err == nil {
		for _, mx := range mxs {
			records = append(records, DNSRecord{
				Type:  "MX",
				Value: fmt.Sprintf("%s (%d)", mx.Host, mx.Pref),
			})
			log.Printf("AsyncDNSQuery: MX , %s (%d)", mx.Host, mx.Pref)
		}
	} else {
		log.Printf("AsyncDNSQuery: MX , %v", err)
	}

	// NS 
	log.Printf("AsyncDNSQuery: NS , %s", domain)
	if nss, err := net.LookupNS(domain); err == nil {
		for _, ns := range nss {
			records = append(records, DNSRecord{
				Type:  "NS",
				Value: ns.Host,
			})
			log.Printf("AsyncDNSQuery: NS , %s", ns.Host)
		}
	} else {
		log.Printf("AsyncDNSQuery: NS , %v", err)
	}

	// TXT 
	log.Printf("AsyncDNSQuery: TXT , %s", domain)
	if txts, err := net.LookupTXT(domain); err == nil {
		for _, txt := range txts {
			records = append(records, DNSRecord{
				Type:  "TXT",
				Value: txt,
			})
			log.Printf("AsyncDNSQuery: TXT , %s", txt)
		}
	} else {
		log.Printf("AsyncDNSQuery: TXT , %v", err)
	}

	// CNAME 
	log.Printf("AsyncDNSQuery: CNAME , %s", domain)
	if cname, err := net.LookupCNAME(domain); err == nil && cname != domain+"." {
		records = append(records, DNSRecord{
			Type:  "CNAME",
			Value: strings.TrimSuffix(cname, "."),
		})
		log.Printf("AsyncDNSQuery: CNAME , %s", strings.TrimSuffix(cname, "."))
	} else if err != nil {
		log.Printf("AsyncDNSQuery: CNAME , %v", err)
	}

	// 
	response := DNSResponse{
		Domain:    domain,
		Records:   records,
		QueryTime: time.Now().Format("2006-01-02 15:04:05"),
		IsCached:  false,
		CacheTime: time.Now().Format("2006-01-02 15:04:05"),
	}

	// 
	if resultJSON, err := json.Marshal(response); err == nil {
		rdb.Set(ctx, cacheKey, resultJSON, 1*time.Hour)
		log.Printf("AsyncDNSQuery: DNS , %s, 1", domain)
	} else {
		log.Printf("AsyncDNSQuery: DNS , %v", err)
	}

	elapsedTime := time.Since(startTime)
	log.Printf("AsyncDNSQuery: , %s, %v, %d", domain, elapsedTime, len(records))

	// 
	return gin.H{
		"domain":     response.Domain,
		"records":    response.Records,
		"query_time": response.QueryTime,
		"is_cached":  response.IsCached,
		"cache_time": response.CacheTime,
	}, nil
}

// AsyncItdogScreenshot 
func AsyncItdogScreenshot(ctx context.Context, rdb *redis.Client) (gin.H, error) {
	// 
	domain, ok := ctx.Value("domain").(string)
	if !ok || domain == "" {
		log.Printf("AsyncItdogScreenshot: ")
		return nil, fmt.Errorf("domain not found")
	}

	// 
	startTime := time.Now()

	// 
	if !strings.Contains(domain, ".") {
		log.Printf("AsyncItdogScreenshot: u65e0u6548u7684u57dfu540du683cu5f0f %s", domain)
		return nil, fmt.Errorf("invalid domain format")
	}

	// 
	cacheKey := fmt.Sprintf("itdog_screenshot:%s", domain)

	// 
	cachedData, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		// 
		var response map[string]interface{}
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			log.Printf("AsyncItdogScreenshot: , %s", domain)
			response["fromCache"] = true
			return response, nil
		}
		log.Printf("AsyncItdogScreenshot: , %v", err)
	}

	// 
	screenshotDir := "./static/itdog"
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		log.Printf("AsyncItdogScreenshot: , %v", err)
		return nil, fmt.Errorf("failed to create screenshot directory: %v", err)
	}

	// 
	fileName := fmt.Sprintf("%s_%d.png", domain, time.Now().Unix())
	filePath := filepath.Join(screenshotDir, fileName)
	fileURL := fmt.Sprintf("/static/itdog/%s", fileName)

	// 
	sb := services.GetServiceBreakers()

	// 
	err = sb.ItdogBreaker.Execute(func() error {
		// 
		browserCtx, cancel := chromedp.NewContext(
			context.Background(),
			chromedp.WithLogf(log.Printf),
		)
		defer cancel()

		// 
		timeoutCtx, cancel := context.WithTimeout(browserCtx, 90*time.Second)
		defer cancel()

		// 
		var buf []byte

		// 
		err := chromedp.Run(timeoutCtx,
			// 
			chromedp.Navigate(fmt.Sprintf("https://www.itdog.cn/ping/%s", domain)),
			
			// 
			chromedp.WaitVisible(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery),
			
			// 
			chromedp.Click(".btn.btn-primary.ml-3.mb-3", chromedp.ByQuery),
			
			// 
			chromedp.Sleep(2*time.Second),
			
			// 
			chromedp.ActionFunc(func(ctx context.Context) error {
				var isDone bool
				var attempts int
				for attempts < 45 { 
					// 
					err := chromedp.Evaluate(`(() => {
						const progressBar = document.querySelector('.progress-bar');
						const nodeNum = document.querySelector('#check_node_num');
						if (!progressBar || !nodeNum) return false;
						
						// 
						const current = parseInt(progressBar.getAttribute('aria-valuenow') || '0');
						const total = parseInt(nodeNum.textContent || '0');
						
						// 
						return total > 0 && current === total;
					})()`, &isDone).Do(ctx)
					
					if err != nil {
						return err
					}
					
					if isDone {
						return nil 
					}
					
					// 
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(1 * time.Second):
						attempts++
					}
				}
				return nil 
			}),
			
			// 
			chromedp.Sleep(3*time.Second),
			
			// 
			chromedp.Screenshot("#china_map", &buf, chromedp.NodeVisible, chromedp.ByQuery),
		)
		
		if err != nil {
			log.Printf("AsyncItdogScreenshot: itdog, %v", err)
			return err
		}
		
		// 
		if err := os.WriteFile(filePath, buf, 0644); err != nil {
			log.Printf("AsyncItdogScreenshot: , %v", err)
			return err
		}
		
		return nil
	})
	
	if err != nil {
		log.Printf("AsyncItdogScreenshot: ITDog, %v", err)
		
		// 
		if err.Error() == "circuit open" {
			return gin.H{
				"success": false,
				"error":   "ITDog",
				"message": "",
			}, nil
		}
		
		// 
		return gin.H{
			"success": false,
			"error":   fmt.Sprintf("ITDog: %v", err),
		}, nil
	}
	
	// 
	response := gin.H{
		"success":   true,
		"imageUrl":  fileURL,
		"fromCache": false,
	}
	
	// 
	cacheData, _ := json.Marshal(response)
	rdb.Set(ctx, cacheKey, cacheData, 3*time.Hour) 
	
	// 
	duration := time.Since(startTime).Milliseconds()
	log.Printf("AsyncItdogScreenshot: ITDog, %s, : %dms", domain, duration)
	
	return response, nil
}

// AsyncScreenshot 
func AsyncScreenshot(ctx context.Context, rdb *redis.Client) (gin.H, error) {
	// 
	domain, ok := ctx.Value("domain").(string)
	if !ok || domain == "" {
		log.Printf("AsyncScreenshot: ")
		return nil, fmt.Errorf("domain not found")
	}

	// 
	startTime := time.Now()

	// 
	if !strings.Contains(domain, ".") {
		log.Printf("AsyncScreenshot: %s", domain)
		return nil, fmt.Errorf("invalid domain format")
	}

	// 
	cacheKey := fmt.Sprintf("screenshot:%s", domain)

	// 
	cachedData, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		// 
		var response ScreenshotResponse
		if err := json.Unmarshal([]byte(cachedData), &response); err == nil {
			log.Printf("AsyncScreenshot: u6210u529fu4eceu7f13u5b58u8bfbu53d6, %s", domain)
			response.FromCache = true
			return gin.H{
				"success":   response.Success,
				"imageUrl":  response.ImageUrl,
				"fromCache": response.FromCache,
				"error":     response.Error,
				"message":   response.Message,
			}, nil
		}
		log.Printf("AsyncScreenshot: u7f13u5b58u89e3u6790u9519u8bef, %v", err)
	}

	// u9632u6b62u91cdu590du5904u7406u5f53u524du6b63u5728u5904u7406u7684u8bf7u6c42
			// u4f7fu7528u4e00u4e2au4e34u65f6u952eu4f5cu4e3au5904u7406u6807u8bb0
		processingKey := fmt.Sprintf("processing:screenshot:%s", domain)
		exists, err := rdb.Exists(ctx, processingKey).Result()
		if err == nil && exists > 0 {
			log.Printf("AsyncScreenshot: u5df2u6709u5e76u53d1u8bf7u6c42u6b63u5728u5904u7406, %s", domain)
			return gin.H{
				"success": false,
				"message": "u6b63u5728u751fu6210u8be5u7f51u7ad9u7684u622au56feuff0cu8bf7u7a0du540eu518du8bd5",
				"error":   "concurrent_request",
			}, nil
		}
		// u8bbeu7f6eu4e34u65f6u952euff0cu6709u6548u671f30u79d2
		rdb.Set(ctx, processingKey, "1", 30*time.Second)
		defer rdb.Del(ctx, processingKey)

	// 
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		log.Printf("AsyncScreenshot: u521bu5efau622au56feu76eeu5f55u5931u8d25, %v", err)
		return nil, fmt.Errorf("failed to create screenshot directory: %v", err)
	}

	// 
	fileName := fmt.Sprintf("%s_%d.png", domain, time.Now().Unix())
	filePath := filepath.Join(screenshotDir, fileName)
	fileURL := fmt.Sprintf("/static/screenshots/%s", fileName)

	// u8bbeu7f6echromedpu9009u9879
	log.Printf("AsyncScreenshot: u5f00u59cbu751fu6210u622au56fe, %s", domain)

	// u4f7fu7528allocateu800cu4e0du662fnewcontextuff0cu53efu4ee5u66f4u597du5730u63a7u5236u6d4fu89c8u5668
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.WindowSize(1280, 720),
	)
	
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// u521bu5efau6d4fu89c8u5668u5b9eu4f8b
	chromectx, cancel := chromedp.NewContext(
		allocCtx,
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()

	// u8bbeu7f6e30u79d2u8d85u65f6
	timeoutCtx, cancel := context.WithTimeout(chromectx, 30*time.Second)
	defer cancel()

	// u6dfbu52a0u9519u8befu6355u83b7
	var buf []byte
	err = func() error {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("AsyncScreenshot: u622au56feu8fc7u7a0bu5d29u6e83u5df2u6062u590d: %v", r)
				err = fmt.Errorf("u622au56feu8fc7u7a0bu51fau73b0u672au9884u671fu9519u8bef: %v", r)
			}
		}()

		// u5c1du8bd5u901au8fc7HTTPu8fdeu63a5u6d4bu8bd5u57dfu540du662fu5426u53efu8bbfu95ee
		connSuccess := false
		testURL := fmt.Sprintf("http://%s", domain)
		testClient := &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		testReq, _ := http.NewRequest("HEAD", testURL, nil)
		testResp, testErr := testClient.Do(testReq)
		if testErr == nil {
			testResp.Body.Close()
			connSuccess = true
			log.Printf("AsyncScreenshot: u57dfu540du53efu8bbfu95eeu6027u6d4bu8bd5u6210u529f: %s", domain)
		} else {
			log.Printf("AsyncScreenshot: u57dfu540du53efu80fdu65e0u6cd5u8bbfu95ee: %s, u9519u8bef: %v", domain, testErr)
			// u6d4bu8bd5https
			httpsURL := fmt.Sprintf("https://%s", domain)
			httpsReq, _ := http.NewRequest("HEAD", httpsURL, nil)
			httpsResp, httpsErr := testClient.Do(httpsReq)
			if httpsErr == nil {
				httpsResp.Body.Close()
				connSuccess = true
				log.Printf("AsyncScreenshot: HTTPSu57dfu540du6d4bu8bd5u6210u529f: %s", domain)
			} else {
				log.Printf("AsyncScreenshot: u57dfu540du5b8cu5168u65e0u6cd5u8bbfu95ee: %s", domain)
			}
		}

		if !connSuccess {
			return fmt.Errorf("u57dfu540du65e0u6cd5u8bbfu95ee: %s", domain)
		}

		// u6267u884cu622au56feu64cdu4f5c
		return chromedp.Run(timeoutCtx,
			chromedp.Navigate(fmt.Sprintf("https://%s", domain)),
			chromedp.Sleep(5*time.Second),
			chromedp.CaptureScreenshot(&buf),
		)
	}()

	if err != nil {
		log.Printf("AsyncScreenshot: u622au56feu5931u8d25, %v", err)
		
		// u5904u7406u5e38u89c1u9519u8bef
		if strings.Contains(err.Error(), "net::ERR_NAME_NOT_RESOLVED") || 
		   strings.Contains(err.Error(), "net::ERR_CONNECTION_REFUSED") ||
		   strings.Contains(err.Error(), "net::ERR_CONNECTION_TIMED_OUT") ||
		   strings.Contains(err.Error(), "net::ERR_CONNECTION_RESET") ||
		   strings.Contains(err.Error(), "net::ERR_INTERNET_DISCONNECTED") ||
		   strings.Contains(err.Error(), "u57dfu540du65e0u6cd5u8bbfu95ee") {
			// u8fd4u56deu53efu8bfbu7684u9519u8befu4fe1u606f
			response := ScreenshotResponse{
				Success:   false,
				FromCache: false,
				Error:     "domain_unreachable",
				Message:   fmt.Sprintf("u65e0u6cd5u8bbfu95eeu57dfu540d: %s", domain),
			}

			// u7f13u5b58u5931u8d25u7ed3u679cuff0cu4f46u65f6u95f4u8f83u77ed
			if responseJSON, err := json.Marshal(response); err == nil {
				rdb.Set(ctx, cacheKey, responseJSON, 1*time.Hour) // u7f13u5b58u4e00u5c0fu65f6
			}
			
			return gin.H{
				"success": false,
				"error":   "domain_unreachable",
				"message": fmt.Sprintf("u65e0u6cd5u8bbfu95eeu57dfu540d: %s", domain),
			}, nil
		}
		
		// u5176u4ed6u9519u8bef
		return gin.H{
			"success": false,
			"error":   "screenshot_failed",
			"message": fmt.Sprintf("u622au56feu8fc7u7a0bu51fau9519: %v", err),
		}, nil
	}

	// u4fddu5b58u622au56feu5230u6587u4ef6
	if err := os.WriteFile(filePath, buf, 0644); err != nil {
		log.Printf("AsyncScreenshot: u4fddu5b58u622au56feu5931u8d25, %v", err)
		return nil, fmt.Errorf("failed to save screenshot: %v", err)
	}

	// u6784u5efau54cdu5e94
	response := ScreenshotResponse{
		Success:   true,
		ImageUrl:  fileURL,
		FromCache: false,
	}

	// u7f13u5b58u622au56feu7ed3u679c
	if responseJSON, err := json.Marshal(response); err == nil {
		rdb.Set(ctx, cacheKey, responseJSON, screenshotCacheDuration)
		log.Printf("AsyncScreenshot: u5df2u7f13u5b58u622au56feu7ed3u679c, %s, u7f13u5b58u671f24u5c0fu65f6", domain)
	} else {
		log.Printf("AsyncScreenshot: u7f13u5b58u622au56feu7ed3u679cu5931u8d25, %v", err)
	}

	// u8bb0u5f55u6267u884cu65f6u95f4
	duration := time.Since(startTime).Milliseconds()
	log.Printf("AsyncScreenshot: u622au56feu5b8cu6210, %s, u8017u65f6%dms", domain, duration)

	return gin.H{
		"success":   response.Success,
		"imageUrl":  response.ImageUrl,
		"fromCache": response.FromCache,
		"error":     response.Error,
		"message":   response.Message,
	}, nil
}
