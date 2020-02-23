package sender

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/jaeles-project/jaeles/utils"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jaeles-project/jaeles/libs"
	"github.com/sirupsen/logrus"
)

// JustSend just sending request
func JustSend(options libs.Options, req libs.Request) (res libs.Response, err error) {
	method := req.Method
	url := req.URL
	body := req.Body
	headers := GetHeaders(req)

	// update it again
	var newHeader []map[string]string
	for k, v := range headers {
		element := make(map[string]string)
		element[k] = v
		newHeader = append(newHeader, element)
	}
	req.Headers = newHeader

	// disable log when retry
	logger := logrus.New()
	if !options.Debug {
		logger.Out = ioutil.Discard
	}
	client := resty.New()
	client.SetLogger(logger)
	client.SetCloseConnection(true)

	// setting for client
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	client.SetDisableWarn(true)
	client.SetHeaders(headers)

	// redirect policy
	if req.Redirect == false {
		client.SetRedirectPolicy(resty.RedirectPolicyFunc(func(req *http.Request, via []*http.Request) error {
			// keep the header the same
			// client.SetHeaders(headers)

			res.StatusCode = req.Response.StatusCode
			res.Status = req.Response.Status
			resp := req.Response
			bodyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				utils.ErrorF("%v", err)
			}
			bodyString := string(bodyBytes)
			resLength := len(bodyString)
			// format the headers
			var resHeaders []map[string]string
			for k, v := range resp.Header {
				element := make(map[string]string)
				//fmt.Printf("%v: %v\n", k, v)
				element[k] = strings.Join(v[:], "")
				resLength += len(fmt.Sprintf("%s: %s\n", k, strings.Join(v[:], "")))
				resHeaders = append(resHeaders, element)
			}

			// response time in second
			resTime := 0.0
			resHeaders = append(resHeaders,
				map[string]string{"Total Length": strconv.Itoa(resLength)},
				map[string]string{"Response Time": fmt.Sprintf("%f", resTime)},
			)

			// set some variable
			res.Headers = resHeaders
			res.StatusCode = resp.StatusCode
			res.Status = fmt.Sprintf("%v %v", resp.Status, resp.Proto)
			res.Body = bodyString
			res.ResponseTime = resTime
			res.Length = resLength
			// beautify
			res.Beautify = BeautifyResponse(res)
			return errors.New("auto redirect is disabled")
		}))

		client.AddRetryCondition(
			func(r *resty.Response, err error) bool {
				return false
			},
		)

	} else {
		client.SetRedirectPolicy(resty.RedirectPolicyFunc(func(req *http.Request, via []*http.Request) error {
			// keep the header the same
			client.SetHeaders(headers)
			return nil
		}))
	}

	if options.Retry > 0 {
		client.SetRetryCount(options.Retry)
	}
	client.SetTimeout(time.Duration(options.Timeout) * time.Second)
	if req.Timeout > 0 {
		client.SetTimeout(time.Duration(req.Timeout) * time.Second)
	}

	client.SetRetryWaitTime(time.Duration(options.Timeout/2) * time.Second)
	client.SetRetryMaxWaitTime(time.Duration(options.Timeout) * time.Second)

	if options.Proxy != "" {
		client.SetProxy(options.Proxy)
	}
	// override proxy
	if req.Proxy != "" && req.Proxy != "blank" {
		client.SetProxy(req.Proxy)
	}

	var resp *resty.Response
	// really sending things here
	switch method {
	case "GET":
		resp, err = client.R().
			SetBody([]byte(body)).
			Get(url)
		break
	case "POST":
		resp, err = client.R().
			SetBody([]byte(body)).
			Post(url)
		break
	case "HEAD":
		resp, err = client.R().
			SetBody([]byte(body)).
			Head(url)
		break
	case "OPTIONS":
		resp, err = client.R().
			SetBody([]byte(body)).
			Options(url)
		break
	case "PATCH":
		resp, err = client.R().
			SetBody([]byte(body)).
			Patch(url)
		break
	case "PUT":
		resp, err = client.R().
			SetBody([]byte(body)).
			Put(url)
		break
	case "DELETE":
		resp, err = client.R().
			SetBody([]byte(body)).
			Delete(url)
		break
	}

	// in case we want to get redirect stuff
	if res.StatusCode != 0 {
		return res, nil
	}

	if err != nil || resp == nil {
		utils.ErrorF("%v %v", url, err)
		return libs.Response{}, err
	}

	return ParseResponse(*resp), nil
}

// ParseResponse field to Response
func ParseResponse(resp resty.Response) (res libs.Response) {
	// var res libs.Response
	resLength := len(string(resp.Body()))
	// format the headers
	var resHeaders []map[string]string
	for k, v := range resp.RawResponse.Header {
		element := make(map[string]string)
		element[k] = strings.Join(v[:], "")
		resLength += len(fmt.Sprintf("%s: %s\n", k, strings.Join(v[:], "")))
		resHeaders = append(resHeaders, element)
	}
	// response time in second
	resTime := float64(resp.Time()) / float64(time.Second)
	resHeaders = append(resHeaders,
		map[string]string{"Total Length": strconv.Itoa(resLength)},
		map[string]string{"Response Time": fmt.Sprintf("%f", resTime)},
	)

	// set some variable
	res.Headers = resHeaders
	res.StatusCode = resp.StatusCode()
	res.Status = fmt.Sprintf("%v %v", resp.Status(), resp.RawResponse.Proto)
	res.Body = string(resp.Body())
	res.ResponseTime = resTime
	res.Length = resLength
	// beautify
	res.Beautify = BeautifyResponse(res)
	return res
}

// GetHeaders generate headers if not provide
func GetHeaders(req libs.Request) map[string]string {
	// random user agent
	UserAgens := []string{
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3941.0 Safari/537.36",
		"Mozilla/5.0 (X11; U; Windows NT 6; en-US) AppleWebKit/534.12 (KHTML, like Gecko) Chrome/9.0.587.0 Safari/534.12",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/74.0.3729.169 Safari/537.36",
	}

	headers := make(map[string]string)
	if len(req.Headers) == 0 {
		rand.Seed(time.Now().Unix())
		headers["User-Agent"] = UserAgens[rand.Intn(len(UserAgens))]
		return headers
	}

	for _, header := range req.Headers {
		for key, value := range header {
			headers[key] = value
		}
	}

	rand.Seed(time.Now().Unix())
	// append user agent in case you didn't set user-agent
	if headers["User-Agent"] == "" {
		rand.Seed(time.Now().Unix())
		headers["User-Agent"] = UserAgens[rand.Intn(len(UserAgens))]
	}
	return headers
}

// BeautifyRequest beautify request
func BeautifyRequest(req libs.Request) string {
	var beautifyReq string
	// hardcoded HTTP/1.1 for now
	beautifyReq += fmt.Sprintf("%v %v HTTP/1.1\n", req.Method, req.URL)

	for _, header := range req.Headers {
		for key, value := range header {
			if key != "" && value != "" {
				beautifyReq += fmt.Sprintf("%v: %v\n", key, value)
			}
		}
	}
	if req.Body != "" {
		beautifyReq += fmt.Sprintf("\n%v\n", req.Body)
	}
	return beautifyReq
}

// BeautifyResponse beautify response
func BeautifyResponse(res libs.Response) string {
	var beautifyRes string
	beautifyRes += fmt.Sprintf("%v \n", res.Status)

	for _, header := range res.Headers {
		for key, value := range header {
			beautifyRes += fmt.Sprintf("%v: %v\n", key, value)
		}
	}

	beautifyRes += fmt.Sprintf("\n%v\n", res.Body)
	return beautifyRes
}
