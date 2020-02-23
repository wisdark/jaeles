package core

import (
	"bufio"
	"fmt"
	"github.com/jaeles-project/jaeles/utils"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/jaeles-project/jaeles/database"
	"github.com/jaeles-project/jaeles/libs"
	"github.com/thoas/go-funk"
	"gopkg.in/yaml.v2"
)

// ParseSign parsing YAML signature file
func ParseSign(signFile string) (sign libs.Signature, err error) {
	yamlFile, err := ioutil.ReadFile(signFile)
	if err != nil {
		utils.ErrorF("yamlFile.Get err  #%v - %v", err, signFile)
	}
	err = yaml.Unmarshal(yamlFile, &sign)
	if err != nil {
		utils.ErrorF("Error: %v - %v", err, signFile)
	}
	// set some default value
	if sign.Info.Category == "" {
		if strings.Contains(sign.ID, "-") {
			sign.Info.Category = strings.Split(sign.ID, "-")[0]
		} else {
			sign.Info.Category = sign.ID
		}
	}
	if sign.Info.Name == "" {
		sign.Info.Name = sign.ID
	}
	if sign.Info.Risk == "" {
		sign.Info.Risk = "Potential"
	}
	return sign, err
}

// ParsePassive parsing YAML passive file
func ParsePassive(passiveFile string) (passive libs.Passive, err error) {
	yamlFile, err := ioutil.ReadFile(passiveFile)
	if err != nil {
		utils.ErrorF("yamlFile.Get err  #%v - %v", err, passiveFile)
	}
	err = yaml.Unmarshal(yamlFile, &passive)
	if err != nil {
		utils.ErrorF("Error: %v - %v", err, passiveFile)
	}
	return passive, err
}

// ParseTarget parsing target and some variable for template
func ParseTarget(raw string) map[string]string {
	target := make(map[string]string)
	target["Raw"] = raw
	if raw == "" {
		return target
	}
	u, err := url.Parse(raw)

	// something wrong so parsing it again
	if err != nil || u.Scheme == "" || strings.Contains(u.Scheme, ".") {
		raw = fmt.Sprintf("https://%v", raw)
		u, err = url.Parse(raw)
		if err != nil {
			return target
		}
	}
	var hostname string
	var query string
	port := u.Port()
	// var domain string
	domain := u.Hostname()

	query = u.RawQuery
	if u.Port() == "" {
		if strings.Contains(u.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}

		hostname = u.Hostname()
	} else {
		// ignore common port in Host
		if u.Port() == "443" || u.Port() == "80" {
			hostname = u.Hostname()
		} else {
			hostname = u.Hostname() + ":" + u.Port()
		}
	}

	target["Scheme"] = u.Scheme
	target["Path"] = u.Path
	target["Domain"] = domain
	target["Host"] = hostname
	target["Port"] = port
	target["RawQuery"] = query

	if (target["RawQuery"] != "") && (port == "80" || port == "443") {
		target["URL"] = fmt.Sprintf("%v://%v%v?%v", target["Scheme"], target["Host"], target["Path"], target["RawQuery"])
	} else if port != "80" && port != "443" {
		target["URL"] = fmt.Sprintf("%v://%v:%v%v?%v", target["Scheme"], target["Domain"], target["Port"], target["Path"], target["RawQuery"])
	} else {
		target["URL"] = fmt.Sprintf("%v://%v%v", target["Scheme"], target["Host"], target["Path"])
	}

	uu, _ := url.Parse(raw)
	target["BaseURL"] = fmt.Sprintf("%v://%v", uu.Scheme, uu.Host)
	target["Extension"] = filepath.Ext(target["BaseURL"])
	return target
}

// MoreVariables get more options to render in sign template
func MoreVariables(target map[string]string, sign libs.Signature, options libs.Options) map[string]string {
	realTarget := target

	ssrf := database.GetDefaultBurpCollab()
	if ssrf != "" {
		target["oob"] = ssrf
	} else {
		target["oob"] = database.GetCollab()
	}

	// more options
	realTarget["rootPath"] = options.RootFolder
	realTarget["resourcePath"] = options.ResourcesFolder
	realTarget["proxy"] = options.Proxy
	realTarget["output"] = options.Output

	// default params in signature
	signParams := sign.Params
	if len(signParams) > 0 {
		for _, param := range signParams {
			for k, v := range param {
				// variable as a script
				if strings.Contains(v, "(") && strings.Contains(v, ")") {
					newValue := RunVariables(v)
					if len(newValue) > 0 {
						realTarget[k] = newValue[0]
					}
				} else {
					realTarget[k] = v
				}
			}
		}
	}

	// more params
	if len(options.Params) > 0 {
		params := ParseParams(options.Params)
		if len(params) > 0 {
			for k, v := range params {
				realTarget[k] = v
			}
		}
	}
	return realTarget
}

// ParseParams parse more params from cli
func ParseParams(rawParams []string) map[string]string {
	params := make(map[string]string)

	for _, item := range rawParams {
		if strings.Contains(item, "=") {
			data := strings.Split(item, "=")
			params[data[0]] = strings.Replace(item, data[0]+"=", "", -1)
		}
	}
	return params
}

// ParseOrigin parse origin request
func ParseOrigin(req libs.Request, sign libs.Signature, options libs.Options) libs.Request {
	target := sign.Target
	// resolve some parts with global variables first
	req.Target = target
	req.URL = ResolveVariable(req.URL, target)
	// @NOTE: backward compatible
	if req.URL == "" && req.Path != "" {
		req.URL = ResolveVariable(req.Path, target)
	}
	req.Body = ResolveVariable(req.Body, target)
	req.Headers = ResolveHeader(req.Headers, target)
	req.Middlewares = ResolveDetection(req.Middlewares, target)
	req.Conclusions = ResolveDetection(req.Conclusions, target)

	// parse raw request
	if req.Raw != "" {
		rawReq := ResolveVariable(req.Raw, target)
		burpReq := ParseBurpRequest(rawReq)
		burpReq.Detections = ResolveDetection(req.Detections, target)
		burpReq.Middlewares = ResolveDetection(req.Middlewares, target)
		burpReq.Conclusions = ResolveDetection(req.Conclusions, target)
		return burpReq
	}
	return req
}

// ParseRequest parse request part in YAML signature file
func ParseRequest(req libs.Request, sign libs.Signature, options libs.Options) []libs.Request {
	var Reqs []libs.Request
	target := sign.Target

	// resolve some parts with global variables first
	req.Target = target
	req.URL = ResolveVariable(req.URL, target)
	// @NOTE: backward compatible
	if req.URL == "" && req.Path != "" {
		req.URL = ResolveVariable(req.Path, target)
	}
	req.Body = ResolveVariable(req.Body, target)
	req.Headers = ResolveHeader(req.Headers, target)
	req.Middlewares = ResolveDetection(req.Middlewares, target)
	req.Conditions = ResolveDetection(req.Conditions, target)

	if sign.Type != "fuzz" {
		// in case we only want to run a middleware alone
		if req.Raw != "" {
			rawReq := ResolveVariable(req.Raw, target)
			burpReq := ParseBurpRequest(rawReq)
			burpReq.Detections = ResolveDetection(req.Detections, target)
			burpReq.Middlewares = ResolveDetection(req.Middlewares, target)
			Reqs = append(Reqs, burpReq)
		}

		// if req.path is blank
		if req.URL == "" && funk.IsEmpty(req.Middlewares) {
			return Reqs
		} else if !funk.IsEmpty(req.Middlewares) {
			Req := req
			Req.Middlewares = ResolveDetection(req.Middlewares, target)
			Reqs = append(Reqs, Req)
			return Reqs
		}
		req.Detections = ResolveDetection(req.Detections, target)
		// normal requests here
		Req := req
		Req.Redirect = req.Redirect
		if Req.URL != "" {
			Reqs = append(Reqs, Req)
		}
		return Reqs
	}

	// start parse fuzz req
	// only take URL as a input from cli
	var record libs.Record

	// parse raw request in case we have -r options as a origin request
	if req.Raw != "" {
		rawReq := ResolveVariable(req.Raw, target)
		burpReq := ParseBurpRequest(rawReq)
		// resolve again with custom delimiter generator
		burpReq.Generators = req.Generators
		burpReq.Detections = req.Detections
		burpReq.Middlewares = req.Middlewares
		record.OriginReq = burpReq
	} else {
		record.OriginReq.URL = target["URL"]
	}

	record.Request = req
	reqs := ParseFuzzRequest(record, sign)
	if len(reqs) > 0 {
		Reqs = append(Reqs, reqs...)
	}
	utils.DebugF("[New Parsed Reuqest] %v", len(Reqs))

	// repeat section
	if req.Repeat == 0 {
		return Reqs
	}
	realReqsWithRepeat := Reqs
	for i := 0; i < req.Repeat-1; i++ {
		realReqsWithRepeat = append(realReqsWithRepeat, Reqs...)
	}
	return realReqsWithRepeat
}

// ParseFuzzRequest parse request recive in API server
func ParseFuzzRequest(record libs.Record, sign libs.Signature) []libs.Request {
	req := record.Request

	var Reqs []libs.Request
	// color.Green("-- Start do Injecting")
	if req.URL == "" {
		req.URL = record.OriginReq.URL
	}
	Reqs = Generators(req, sign)
	return Reqs
}

// ParsePayloads parse payload to replace
func ParsePayloads(sign libs.Signature) []string {
	var rawPayloads []string
	rawPayloads = append(rawPayloads, sign.Payloads...)
	// strip out blank line
	for index, value := range rawPayloads {
		if strings.Trim(value, " ") == "" {
			rawPayloads = append(rawPayloads[:index], rawPayloads[index+1:]...)
		}
	}
	return rawPayloads
}

// ParseBurpRequest parse burp style request
func ParseBurpRequest(raw string) (req libs.Request) {
	var realReq libs.Request
	realReq.Raw = raw
	reader := bufio.NewReader(strings.NewReader(raw))
	parsedReq, err := http.ReadRequest(reader)
	if err != nil {
		return realReq
	}
	realReq.Method = parsedReq.Method
	// URL part
	if parsedReq.URL.Host == "" {
		realReq.Host = parsedReq.Host
		parsedReq.URL.Host = parsedReq.Host
	}
	if parsedReq.URL.Scheme == "" {
		if parsedReq.Referer() == "" {
			realReq.Scheme = "https"
			parsedReq.URL.Scheme = "https"
		} else {
			u, err := url.Parse(parsedReq.Referer())
			if err == nil {
				realReq.Scheme = u.Scheme
				parsedReq.URL.Scheme = u.Scheme
			}
		}
	}
	// fmt.Println(parsedReq.URL)
	// parsedReq.URL.RequestURI = parsedReq.RequestURI
	realReq.URL = parsedReq.URL.String()
	realReq.Path = parsedReq.RequestURI
	realReq.Headers = ParseHeaders(parsedReq.Header)

	body, _ := ioutil.ReadAll(parsedReq.Body)
	realReq.Body = string(body)

	return realReq
}

// ParseHeaders parse header for sending method
func ParseHeaders(rawHeaders map[string][]string) []map[string]string {
	var headers []map[string]string
	for name, value := range rawHeaders {
		header := map[string]string{
			name: strings.Join(value[:], ""),
		}
		headers = append(headers, header)
	}
	return headers
}

// ParseBurpResponse parse burp style response
func ParseBurpResponse(rawReq string, rawRes string) (res libs.Response) {
	// var res libs.Response
	readerr := bufio.NewReader(strings.NewReader(rawReq))
	parsedReq, _ := http.ReadRequest(readerr)

	reader := bufio.NewReader(strings.NewReader(rawRes))
	parsedRes, err := http.ReadResponse(reader, parsedReq)
	if err != nil {
		return res
	}

	res.Status = fmt.Sprintf("%v %v", parsedRes.Status, parsedRes.Proto)
	res.StatusCode = parsedRes.StatusCode

	var headers []map[string]string
	for name, value := range parsedReq.Header {
		header := map[string]string{
			name: strings.Join(value[:], ""),
		}
		headers = append(headers, header)
	}
	res.Headers = headers

	body, _ := ioutil.ReadAll(parsedRes.Body)
	res.Body = string(body)

	return res
}

// ParseRequestFromServer parse request receive from API server
func ParseRequestFromServer(record *libs.Record, req libs.Request, sign libs.Signature) {
	if req.Raw != "" {
		parsedReq := ParseBurpRequest(req.Raw)
		// check if parse request ok
		if parsedReq.Method != "ParseRequest" {
			record.Request = parsedReq
		} else {
			record.Request = record.OriginReq
		}
	} else {
		record.Request = record.OriginReq
	}

	// get some component from sign
	if req.Method != "" {
		record.Request.Method = req.Method
	}
	if req.Path != "" {
		record.Request.Path = req.Path
	} else {
		record.Request.Path = record.Request.URL
	}
	if req.Body != "" {
		record.Request.Body = req.Body
	}

	// header stuff
	if len(req.Headers) > 0 {
		realHeaders := req.Headers
		keys := []string{}
		for _, realHeader := range req.Headers {
			for key := range realHeader {
				keys = append(keys, key)
			}
		}
		for _, rawHeader := range record.Request.Headers {
			for key := range rawHeader {
				// only add header we didn't define
				if !funk.Contains(keys, key) {
					realHeaders = append(realHeaders, rawHeader)
				}
			}
		}
		record.Request.Headers = realHeaders
	}
	record.Request.Generators = req.Generators
	record.Request.Encoding = req.Encoding
	record.Request.Middlewares = req.Middlewares
	record.Request.Redirect = req.Redirect

	// resolve template
	target := ParseTarget(record.Request.URL)
	record.Request.URL = ResolveVariable(record.Request.Path, target)
	record.Request.Body = ResolveVariable(record.Request.Body, target)
	record.Request.Headers = ResolveHeader(record.Request.Headers, target)
	record.Request.Detections = ResolveDetection(req.Detections, target)
}
